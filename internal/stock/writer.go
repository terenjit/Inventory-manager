package stock

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"inventory-manager/internal/apperr"
)

type Reason string
type RefType string

const (
	ReasonSale           Reason = "sale"
	ReasonSaleVoid       Reason = "sale_void"
	ReasonTransferOut    Reason = "transfer_out"
	ReasonTransferIn     Reason = "transfer_in"
	ReasonTransferCancel Reason = "transfer_cancel"
	ReasonPurchase       Reason = "purchase"
	ReasonOpname         Reason = "opname"
	ReasonImport         Reason = "import"
	ReasonAdjustment     Reason = "adjustment"

	RefOrder    RefType = "order"
	RefTransfer RefType = "transfer"
	RefPurchase RefType = "purchase"
	RefOpname   RefType = "opname"
	RefManual   RefType = "manual"
)

type Writer struct {
	db *sqlx.DB
}

func NewWriter(db *sqlx.DB) *Writer { return &Writer{db: db} }

// Decrement removes qty from inventory inside tx.
// Locks the row with SELECT FOR UPDATE, then updates with qty >= check.
// Writes exactly one stock_movements row in the same transaction.
func (w *Writer) Decrement(
	ctx context.Context,
	tx *sqlx.Tx,
	storeID, productID uint,
	qty decimal.Decimal,
	reason Reason,
	refType RefType,
	refID uint,
	note string,
	createdBy uint,
) error {
	// Lock the inventory row.
	var current decimal.Decimal
	err := tx.GetContext(ctx, &current,
		"SELECT qty FROM inventory WHERE store_id = ? AND product_id = ? FOR UPDATE",
		storeID, productID,
	)
	if err != nil {
		// No inventory row means qty = 0.
		return ErrInsufficientStock
	}
	if current.LessThan(qty) {
		return ErrInsufficientStock
	}

	// Update with the qty >= guard — last line of defense against overselling.
	res, err := tx.ExecContext(ctx,
		"UPDATE inventory SET qty = qty - ? WHERE store_id = ? AND product_id = ? AND qty >= ?",
		qty, storeID, productID, qty,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		// Concurrent update stole the stock between our SELECT and UPDATE.
		return ErrInsufficientStock
	}

	qtyAfter := current.Sub(qty)
	return w.insertMovement(ctx, tx, storeID, productID, qty.Neg(), qtyAfter, reason, refType, refID, note, createdBy)
}

// Increment adds qty to inventory inside tx.
// Upserts the inventory row then writes one stock_movements row.
func (w *Writer) Increment(
	ctx context.Context,
	tx *sqlx.Tx,
	storeID, productID uint,
	qty decimal.Decimal,
	reason Reason,
	refType RefType,
	refID uint,
	note string,
	createdBy uint,
) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO inventory (store_id, product_id, qty)
		 VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE qty = qty + VALUES(qty)`,
		storeID, productID, qty,
	)
	if err != nil {
		return apperr.Internal(err)
	}

	var qtyAfter decimal.Decimal
	if err := tx.GetContext(ctx, &qtyAfter,
		"SELECT qty FROM inventory WHERE store_id = ? AND product_id = ?",
		storeID, productID,
	); err != nil {
		return apperr.Internal(err)
	}

	return w.insertMovement(ctx, tx, storeID, productID, qty, qtyAfter, reason, refType, refID, note, createdBy)
}

func (w *Writer) insertMovement(
	ctx context.Context,
	tx *sqlx.Tx,
	storeID, productID uint,
	delta, qtyAfter decimal.Decimal,
	reason Reason,
	refType RefType,
	refID uint,
	note string,
	createdBy uint,
) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO stock_movements
		   (store_id, product_id, delta, qty_after, reason, ref_type, ref_id, note, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		storeID, productID, delta, qtyAfter, reason, refType, nullableUint(refID), note, createdBy,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// Adjust applies a signed delta for opname corrections.
// Unlike Decrement, it does not enforce qty >= requested — the result is floored at 0.
// Actual delta stored in movements is the real change (clamped), not the intended delta.
func (w *Writer) Adjust(
	ctx context.Context,
	tx *sqlx.Tx,
	storeID, productID uint,
	delta decimal.Decimal,
	refID uint,
	note string,
	createdBy uint,
) error {
	if delta.IsZero() {
		return nil
	}

	// Ensure the row exists so we can lock it.
	if _, err := tx.ExecContext(ctx,
		"INSERT IGNORE INTO inventory (store_id, product_id, qty) VALUES (?, ?, 0)",
		storeID, productID,
	); err != nil {
		return apperr.Internal(err)
	}

	var current decimal.Decimal
	if err := tx.GetContext(ctx, &current,
		"SELECT qty FROM inventory WHERE store_id = ? AND product_id = ? FOR UPDATE",
		storeID, productID,
	); err != nil {
		return apperr.Internal(err)
	}

	newQty := current.Add(delta)
	if newQty.IsNegative() {
		newQty = decimal.Zero
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE inventory SET qty = ? WHERE store_id = ? AND product_id = ?",
		newQty, storeID, productID,
	); err != nil {
		return apperr.Internal(err)
	}

	actualDelta := newQty.Sub(current)
	if actualDelta.IsZero() {
		return nil
	}
	return w.insertMovement(ctx, tx, storeID, productID, actualDelta, newQty, ReasonOpname, RefOpname, refID, note, createdBy)
}

// IsDeadlock reports whether err is a MySQL deadlock error (1213).
func IsDeadlock(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1213
}

// WithRetry runs fn in a transaction, retrying once on deadlock.
func (w *Writer) WithRetry(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	for attempt := 0; attempt < 2; attempt++ {
		tx, err := w.db.BeginTxx(ctx, nil)
		if err != nil {
			return apperr.Internal(fmt.Errorf("begin tx: %w", err))
		}
		err = fn(tx)
		if err != nil {
			_ = tx.Rollback()
			if IsDeadlock(err) && attempt == 0 {
				continue
			}
			return err
		}
		if err := tx.Commit(); err != nil {
			return apperr.Internal(fmt.Errorf("commit: %w", err))
		}
		return nil
	}
	return apperr.Internal(fmt.Errorf("transaction failed after retry"))
}

func nullableUint(v uint) interface{} {
	if v == 0 {
		return nil
	}
	return v
}
