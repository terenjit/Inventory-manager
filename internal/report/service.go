package report

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"inventory-manager/internal/apperr"
	"inventory-manager/internal/auth"
)

var wib = time.FixedZone("WIB", 7*3600)

type Service interface {
	Dashboard(ctx context.Context, actor auth.Actor, storeID uint) (*DashboardStats, error)
	SalesReport(ctx context.Context, actor auth.Actor, f SalesFilter) (*SalesReport, error)
	CommissionReport(ctx context.Context, actor auth.Actor, f CommissionFilter) ([]CommissionRow, error)
	CommissionFull(ctx context.Context, actor auth.Actor, f CommissionFilter) (*CommissionResult, error)
	StockReport(ctx context.Context, actor auth.Actor, f StockFilter) (*StockReport, error)
	GetReceipt(ctx context.Context, actor auth.Actor, orderID uint) (*Receipt, error)
}

type service struct{ db *sqlx.DB }

func NewService(db *sqlx.DB) Service { return &service{db: db} }

// ── Dashboard ──────────────────────────────────────────────────────────────

func (s *service) Dashboard(ctx context.Context, actor auth.Actor, storeID uint) (*DashboardStats, error) {
	now := time.Now().In(wib)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, wib).UTC()
	todayEnd := todayStart.Add(24 * time.Hour)
	yesterdayStart := todayStart.Add(-24 * time.Hour)

	if actor.IsCashier() && actor.StoreID != nil {
		storeID = *actor.StoreID
	}

	stats := &DashboardStats{StoreID: storeID}

	storeFilter := ""
	storeArgs := []any{}
	if storeID > 0 {
		storeFilter = " AND o.store_id = ?"
		storeArgs = append(storeArgs, storeID)
	}

	// Omzet today.
	s.db.GetContext(ctx, &stats.OmzetToday,
		"SELECT COALESCE(SUM(p.amount),0) FROM payments p JOIN orders o ON o.id=p.order_id WHERE p.created_at>=? AND p.created_at<?"+storeFilter,
		append([]any{todayStart, todayEnd}, storeArgs...)...,
	)
	// Omzet yesterday.
	s.db.GetContext(ctx, &stats.OmzetYesterday,
		"SELECT COALESCE(SUM(p.amount),0) FROM payments p JOIN orders o ON o.id=p.order_id WHERE p.created_at>=? AND p.created_at<?"+storeFilter,
		append([]any{yesterdayStart, todayStart}, storeArgs...)...,
	)
	// Transaction count today.
	s.db.GetContext(ctx, &stats.TxCountToday,
		"SELECT COUNT(*) FROM orders o WHERE o.created_at>=? AND o.created_at<? AND o.status!='void'"+storeFilter,
		append([]any{todayStart, todayEnd}, storeArgs...)...,
	)
	// Low stock count.
	lowStockFilter := ""
	lowStockArgs := []any{}
	if storeID > 0 {
		lowStockFilter = " AND i.store_id = ?"
		lowStockArgs = append(lowStockArgs, storeID)
	}
	s.db.GetContext(ctx, &stats.LowStockCount,
		`SELECT COUNT(DISTINCT p.id) FROM products p
		   LEFT JOIN inventory i ON i.product_id = p.id`+lowStockFilter+`
		  WHERE p.is_active=TRUE AND p.min_stock>0
		    AND COALESCE(i.qty,0)<p.min_stock`,
		lowStockArgs...,
	)

	// 7-day chart.
	sevenDaysAgo := todayStart.Add(-6 * 24 * time.Hour)
	s.db.SelectContext(ctx, &stats.Chart7Days,
		`SELECT DATE(CONVERT_TZ(p.created_at,'+00:00','+07:00')) AS day,
		        COALESCE(SUM(p.amount),0) AS omzet
		   FROM payments p JOIN orders o ON o.id=p.order_id
		  WHERE p.created_at>=?`+storeFilter+`
		  GROUP BY day ORDER BY day`,
		append([]any{sevenDaysAgo}, storeArgs...)...,
	)

	// Top 10 products (30 days).
	thirtyDaysAgo := todayStart.Add(-29 * 24 * time.Hour)
	s.db.SelectContext(ctx, &stats.TopProducts,
		`SELECT oi.product_id, p.code AS product_code, p.name AS product_name,
		        SUM(oi.qty) AS qty_sold, SUM(oi.line_total) AS revenue
		   FROM order_items oi
		   JOIN orders o ON o.id=oi.order_id
		   JOIN products p ON p.id=oi.product_id
		  WHERE o.created_at>=? AND o.status!='void'`+storeFilter+`
		  GROUP BY oi.product_id ORDER BY revenue DESC LIMIT 10`,
		append([]any{thirtyDaysAgo}, storeArgs...)...,
	)

	// Recent 10 transactions.
	s.db.SelectContext(ctx, &stats.RecentOrders,
		`SELECT o.id, o.order_no, st.name AS store_name,
		        u.full_name AS cashier_name, o.total, o.status, o.created_at
		   FROM orders o
		   JOIN stores st ON st.id=o.store_id
		   JOIN users u ON u.id=o.cashier_id
		  WHERE 1=1`+storeFilter+`
		  ORDER BY o.created_at DESC LIMIT 10`,
		storeArgs...,
	)

	return stats, nil
}

