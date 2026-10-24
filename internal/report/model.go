package report

import (
	"time"

	"github.com/shopspring/decimal"
)

// ── Dashboard ──────────────────────────────────────────────────────────────

type DashboardStats struct {
	StoreID          uint            `json:"store_id,omitempty"`
	OmzetToday       decimal.Decimal `json:"omzet_today"`
	OmzetYesterday   decimal.Decimal `json:"omzet_yesterday"`
	TxCountToday     int             `json:"tx_count_today"`
	LowStockCount    int             `json:"low_stock_count"`
	Chart7Days       []DayOmzet      `json:"chart_7days"`
	TopProducts      []TopProduct    `json:"top_products"`
	RecentOrders     []RecentOrder   `json:"recent_orders"`
}

type DayOmzet struct {
	Date  string          `json:"date" db:"day"`
	Omzet decimal.Decimal `json:"omzet" db:"omzet"`
}

type TopProduct struct {
	ProductID   uint            `json:"product_id"   db:"product_id"`
	ProductCode string          `json:"product_code" db:"product_code"`
	ProductName string          `json:"product_name" db:"product_name"`
	QtySold     decimal.Decimal `json:"qty_sold"     db:"qty_sold"`
	Revenue     decimal.Decimal `json:"revenue"      db:"revenue"`
}

type RecentOrder struct {
	ID          uint            `json:"id"           db:"id"`
	OrderNo     string          `json:"order_no"     db:"order_no"`
	StoreName   string          `json:"store_name"   db:"store_name"`
	CashierName string          `json:"cashier_name" db:"cashier_name"`
	Total       decimal.Decimal `json:"total"        db:"total"`
	Status      string          `json:"status"       db:"status"`
	CreatedAt   time.Time       `json:"created_at"   db:"created_at"`
}

// ── Sales Report ──────────────────────────────────────────────────────────

type SalesRow struct {
	ID             uint            `json:"id"              db:"id"`
	OrderNo        string          `json:"order_no"        db:"order_no"`
	StoreName      string          `json:"store_name"      db:"store_name"`
	CashierName    string          `json:"cashier_name"    db:"cashier_name"`
	SalesName      *string         `json:"sales_name"      db:"sales_name"`
	CustomerName   *string         `json:"customer_name"   db:"customer_name"`
	Subtotal       decimal.Decimal `json:"subtotal"        db:"subtotal"`
	DiscountAmount decimal.Decimal `json:"discount_amount" db:"discount_amount"`
	Total          decimal.Decimal `json:"total"           db:"total"`
	PaidAmount     decimal.Decimal `json:"paid_amount"     db:"paid_amount"`
	Status         string          `json:"status"          db:"status"`
	CreatedAt      time.Time       `json:"created_at"      db:"created_at"`
}

type PaymentMethodBreakdown struct {
	Method string          `json:"method" db:"method"`
	Total  decimal.Decimal `json:"total"  db:"total"`
	Count  int             `json:"count"  db:"cnt"`
}

type HourlySales struct {
	Hour  int             `json:"hour"  db:"hour"`
	Count int             `json:"count" db:"cnt"`
	Total decimal.Decimal `json:"total" db:"total"`
}

type SalesReport struct {
	Data       []SalesRow               `json:"data"`
	Total      int                      `json:"total"`
	Page       int                      `json:"page"`
	Limit      int                      `json:"limit"`
	GrandTotal decimal.Decimal          `json:"grand_total"`
	ByMethod   []PaymentMethodBreakdown `json:"by_method"`
	ByHour     []HourlySales            `json:"by_hour"`
}

// ── Komisi Sales ──────────────────────────────────────────────────────────

type CommissionRow struct {
	SalespersonID    uint            `json:"salesperson_id"   db:"salesperson_id"`
	SalespersonName  string          `json:"salesperson_name" db:"salesperson_name"`
	CommissionRate   decimal.Decimal `json:"commission_rate"  db:"commission_rate"`
	TotalSales       decimal.Decimal `json:"total_sales"      db:"total_sales"`
	CommissionEarned decimal.Decimal `json:"commission_earned" db:"commission_earned"`
	TxCount          int             `json:"tx_count"         db:"tx_count"`
}

