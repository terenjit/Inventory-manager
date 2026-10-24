package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"inventory-manager/internal/apperr"
)

var wib = time.FixedZone("WIB", 7*3600)

type Repository interface {
	NextDocNo(ctx context.Context, tx *sqlx.Tx, storeID uint, storeCode string) (string, error)
	Create(ctx context.Context, tx *sqlx.Tx, o *Order, items []*OrderItem) (uint, error)
	GetByID(ctx context.Context, id uint) (*Order, error)
	GetByOrderNo(ctx context.Context, orderNo string) (*Order, error)
	GetItems(ctx context.Context, orderID uint) ([]*OrderItem, error)
	GetItemsByOrderIDs(ctx context.Context, orderIDs []uint) ([]*OrderItem, error)
	List(ctx context.Context, f ListFilter) ([]*Order, int, error)
	AddPayment(ctx context.Context, tx *sqlx.Tx, p *Payment) error
	UpdateOrderPayment(ctx context.Context, tx *sqlx.Tx, orderID uint, newPaid, total decimal.Decimal) error
	VoidOrder(ctx context.Context, tx *sqlx.Tx, orderID uint, reason string, voidedBy uint) error
	UpdateMeta(ctx context.Context, id uint, input UpdateMetaInput) error
	AppendItems(ctx context.Context, tx *sqlx.Tx, orderID uint, items []*OrderItem) error
	UpdateTotals(ctx context.Context, tx *sqlx.Tx, orderID uint, subtotal, discountAmount, total decimal.Decimal) error
}

type sqlxRepo struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &sqlxRepo{db: db} }

func (r *sqlxRepo) NextDocNo(ctx context.Context, tx *sqlx.Tx, storeID uint, storeCode string) (string, error) {
	now := time.Now().In(wib)
	period := now.Format("2006-01-02")

	_, err := tx.ExecContext(ctx,
		`INSERT INTO doc_sequences (store_id, doc_type, period, last_no) VALUES (?, 'order', ?, 1)
		 ON DUPLICATE KEY UPDATE last_no = last_no + 1`,
		storeID, period,
	)
	if err != nil {
		return "", apperr.Internal(err)
	}

	var lastNo int
	if err := tx.GetContext(ctx, &lastNo,
		`SELECT last_no FROM doc_sequences WHERE store_id = ? AND doc_type = 'order' AND period = ?`,
		storeID, period,
	); err != nil {
		return "", apperr.Internal(err)
	}

	return fmt.Sprintf("INV/%s/%s/%04d", storeCode, now.Format("20060102"), lastNo), nil
}