// ── Sales Report ──────────────────────────────────────────────────────────

func (s *service) SalesReport(ctx context.Context, actor auth.Actor, f SalesFilter) (*SalesReport, error) {
	if actor.IsCashier() && actor.StoreID != nil {
		f.StoreID = *actor.StoreID
	}
	setDefaultDates(&f.DateFrom, &f.DateTo)

	where, args := buildSalesWhere(f)

	var total int
	s.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM orders o JOIN stores st ON st.id=o.store_id JOIN users u ON u.id=o.cashier_id LEFT JOIN salespeople sp ON sp.id=o.sales_id"+where, args...)

	var grandTotal decimal.Decimal
	s.db.GetContext(ctx, &grandTotal,
		"SELECT COALESCE(SUM(o.total),0) FROM orders o JOIN stores st ON st.id=o.store_id JOIN users u ON u.id=o.cashier_id LEFT JOIN salespeople sp ON sp.id=o.sales_id"+where,
		args...,
	)

	limit := pageLimit(f.Limit)
	page := pageNum(f.Page)
	pageArgs := append(args, limit, (page-1)*limit)

	var rows []SalesRow
	s.db.SelectContext(ctx, &rows,
		`SELECT o.id, o.order_no, st.name AS store_name, u.full_name AS cashier_name,
		        sp.name AS sales_name, o.customer_name, o.subtotal,
		        o.discount_amount, o.total, o.paid_amount, o.status, o.created_at
		   FROM orders o
		   JOIN stores st ON st.id=o.store_id
		   JOIN users u ON u.id=o.cashier_id
		   LEFT JOIN salespeople sp ON sp.id=o.sales_id`+
			where+` ORDER BY o.created_at DESC LIMIT ? OFFSET ?`,
		pageArgs...,
	)

	var byMethod []PaymentMethodBreakdown
	s.db.SelectContext(ctx, &byMethod,
		`SELECT p.method, COALESCE(SUM(p.amount),0) AS total, COUNT(*) AS cnt
		   FROM payments p
		   JOIN orders o ON o.id=p.order_id
		   JOIN stores st ON st.id=o.store_id
		   JOIN users u ON u.id=o.cashier_id
		   LEFT JOIN salespeople sp ON sp.id=o.sales_id`+
			where+` GROUP BY p.method`,
		args...,
	)

	var byHour []HourlySales
	s.db.SelectContext(ctx, &byHour,
		`SELECT HOUR(CONVERT_TZ(o.created_at,'+00:00','+07:00')) AS hour,
		        COUNT(*) AS cnt, COALESCE(SUM(o.total),0) AS total
		   FROM orders o
		   JOIN stores st ON st.id=o.store_id
		   JOIN users u ON u.id=o.cashier_id
		   LEFT JOIN salespeople sp ON sp.id=o.sales_id`+
			where+` GROUP BY hour ORDER BY hour`,
		args...,
	)

	return &SalesReport{
		Data: rows, Total: total, Page: page, Limit: limit,
		GrandTotal: grandTotal, ByMethod: byMethod, ByHour: byHour,
	}, nil
}

// ── Komisi Sales ──────────────────────────────────────────────────────────

func (s *service) CommissionReport(ctx context.Context, actor auth.Actor, f CommissionFilter) ([]CommissionRow, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	setDefaultDates(&f.DateFrom, &f.DateTo)

	conditions := []string{"sp.is_active=TRUE"}
	args := []any{f.DateFrom.UTC(), f.DateTo.UTC()}

	if f.SalespersonID > 0 {
		conditions = append(conditions, "sp.id=?")
		args = append(args, f.SalespersonID)
	}

	where := " WHERE " + strings.Join(conditions, " AND ")

	var rows []CommissionRow
	err := s.db.SelectContext(ctx, &rows,
		`SELECT sp.id AS salesperson_id, sp.name AS salesperson_name,
		        sp.commission_rate,
		        COALESCE(SUM(CASE WHEN o.status IN ('paid','partial') THEN o.total ELSE 0 END), 0) AS total_sales,
		        COALESCE(SUM(CASE WHEN o.status IN ('paid','partial') THEN o.total * sp.commission_rate / 100 ELSE 0 END), 0) AS commission_earned,
		        COUNT(CASE WHEN o.status IN ('paid','partial') THEN 1 END) AS tx_count
		   FROM salespeople sp
		   LEFT JOIN orders o ON o.sales_id=sp.id AND o.created_at>=? AND o.created_at<=?`+
			where+` GROUP BY sp.id ORDER BY commission_earned DESC`,
		args...,
	)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return rows, nil
}