type CommissionDetailRow struct {
	OrderNo         string          `json:"order_no"         db:"order_no"`
	CreatedAt       time.Time       `json:"created_at"       db:"created_at"`
	SalespersonID   uint            `json:"salesperson_id"   db:"salesperson_id"`
	SalesName       string          `json:"sales_name"       db:"sales_name"`
	CustomerName    *string         `json:"customer_name"    db:"customer_name"`
	ProductCode     string          `json:"product_code"     db:"product_code"`
	ProductName     string          `json:"product_name"     db:"product_name"`
	Unit            string          `json:"unit"             db:"unit"`
	Qty             decimal.Decimal `json:"qty"              db:"qty"`
	LineTotal       decimal.Decimal `json:"line_total"       db:"line_total"`
	CommissionRate  decimal.Decimal `json:"commission_rate"  db:"commission_rate"`
	CommissionValue decimal.Decimal `json:"commission_value" db:"commission_value"`
}

type CommissionResult struct {
	Summary []CommissionRow       `json:"summary"`
	Details []CommissionDetailRow `json:"details"`
}

// ── Stock Report (Kartu Stok) ─────────────────────────────────────────────

type MovementRow struct {
	ID          uint            `json:"id"           db:"id"`
	StoreID     uint            `json:"store_id"     db:"store_id"`
	StoreName   string          `json:"store_name"   db:"store_name"`
	ProductCode string          `json:"product_code" db:"product_code"`
	ProductName string          `json:"product_name" db:"product_name"`
	Unit        string          `json:"unit"         db:"unit"`
	Delta       decimal.Decimal `json:"delta"        db:"delta"`
	QtyAfter    decimal.Decimal `json:"qty_after"    db:"qty_after"`
	Reason      string          `json:"reason"       db:"reason"`
	RefType     *string         `json:"ref_type"     db:"ref_type"`
	RefID       *uint           `json:"ref_id"       db:"ref_id"`
	Note        *string         `json:"note"         db:"note"`
	CreatedBy   string          `json:"created_by"   db:"created_by"`
	CreatedAt   time.Time       `json:"created_at"   db:"created_at"`
}

type StockReport struct {
	Data  []MovementRow `json:"data"`
	Total int           `json:"total"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
}

// ── Filters ───────────────────────────────────────────────────────────────

type SalesFilter struct {
	StoreID   uint
	DateFrom  time.Time
	DateTo    time.Time
	CashierID uint
	SalesID   uint
	Status    string
	Page      int
	Limit     int
}

type CommissionFilter struct {
	DateFrom    time.Time
	DateTo      time.Time
	SalespersonID uint
}

type StockFilter struct {
	ProductID uint
	StoreID   uint
	DateFrom  time.Time
	DateTo    time.Time
	Page      int
	Limit     int
}

// ── Receipt ───────────────────────────────────────────────────────────────

type ReceiptPayment struct {
	Method    string          `json:"method"`
	Amount    decimal.Decimal `json:"amount"`
	Reference *string         `json:"reference,omitempty"`
}

type ReceiptItem struct {
	ProductCode string          `json:"product_code"`
	ProductName string          `json:"product_name"`
	Unit        string          `json:"unit"`
	Qty         decimal.Decimal `json:"qty"`
	UnitPrice   decimal.Decimal `json:"unit_price"`
	LineTotal   decimal.Decimal `json:"line_total"`
}

type Receipt struct {
	OrderNo        string           `json:"order_no"`
	StoreName      string           `json:"store_name"`
	CashierName    string           `json:"cashier_name"`
	SalesName      *string          `json:"sales_name,omitempty"`
	CustomerName   *string          `json:"customer_name,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	Items          []ReceiptItem    `json:"items"`
	Subtotal       decimal.Decimal  `json:"subtotal"`
	DiscountAmount decimal.Decimal  `json:"discount_amount"`
	Total          decimal.Decimal  `json:"total"`
	Payments       []ReceiptPayment `json:"payments"`
	PaidAmount     decimal.Decimal  `json:"paid_amount"`
	Remaining      decimal.Decimal  `json:"remaining"`
}
