package purchase

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"inventory-manager/internal/apperr"
)

type Repository interface {
	Create(ctx context.Context, inv *Invoice, items []*InvoiceItem) (uint, error)
	GetByID(ctx context.Context, id uint) (*Invoice, error)
	GetItems(ctx context.Context, invoiceID uint) ([]*InvoiceItem, error)
	List(ctx context.Context, f ListFilter) ([]*Invoice, int, decimal.Decimal, error)
	SetGoodsReceivedDate(ctx context.Context, id uint, date time.Time) error
	UpdatePayment(ctx context.Context, id uint, paidAmount, totalAmount decimal.Decimal) error
}

type sqlxRepo struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &sqlxRepo{db: db} }

func (r *sqlxRepo) Create(ctx context.Context, inv *Invoice, items []*InvoiceItem) (uint, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO supplier_invoices
		   (supplier_id, store_id, invoice_no, invoice_date, goods_received_date,
		    amount, due_date, note, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.SupplierID, inv.StoreID, inv.InvoiceNo, inv.InvoiceDate,
		inv.GoodsReceivedDate, inv.Amount, inv.DueDate, inv.Note, inv.CreatedBy,
	)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return 0, apperr.Conflict("invoice number already exists for this supplier")
		}
		return 0, apperr.Internal(err)
	}
	id, _ := res.LastInsertId()

	for _, item := range items {
		_, err := r.db.ExecContext(ctx,
			"INSERT INTO supplier_invoice_items (invoice_id, product_id, qty, unit_cost) VALUES (?, ?, ?, ?)",
			id, item.ProductID, item.Qty, item.UnitCost,
		)
		if err != nil {
			return 0, apperr.Internal(err)
		}
	}
	return uint(id), nil
}

func (r *sqlxRepo) GetByID(ctx context.Context, id uint) (*Invoice, error) {
	var inv Invoice
	err := r.db.GetContext(ctx, &inv,
		`SELECT si.*,
		        s.name AS supplier_name,
		        st.name AS store_name
		   FROM supplier_invoices si
		   JOIN suppliers s  ON s.id  = si.supplier_id
		   JOIN stores    st ON st.id = si.store_id
		  WHERE si.id = ?`, id,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("invoice not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &inv, nil
}

func (r *sqlxRepo) GetItems(ctx context.Context, invoiceID uint) ([]*InvoiceItem, error) {
	var items []*InvoiceItem
	if err := r.db.SelectContext(ctx, &items,
		`SELECT sii.invoice_id, sii.product_id, sii.qty, sii.unit_cost,
		        p.code AS product_code, p.name AS product_name, p.unit
		   FROM supplier_invoice_items sii
		   JOIN products p ON p.id = sii.product_id
		  WHERE sii.invoice_id = ?
		  ORDER BY p.name`, invoiceID,
	); err != nil {
		return nil, apperr.Internal(err)
	}
	return items, nil
}

func (r *sqlxRepo) List(ctx context.Context, f ListFilter) ([]*Invoice, int, decimal.Decimal, error) {
	where, args := buildWhere(f)

	var total int
	if err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM supplier_invoices si"+where, args...); err != nil {
		return nil, 0, decimal.Zero, apperr.Internal(err)
	}

	// Sum of outstanding (belum lunas) = SUM(amount - paid_amount) for unpaid/partial.
	var totalUnpaid decimal.Decimal
	unpaidWhere, unpaidArgs := buildWhere(ListFilter{
		SupplierID: f.SupplierID,
		StoreID:    f.StoreID,
		Status:     "", // all non-paid
	})
	r.db.GetContext(ctx, &totalUnpaid,
		`SELECT COALESCE(SUM(amount - paid_amount), 0)
		   FROM supplier_invoices si`+unpaidWhere+` AND si.status != 'paid'`,
		unpaidArgs...,
	)

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := f.Page
	if page < 1 {
		page = 1
	}

	query := `SELECT si.*,
	                 s.name  AS supplier_name,
	                 st.name AS store_name
	            FROM supplier_invoices si
	            JOIN suppliers s  ON s.id  = si.supplier_id
	            JOIN stores    st ON st.id = si.store_id` +
		where + ` ORDER BY si.invoice_date DESC LIMIT ? OFFSET ?`

	args = append(args, limit, (page-1)*limit)
	var invoices []*Invoice
	if err := r.db.SelectContext(ctx, &invoices, query, args...); err != nil {
		return nil, 0, decimal.Zero, apperr.Internal(err)
	}
	return invoices, total, totalUnpaid, nil
}

// SetGoodsReceivedDate records when goods physically arrived — informational only.
func (r *sqlxRepo) SetGoodsReceivedDate(ctx context.Context, id uint, date time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE supplier_invoices SET goods_received_date = ? WHERE id = ?",
		date.Format("2006-01-02"), id,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// UpdatePayment sets paid_amount and auto-computes status.
func (r *sqlxRepo) UpdatePayment(ctx context.Context, id uint, paidAmount, totalAmount decimal.Decimal) error {
	status := computeStatus(paidAmount, totalAmount)
	var paidAt interface{}
	if status == "paid" {
		paidAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE supplier_invoices SET paid_amount = ?, status = ?, paid_at = ? WHERE id = ?",
		paidAmount, status, paidAt, id,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func computeStatus(paid, total decimal.Decimal) string {
	if paid.IsZero() {
		return "unpaid"
	}
	if paid.GreaterThanOrEqual(total) {
		return "paid"
	}
	return "partial"
}

func buildWhere(f ListFilter) (string, []any) {
	conditions := []string{}
	args := []any{}

	if f.SupplierID > 0 {
		conditions = append(conditions, "si.supplier_id = ?")
		args = append(args, f.SupplierID)
	}
	if f.StoreID > 0 {
		conditions = append(conditions, "si.store_id = ?")
		args = append(args, f.StoreID)
	}
	if f.Status != "" {
		conditions = append(conditions, "si.status = ?")
		args = append(args, f.Status)
	}
	if f.DueBefore != nil {
		conditions = append(conditions, "si.due_date <= ?")
		args = append(args, f.DueBefore.Format("2006-01-02"))
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}
