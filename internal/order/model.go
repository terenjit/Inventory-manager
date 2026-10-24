package order

import (
	"time"

	"github.com/shopspring/decimal"
)

// Order is the DB row with joined fields.
type Order struct {
	ID             uint            `db:"id"`
	OrderNo        string          `db:"order_no"`
	StoreID        uint            `db:"store_id"`
	StoreName      string          `db:"store_name"`
	StoreCode      string          `db:"store_code"`
	CashierID      uint            `db:"cashier_id"`
	CashierName    string          `db:"cashier_name"`
	SalesID        *uint           `db:"sales_id"`
	SalesName      *string         `db:"sales_name"`
	CustomerName   *string         `db:"customer_name"`
	Note           *string         `db:"note"`
	Subtotal       decimal.Decimal `db:"subtotal"`
	Tax            decimal.Decimal `db:"tax"`
	DiscountPct    decimal.Decimal `db:"discount_pct"`
	DiscountRp     decimal.Decimal `db:"discount_rp"`
	DiscountAmount decimal.Decimal `db:"discount_amount"`
	Total          decimal.Decimal `db:"total"`
	PaidAmount     decimal.Decimal `db:"paid_amount"`
	Status         string          `db:"status"`
	VoidReason  *string    `db:"void_reason"`
	VoidedBy    *uint      `db:"voided_by"`
	VoidedAt    *time.Time `db:"voided_at"`
	UpdatedBy   *uint      `db:"updated_by"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

// OrderItem is the DB row with joined product fields.
type OrderItem struct {
	ID          uint            `db:"id"`
	OrderID     uint            `db:"order_id"`
	ProductID   uint            `db:"product_id"`
	ProductCode string          `db:"product_code"`
	ProductName string          `db:"product_name"`
	Unit        string          `db:"unit"`
	Qty         decimal.Decimal `db:"qty"`
	UnitPrice   decimal.Decimal `db:"unit_price"`
	UnitCost    decimal.Decimal `db:"unit_cost"`
	LineTotal   decimal.Decimal `db:"line_total"`
}

// CreateOrderInput is the request body from the cashier UI.
type CreateOrderInput struct {
	StoreID      *uint            `json:"store_id"`
	CustomerName *string          `json:"customer_name"`
	SalesID      *uint            `json:"sales_id"`
	Note         *string          `json:"note"`
	DiscountPct  decimal.Decimal  `json:"discount_pct"`
	DiscountRp   decimal.Decimal  `json:"discount_rp"`
	Items        []OrderItemInput `json:"items" binding:"required,min=1"`
}

type OrderItemInput struct {
	ProductID uint            `json:"product_id" binding:"required"`
	Qty       decimal.Decimal `json:"qty"`
}

// ListFilter for GET /orders.
type ListFilter struct {
	StoreID   uint
	CashierID uint
	Status    string
	Search    string
	DateFrom  time.Time
	DateTo    time.Time
	Page      int
	Limit     int
}

// Response types — unit_cost not exposed (admin reports phase).

type ItemResponse struct {
	ID          uint            `json:"id"`
	ProductID   uint            `json:"product_id"`
	ProductCode string          `json:"product_code"`
	ProductName string          `json:"product_name"`
	Unit        string          `json:"unit"`
	Qty         decimal.Decimal `json:"qty"`
	UnitPrice   decimal.Decimal `json:"unit_price"`
	LineTotal   decimal.Decimal `json:"line_total"`
}

type Response struct {
	ID             uint            `json:"id"`
	OrderNo        string          `json:"order_no"`
	StoreID        uint            `json:"store_id"`
	StoreName      string          `json:"store_name"`
	CashierID      uint            `json:"cashier_id"`
	CashierName    string          `json:"cashier_name"`
	SalesID        *uint           `json:"sales_id,omitempty"`
	SalesName      *string         `json:"sales_name,omitempty"`
	CustomerName   *string         `json:"customer_name,omitempty"`
	Note           *string         `json:"note,omitempty"`
	Subtotal       decimal.Decimal `json:"subtotal"`
	DiscountPct    decimal.Decimal `json:"discount_pct"`
	DiscountRp     decimal.Decimal `json:"discount_rp"`
	DiscountAmount decimal.Decimal `json:"discount_amount"`
	Tax            decimal.Decimal `json:"tax"`
	Total          decimal.Decimal `json:"total"`
	PaidAmount     decimal.Decimal `json:"paid_amount"`
	Status         string          `json:"status"`
	VoidReason     *string         `json:"void_reason,omitempty"`
	VoidedAt       *time.Time      `json:"voided_at,omitempty"`
	Items          []ItemResponse  `json:"items,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// Payment represents one payment row.
type Payment struct {
	ID         uint            `db:"id"          json:"id"`
	OrderID    uint            `db:"order_id"    json:"order_id"`
	Method     string          `db:"method"      json:"method"`
	Amount     decimal.Decimal `db:"amount"      json:"amount"`
	Reference  *string         `db:"reference"   json:"reference,omitempty"`
	ReceivedBy uint            `db:"received_by" json:"received_by"`
	CreatedAt  time.Time       `db:"created_at"  json:"created_at"`
}

type AddPaymentInput struct {
	Method    string          `json:"method"    binding:"required,oneof=QRIS TRANSFER CASH"`
	Amount    decimal.Decimal `json:"amount"`
	Reference *string         `json:"reference"`
}

type VoidInput struct {
	Reason string `json:"reason" binding:"required,min=1"`
}

type UpdateMetaInput struct {
	CustomerName *string `json:"customer_name"`
	SalesID      *uint   `json:"sales_id"`
	Note         *string `json:"note"`
}

type ContinueInput struct {
	CustomerName *string          `json:"customer_name"`
	SalesID      *uint            `json:"sales_id"`
	Note         *string          `json:"note"`
	Items        []OrderItemInput `json:"items"`
	Payment      *AddPaymentInput `json:"payment"`
}

type ListResult struct {
	Data  []*Response `json:"data"`
	Total int         `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

func (o *Order) ToResponse(items []ItemResponse) *Response {
	return &Response{
		ID:             o.ID,
		OrderNo:        o.OrderNo,
		StoreID:        o.StoreID,
		StoreName:      o.StoreName,
		CashierID:      o.CashierID,
		CashierName:    o.CashierName,
		SalesID:        o.SalesID,
		SalesName:      o.SalesName,
		CustomerName:   o.CustomerName,
		Note:           o.Note,
		Subtotal:       o.Subtotal,
		DiscountPct:    o.DiscountPct,
		DiscountRp:     o.DiscountRp,
		DiscountAmount: o.DiscountAmount,
		Tax:            o.Tax,
		Total:          o.Total,
		PaidAmount:     o.PaidAmount,
		Status:         o.Status,
		VoidReason:     o.VoidReason,
		VoidedAt:       o.VoidedAt,
		Items:          items,
		CreatedAt:      o.CreatedAt,
	}
}

func toItemResponse(i *OrderItem) ItemResponse {
	return ItemResponse{
		ID:          i.ID,
		ProductID:   i.ProductID,
		ProductCode: i.ProductCode,
		ProductName: i.ProductName,
		Unit:        i.Unit,
		Qty:         i.Qty,
		UnitPrice:   i.UnitPrice,
		LineTotal:   i.LineTotal,
	}
}
