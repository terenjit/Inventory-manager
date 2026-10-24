package transfer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"inventory-manager/internal/apperr"
)

var wib = time.FixedZone("WIB", 7*3600)

type Repository interface {
	NextDocNo(ctx context.Context, tx *sqlx.Tx, storeID uint, storeCode string) (string, error)
	Create(ctx context.Context, tx *sqlx.Tx, t *Transfer, items []*TransferItem) (uint, error)
	GetByID(ctx context.Context, id uint) (*Transfer, error)
	GetItems(ctx context.Context, transferID uint) ([]*TransferItem, error)
	List(ctx context.Context, f ListFilter) ([]*Transfer, int, error)
	UpdateStatus(ctx context.Context, tx *sqlx.Tx, id uint, newStatus string, byUser uint) error
}

type sqlxRepo struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &sqlxRepo{db: db} }

func (r *sqlxRepo) NextDocNo(ctx context.Context, tx *sqlx.Tx, storeID uint, storeCode string) (string, error) {
	now := time.Now().In(wib)
	period := now.Format("2006-01-02")

	_, err := tx.ExecContext(ctx,
		`INSERT INTO doc_sequences (store_id, doc_type, period, last_no) VALUES (?, 'transfer', ?, 1)
		 ON DUPLICATE KEY UPDATE last_no = last_no + 1`,
		storeID, period,
	)
	if err != nil {
		return "", apperr.Internal(err)
	}
	var lastNo int
	if err := tx.GetContext(ctx, &lastNo,
		`SELECT last_no FROM doc_sequences WHERE store_id = ? AND doc_type = 'transfer' AND period = ?`,
		storeID, period,
	); err != nil {
		return "", apperr.Internal(err)
	}
	return fmt.Sprintf("TRF/%s/%s/%04d", storeCode, now.Format("20060102"), lastNo), nil
}

func (r *sqlxRepo) Create(ctx context.Context, tx *sqlx.Tx, t *Transfer, items []*TransferItem) (uint, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO stock_transfers (transfer_no, from_store_id, to_store_id, note, sent_by)
		 VALUES (?, ?, ?, ?, ?)`,
		t.TransferNo, t.FromStoreID, t.ToStoreID, t.Note, t.SentBy,
	)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	id, _ := res.LastInsertId()

	for _, item := range items {
		_, err := tx.ExecContext(ctx,
			"INSERT INTO stock_transfer_items (transfer_id, product_id, qty) VALUES (?, ?, ?)",
			id, item.ProductID, item.Qty,
		)
		if err != nil {
			return 0, apperr.Internal(err)
		}
	}
	return uint(id), nil
}

func (r *sqlxRepo) GetByID(ctx context.Context, id uint) (*Transfer, error) {
	var t Transfer
	err := r.db.GetContext(ctx, &t,
		`SELECT t.*,
		        fs.name AS from_store_name,
		        fs.code AS from_store_code,
		        ts.name AS to_store_name,
		        su.full_name AS sent_by_name,
		        ru.full_name AS received_by_name
		   FROM stock_transfers t
		   JOIN stores fs ON fs.id = t.from_store_id
		   JOIN stores ts ON ts.id = t.to_store_id
		   JOIN users su ON su.id = t.sent_by
		   LEFT JOIN users ru ON ru.id = t.received_by
		  WHERE t.id = ?`, id,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("transfer not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &t, nil
}

func (r *sqlxRepo) GetItems(ctx context.Context, transferID uint) ([]*TransferItem, error) {
	var items []*TransferItem
	if err := r.db.SelectContext(ctx, &items,
		`SELECT ti.transfer_id, ti.product_id, ti.qty,
		        p.code AS product_code, p.name AS product_name, p.unit
		   FROM stock_transfer_items ti
		   JOIN products p ON p.id = ti.product_id
		  WHERE ti.transfer_id = ?
		  ORDER BY p.name`, transferID,
	); err != nil {
		return nil, apperr.Internal(err)
	}
	return items, nil
}

func (r *sqlxRepo) List(ctx context.Context, f ListFilter) ([]*Transfer, int, error) {
	where, args := buildWhere(f)

	var total int
	if err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM stock_transfers t"+where, args...); err != nil {
		return nil, 0, apperr.Internal(err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := f.Page
	if page < 1 {
		page = 1
	}

	query := `SELECT t.*,
	                 fs.name AS from_store_name,
	                 fs.code AS from_store_code,
	                 ts.name AS to_store_name,
	                 su.full_name AS sent_by_name,
	                 ru.full_name AS received_by_name
	            FROM stock_transfers t
	            JOIN stores fs ON fs.id = t.from_store_id
	            JOIN stores ts ON ts.id = t.to_store_id
	            JOIN users su ON su.id = t.sent_by
	            LEFT JOIN users ru ON ru.id = t.received_by` +
		where + ` ORDER BY t.sent_at DESC LIMIT ? OFFSET ?`

	args = append(args, limit, (page-1)*limit)
	var transfers []*Transfer
	if err := r.db.SelectContext(ctx, &transfers, query, args...); err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return transfers, total, nil
}

// UpdateStatus transitions status atomically — only if current status is 'sent'.
// Returns 409 if the row was already received or cancelled.
func (r *sqlxRepo) UpdateStatus(ctx context.Context, tx *sqlx.Tx, id uint, newStatus string, byUser uint) error {
	var query string
	var args []any

	switch newStatus {
	case "received":
		query = `UPDATE stock_transfers
		            SET status = 'received', received_by = ?, received_at = NOW(3)
		          WHERE id = ? AND status = 'sent'`
		args = []any{byUser, id}
	case "cancelled":
		query = `UPDATE stock_transfers SET status = 'cancelled' WHERE id = ? AND status = 'sent'`
		args = []any{id}
	default:
		return apperr.Internal(fmt.Errorf("unknown status: %s", newStatus))
	}

	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return apperr.Internal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.Conflict(fmt.Sprintf("transfer cannot be %s (already received or cancelled)", newStatus))
	}
	return nil
}

func buildWhere(f ListFilter) (string, []any) {
	conditions := []string{}
	args := []any{}

	if f.StoreID > 0 {
		switch f.Direction {
		case "incoming":
			conditions = append(conditions, "t.to_store_id = ?")
			args = append(args, f.StoreID)
		case "outgoing":
			conditions = append(conditions, "t.from_store_id = ?")
			args = append(args, f.StoreID)
		default:
			conditions = append(conditions, "(t.from_store_id = ? OR t.to_store_id = ?)")
			args = append(args, f.StoreID, f.StoreID)
		}
	}
	if f.Status != "" {
		conditions = append(conditions, "t.status = ?")
		args = append(args, f.Status)
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