func (r *sqlxRepo) Create(ctx context.Context, tx *sqlx.Tx, o *Order, items []*OrderItem) (uint, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO orders
		   (order_no, store_id, cashier_id, sales_id, customer_name, note,
		    subtotal, tax, discount_pct, discount_rp, discount_amount, total, paid_amount, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 'unpaid')`,
		o.OrderNo, o.StoreID, o.CashierID, o.SalesID, o.CustomerName, o.Note,
		o.Subtotal, o.Tax, o.DiscountPct, o.DiscountRp, o.DiscountAmount, o.Total,
	)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	orderID, _ := res.LastInsertId()

	for _, item := range items {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO order_items (order_id, product_id, qty, unit_price, unit_cost, line_total)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			orderID, item.ProductID, item.Qty, item.UnitPrice, item.UnitCost, item.LineTotal,
		)
		if err != nil {
			return 0, apperr.Internal(err)
		}
	}

	return uint(orderID), nil
}

func (r *sqlxRepo) GetByID(ctx context.Context, id uint) (*Order, error) {
	var o Order
	err := r.db.GetContext(ctx, &o,
		`SELECT o.*,
		        st.name AS store_name,
		        st.code AS store_code,
		        u.full_name AS cashier_name,
		        sp.name AS sales_name
		   FROM orders o
		   JOIN stores st ON st.id = o.store_id
		   JOIN users u ON u.id = o.cashier_id
		   LEFT JOIN salespeople sp ON sp.id = o.sales_id
		  WHERE o.id = ?`, id,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("order not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &o, nil
}

func (r *sqlxRepo) GetItems(ctx context.Context, orderID uint) ([]*OrderItem, error) {
	var items []*OrderItem
	if err := r.db.SelectContext(ctx, &items,
		`SELECT oi.*,
		        p.code AS product_code,
		        p.name AS product_name,
		        p.unit AS unit
		   FROM order_items oi
		   JOIN products p ON p.id = oi.product_id
		  WHERE oi.order_id = ?
		  ORDER BY oi.id`, orderID,
	); err != nil {
		return nil, apperr.Internal(err)
	}
	return items, nil
}

func (r *sqlxRepo) GetItemsByOrderIDs(ctx context.Context, orderIDs []uint) ([]*OrderItem, error) {
	if len(orderIDs) == 0 {
		return nil, nil
	}
	query, args, err := sqlx.In(`
		SELECT oi.*,
		       p.code AS product_code,
		       p.name AS product_name,
		       p.unit AS unit
		  FROM order_items oi
		  JOIN products p ON p.id = oi.product_id
		 WHERE oi.order_id IN (?)
		 ORDER BY oi.order_id, oi.id`, orderIDs)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	var items []*OrderItem
	if err := r.db.SelectContext(ctx, &items, r.db.Rebind(query), args...); err != nil {
		return nil, apperr.Internal(err)
	}
	return items, nil
}

const listJoins = ` JOIN stores st ON st.id = o.store_id
	            JOIN users u ON u.id = o.cashier_id
	            LEFT JOIN salespeople sp ON sp.id = o.sales_id`

func (r *sqlxRepo) List(ctx context.Context, f ListFilter) ([]*Order, int, error) {
	where, args := buildListWhere(f)

	var total int
	if err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM orders o"+listJoins+where, args...); err != nil {
		return nil, 0, apperr.Internal(err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 20
	}
	page := f.Page
	if page < 1 {
		page = 1
	}

	query := `SELECT o.*,
	                 st.name AS store_name,
	                 st.code AS store_code,
	                 u.full_name AS cashier_name,
	                 sp.name AS sales_name
	            FROM orders o` + listJoins +
		where + ` ORDER BY o.created_at DESC LIMIT ? OFFSET ?`

	args = append(args, limit, (page-1)*limit)
	var orders []*Order
	if err := r.db.SelectContext(ctx, &orders, query, args...); err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return orders, total, nil
}

func (r *sqlxRepo) AddPayment(ctx context.Context, tx *sqlx.Tx, p *Payment) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO payments (order_id, method, amount, reference, received_by)
		 VALUES (?, ?, ?, ?, ?)`,
		p.OrderID, p.Method, p.Amount, p.Reference, p.ReceivedBy,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (r *sqlxRepo) UpdateOrderPayment(ctx context.Context, tx *sqlx.Tx, orderID uint, newPaid, total decimal.Decimal) error {
	status := "partial"
	if newPaid.GreaterThanOrEqual(total) {
		status = "paid"
	}
	_, err := tx.ExecContext(ctx,
		"UPDATE orders SET paid_amount = ?, status = ? WHERE id = ?",
		newPaid, status, orderID,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (r *sqlxRepo) VoidOrder(ctx context.Context, tx *sqlx.Tx, orderID uint, reason string, voidedBy uint) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE orders
		    SET status = 'void', void_reason = ?, voided_by = ?, voided_at = NOW(3)
		  WHERE id = ? AND status != 'void'`,
		reason, voidedBy, orderID,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.Conflict("order is already voided")
	}
	return nil
}

func (r *sqlxRepo) GetByOrderNo(ctx context.Context, orderNo string) (*Order, error) {
	var o Order
	err := r.db.GetContext(ctx, &o,
		`SELECT o.*,
		        st.name AS store_name,
		        st.code AS store_code,
		        u.full_name AS cashier_name,
		        sp.name AS sales_name
		   FROM orders o
		   JOIN stores st ON st.id = o.store_id
		   JOIN users u ON u.id = o.cashier_id
		   LEFT JOIN salespeople sp ON sp.id = o.sales_id
		  WHERE o.order_no = ?`, orderNo,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("order not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &o, nil
}

func (r *sqlxRepo) UpdateMeta(ctx context.Context, id uint, input UpdateMetaInput) error {
	setParts := []string{}
	args := []any{}
	if input.CustomerName != nil {
		setParts = append(setParts, "customer_name = ?")
		args = append(args, *input.CustomerName)
	}
	if input.SalesID != nil {
		setParts = append(setParts, "sales_id = ?")
		args = append(args, *input.SalesID)
	}
	if input.Note != nil {
		setParts = append(setParts, "note = ?")
		args = append(args, *input.Note)
	}
	if len(setParts) == 0 {
		return nil
	}
	args = append(args, id)
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf("UPDATE orders SET %s WHERE id = ?", strings.Join(setParts, ", ")),
		args...,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (r *sqlxRepo) AppendItems(ctx context.Context, tx *sqlx.Tx, orderID uint, items []*OrderItem) error {
	for _, item := range items {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO order_items (order_id, product_id, qty, unit_price, unit_cost, line_total)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			orderID, item.ProductID, item.Qty, item.UnitPrice, item.UnitCost, item.LineTotal,
		)
		if err != nil {
			return apperr.Internal(err)
		}
	}
	return nil
}

func (r *sqlxRepo) UpdateTotals(ctx context.Context, tx *sqlx.Tx, orderID uint, subtotal, discountAmount, total decimal.Decimal) error {
	_, err := tx.ExecContext(ctx,
		"UPDATE orders SET subtotal = ?, discount_amount = ?, total = ? WHERE id = ?",
		subtotal, discountAmount, total, orderID,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func buildListWhere(f ListFilter) (string, []any) {
	conditions := []string{}
	args := []any{}

	if f.StoreID > 0 {
		conditions = append(conditions, "o.store_id = ?")
		args = append(args, f.StoreID)
	}
	if f.CashierID > 0 {
		conditions = append(conditions, "o.cashier_id = ?")
		args = append(args, f.CashierID)
	}
	if f.Status != "" {
		conditions = append(conditions, "o.status = ?")
		args = append(args, f.Status)
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		conditions = append(conditions, "(o.order_no LIKE ? OR o.customer_name LIKE ? OR u.full_name LIKE ? OR sp.name LIKE ?)")
		args = append(args, like, like, like, like)
	}
	if !f.DateFrom.IsZero() {
		conditions = append(conditions, "o.created_at >= ?")
		args = append(args, f.DateFrom.UTC())
	}
	if !f.DateTo.IsZero() {
		conditions = append(conditions, "o.created_at <= ?")
		args = append(args, f.DateTo.UTC())
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}