func (s *service) CommissionFull(ctx context.Context, actor auth.Actor, f CommissionFilter) (*CommissionResult, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	setDefaultDates(&f.DateFrom, &f.DateTo)

	summary, err := s.CommissionReport(ctx, actor, f)
	if err != nil {
		return nil, err
	}

	spFilter := ""
	spArgs := []any{f.DateFrom.UTC(), f.DateTo.UTC()}
	if f.SalespersonID > 0 {
		spFilter = " AND sp.id = ?"
		spArgs = append(spArgs, f.SalespersonID)
	}

	var details []CommissionDetailRow
	err = s.db.SelectContext(ctx, &details, `
		SELECT
		  o.order_no,
		  o.created_at,
		  sp.id   AS salesperson_id,
		  sp.name AS sales_name,
		  o.customer_name,
		  p.code  AS product_code,
		  p.name  AS product_name,
		  p.unit,
		  oi.qty,
		  oi.line_total,
		  sp.commission_rate,
		  CASE WHEN o.subtotal > 0
		    THEN ROUND(oi.line_total * o.total / o.subtotal * sp.commission_rate / 100, 0)
		    ELSE ROUND(oi.line_total * sp.commission_rate / 100, 0)
		  END AS commission_value
		FROM order_items oi
		JOIN orders     o  ON o.id  = oi.order_id
		JOIN products   p  ON p.id  = oi.product_id
		JOIN salespeople sp ON sp.id = o.sales_id
		WHERE o.status IN ('paid','partial')
		  AND o.sales_id IS NOT NULL
		  AND o.created_at >= ?
		  AND o.created_at <= ?`+spFilter+`
		ORDER BY o.created_at DESC, oi.id`, spArgs...)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if details == nil {
		details = []CommissionDetailRow{}
	}
	return &CommissionResult{Summary: summary, Details: details}, nil
}

// ── Stock Report ──────────────────────────────────────────────────────────

func (s *service) StockReport(ctx context.Context, actor auth.Actor, f StockFilter) (*StockReport, error) {
	if f.ProductID == 0 {
		return nil, apperr.BadRequest("product_id is required")
	}
	if actor.IsCashier() && actor.StoreID != nil && f.StoreID == 0 {
		f.StoreID = *actor.StoreID
	}
	setDefaultDates(&f.DateFrom, &f.DateTo)

	conditions := []string{"sm.product_id=?"}
	args := []any{f.ProductID}

	if f.StoreID > 0 {
		conditions = append(conditions, "sm.store_id=?")
		args = append(args, f.StoreID)
	}
	conditions = append(conditions, "sm.created_at>=?", "sm.created_at<=?")
	args = append(args, f.DateFrom.UTC(), f.DateTo.UTC())

	where := " WHERE " + strings.Join(conditions, " AND ")

	var total int
	s.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM stock_movements sm"+where, args...)

	limit := pageLimit(f.Limit)
	page := pageNum(f.Page)

	var rows []MovementRow
	s.db.SelectContext(ctx, &rows,
		`SELECT sm.id, sm.store_id, st.name AS store_name,
		        p.code AS product_code, p.name AS product_name, p.unit,
		        sm.delta, sm.qty_after, sm.reason, sm.ref_type, sm.ref_id,
		        sm.note, u.full_name AS created_by, sm.created_at
		   FROM stock_movements sm
		   JOIN stores st ON st.id=sm.store_id
		   JOIN products p ON p.id=sm.product_id
		   JOIN users u ON u.id=sm.created_by`+
			where+` ORDER BY sm.created_at ASC LIMIT ? OFFSET ?`,
		append(args, limit, (page-1)*limit)...,
	)
	return &StockReport{Data: rows, Total: total, Page: page, Limit: limit}, nil
}

// ── Receipt ───────────────────────────────────────────────────────────────

