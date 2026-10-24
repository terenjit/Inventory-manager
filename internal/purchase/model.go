package purchase

import (
	"time"

	"github.com/shopspring/decimal"
)

type Invoice struct {
	ID                uint            `db:"id"`
	SupplierID        uint            `db:"supplier_id"`
	SupplierName      string          `db:"supplier_name"`
	StoreID           uint            `db:"store_id"`
	StoreName         string          `db:"store_name"`
	InvoiceNo         string          `db:"invoice_no"`
	InvoiceDate       time.Time       `db:"invoice_date"`
	GoodsReceivedDate *time.Time      `db:"goods_received_date"`
	Amount            decimal.Decimal `db:"amount"`
	PaidAmount        decimal.Decimal `db:"paid_amount"`
	DueDate           *time.Time      `db:"due_date"`
	Status            string          `db:"status"`
	PaidAt            *time.Time      `db:"paid_at"`
	Note              *string         `db:"note"`
	CreatedBy         uint            `db:"created_by"`
	CreatedAt         time.Time       `db:"created_at"`
}

type InvoiceItem struct {
	InvoiceID   uint            `db:"invoice_id"`
	ProductID   uint            `db:"product_id"`
	ProductCode string          `db:"product_code"`
	ProductName string          `db:"product_name"`
	Unit        string          `db:"unit"`
	Qty         decimal.Decimal `db:"qty"`
	UnitCost    decimal.Decimal `db:"unit_cost"`
}

type CreateInput struct {
	SupplierID        uint        `json:"supplier_id"          binding:"required"`
	StoreID           *uint       `json:"store_id"`
	InvoiceNo         string      `json:"invoice_no"           binding:"required"`
	InvoiceDate       string      `json:"invoice_date"         binding:"required"` // YYYY-MM-DD
	GoodsReceivedDate *string     `json:"goods_received_date"`                     // YYYY-MM-DD, optional
	DueDate           *string     `json:"due_date"`                                // YYYY-MM-DD, optional
	Note              *string     `json:"note"`
	Items             []ItemInput `json:"items" binding:"required,min=1"`
}

type ItemInput struct {
	ProductID uint            `json:"product_id" binding:"required"`
	Qty       decimal.Decimal `json:"qty"`
	UnitCost  decimal.Decimal `json:"unit_cost"`
}

// UpdatePaymentInput — admin sets paid_amount; status is auto-computed.
type UpdatePaymentInput struct {
	PaidAmount decimal.Decimal `json:"paid_amount"`
}

type ListFilter struct {
	SupplierID uint
	Status     string
	StoreID    uint
	DueBefore  *time.Time
	Page       int
	Limit      int
}

type ItemResponse struct {
	ProductID   uint            `json:"product_id"`
	ProductCode string          `json:"product_code"`
	ProductName string          `json:"product_name"`
	Unit        string          `json:"unit"`
	Qty         decimal.Decimal `json:"qty"`
	UnitCost    decimal.Decimal `json:"unit_cost"`
}

type Response struct {
	ID                uint            `json:"id"`
	SupplierID        uint            `json:"supplier_id"`
	SupplierName      string          `json:"supplier_name"`
	StoreID           uint            `json:"store_id"`
	StoreName         string          `json:"store_name"`
	InvoiceNo         string          `json:"invoice_no"`
	InvoiceDate       time.Time       `json:"invoice_date"`
	GoodsReceivedDate *time.Time      `json:"goods_received_date,omitempty"`
	Amount            decimal.Decimal `json:"amount"`
	PaidAmount        decimal.Decimal `json:"paid_amount"`
	Outstanding       decimal.Decimal `json:"outstanding"` // amount - paid_amount
	DueDate           *time.Time      `json:"due_date,omitempty"`
	Status            string          `json:"status"`
	PaidAt            *time.Time      `json:"paid_at,omitempty"`
	Note              *string         `json:"note,omitempty"`
	Items             []ItemResponse  `json:"items,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

type ListResult struct {
	Data        []*Response     `json:"data"`
	Total       int             `json:"total"`
	TotalUnpaid decimal.Decimal `json:"total_unpaid"` // sum of outstanding (belum lunas)
	Page        int             `json:"page"`
	Limit       int             `json:"limit"`
}

func (inv *Invoice) ToResponse(items []ItemResponse) *Response {
	return &Response{
		ID:                inv.ID,
		SupplierID:        inv.SupplierID,
		SupplierName:      inv.SupplierName,
		StoreID:           inv.StoreID,
		StoreName:         inv.StoreName,
		InvoiceNo:         inv.InvoiceNo,
		InvoiceDate:       inv.InvoiceDate,
		GoodsReceivedDate: inv.GoodsReceivedDate,
		Amount:            inv.Amount,
		PaidAmount:        inv.PaidAmount,
		Outstanding:       inv.Amount.Sub(inv.PaidAmount),
		DueDate:           inv.DueDate,
		Status:            inv.Status,
		PaidAt:            inv.PaidAt,
		Note:              inv.Note,
		Items:             items,
		CreatedAt:         inv.CreatedAt,
	}
}