func (s *service) GetReceipt(ctx context.Context, actor auth.Actor, orderID uint) (*Receipt, error) {
	type orderRow struct {
		OrderNo      string          `db:"order_no"`
		StoreName    string          `db:"store_name"`
		CashierName  string          `db:"cashier_name"`
		SalesName    *string         `db:"sales_name"`
		CustomerName *string         `db:"customer_name"`
		Subtotal     decimal.Decimal `db:"subtotal"`
		DiscountAmt  decimal.Decimal `db:"discount_amount"`
		Total        decimal.Decimal `db:"total"`
		PaidAmount   decimal.Decimal `db:"paid_amount"`
		StoreID      uint            `db:"store_id"`
		CreatedAt    time.Time       `db:"created_at"`
	}
	var o orderRow
	if err := s.db.GetContext(ctx, &o,
		`SELECT o.order_no, st.name AS store_name, u.full_name AS cashier_name,
		        sp.name AS sales_name, o.customer_name, o.subtotal,
		        o.discount_amount, o.total, o.paid_amount, o.store_id, o.created_at
		   FROM orders o
		   JOIN stores st ON st.id=o.store_id
		   JOIN users u ON u.id=o.cashier_id
		   LEFT JOIN salespeople sp ON sp.id=o.sales_id
		  WHERE o.id=?`, orderID,
	); err != nil {
		return nil, apperr.NotFound("order not found")
	}

	if actor.IsCashier() && actor.StoreID != nil && o.StoreID != *actor.StoreID {
		return nil, apperr.Forbidden("access denied")
	}

	type itemRow struct {
		ProductCode string          `db:"product_code"`
		ProductName string          `db:"product_name"`
		Unit        string          `db:"unit"`
		Qty         decimal.Decimal `db:"qty"`
		UnitPrice   decimal.Decimal `db:"unit_price"`
		LineTotal   decimal.Decimal `db:"line_total"`
	}
	var items []itemRow
	s.db.SelectContext(ctx, &items,
		`SELECT p.code AS product_code, p.name AS product_name, p.unit,
		        oi.qty, oi.unit_price, oi.line_total
		   FROM order_items oi JOIN products p ON p.id=oi.product_id
		  WHERE oi.order_id=? ORDER BY oi.id`, orderID,
	)

	type payRow struct {
		Method    string          `db:"method"`
		Amount    decimal.Decimal `db:"amount"`
		Reference *string         `db:"reference"`
	}
	var payments []payRow
	s.db.SelectContext(ctx, &payments,
		"SELECT method, amount, reference FROM payments WHERE order_id=? ORDER BY created_at", orderID,
	)

	receiptItems := make([]ReceiptItem, len(items))
	for i, it := range items {
		receiptItems[i] = ReceiptItem{
			ProductCode: it.ProductCode, ProductName: it.ProductName, Unit: it.Unit,
			Qty: it.Qty, UnitPrice: it.UnitPrice, LineTotal: it.LineTotal,
		}
	}
	receiptPayments := make([]ReceiptPayment, len(payments))
	for i, p := range payments {
		receiptPayments[i] = ReceiptPayment{Method: p.Method, Amount: p.Amount, Reference: p.Reference}
	}

	return &Receipt{
		OrderNo:        o.OrderNo,
		StoreName:      o.StoreName,
		CashierName:    o.CashierName,
		SalesName:      o.SalesName,
		CustomerName:   o.CustomerName,
		CreatedAt:      o.CreatedAt,
		Items:          receiptItems,
		Subtotal:       o.Subtotal,
		DiscountAmount: o.DiscountAmt,
		Total:          o.Total,
		Payments:       receiptPayments,
		PaidAmount:     o.PaidAmount,
		Remaining:      o.Total.Sub(o.PaidAmount),
	}, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────

func buildSalesWhere(f SalesFilter) (string, []any) {
	conditions := []string{"o.created_at>=?", "o.created_at<=?"}
	args := []any{f.DateFrom.UTC(), f.DateTo.UTC()}

	if f.StoreID > 0 {
		conditions = append(conditions, "o.store_id=?")
		args = append(args, f.StoreID)
	}
	if f.CashierID > 0 {
		conditions = append(conditions, "o.cashier_id=?")
		args = append(args, f.CashierID)
	}
	if f.SalesID > 0 {
		conditions = append(conditions, "o.sales_id=?")
		args = append(args, f.SalesID)
	}
	if f.Status != "" {
		conditions = append(conditions, "o.status=?")
		args = append(args, f.Status)
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func setDefaultDates(from, to *time.Time) {
	now := time.Now().In(wib)
	if from.IsZero() {
		*from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, wib)
	}
	if to.IsZero() {
		*to = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, wib)
	}
}

func pageLimit(l int) int {
	if l <= 0 || l > 100 {
		return 20
	}
	return l
}

func pageNum(p int) int {
	if p < 1 {
		return 1
	}
	return p
}
