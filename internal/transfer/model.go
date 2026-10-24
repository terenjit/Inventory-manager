package transfer

import (
	"time"

	"github.com/shopspring/decimal"
)

type Transfer struct {
	ID             uint       `db:"id"`
	TransferNo     string     `db:"transfer_no"`
	FromStoreID    uint       `db:"from_store_id"`
	FromStoreName  string     `db:"from_store_name"`
	FromStoreCode  string     `db:"from_store_code"`
	ToStoreID      uint       `db:"to_store_id"`
	ToStoreName    string     `db:"to_store_name"`
	Status         string     `db:"status"`
	Note           *string    `db:"note"`
	SentBy         uint       `db:"sent_by"`
	SentByName     string     `db:"sent_by_name"`
	SentAt         time.Time  `db:"sent_at"`
	ReceivedBy     *uint      `db:"received_by"`
	ReceivedByName *string    `db:"received_by_name"`
	ReceivedAt     *time.Time `db:"received_at"`
}

type TransferItem struct {
	TransferID  uint            `db:"transfer_id"`
	ProductID   uint            `db:"product_id"`
	ProductCode string          `db:"product_code"`
	ProductName string          `db:"product_name"`
	Unit        string          `db:"unit"`
	Qty         decimal.Decimal `db:"qty"`
}

type CreateInput struct {
	FromStoreID *uint             `json:"from_store_id"`
	ToStoreID   uint              `json:"to_store_id" binding:"required"`
	Note        *string           `json:"note"`
	Items       []ItemInput       `json:"items" binding:"required,min=1"`
}

type ItemInput struct {
	ProductID uint            `json:"product_id" binding:"required"`
	Qty       decimal.Decimal `json:"qty"`
}

type ListFilter struct {
	Direction string // "incoming", "outgoing", ""
	Status    string
	StoreID   uint
	Page      int
	Limit     int
}

type ItemResponse struct {
	ProductID   uint            `json:"product_id"`
	ProductCode string          `json:"product_code"`
	ProductName string          `json:"product_name"`
	Unit        string          `json:"unit"`
	Qty         decimal.Decimal `json:"qty"`
}

type Response struct {
	ID             uint           `json:"id"`
	TransferNo     string         `json:"transfer_no"`
	FromStoreID    uint           `json:"from_store_id"`
	FromStoreName  string         `json:"from_store_name"`
	ToStoreID      uint           `json:"to_store_id"`
	ToStoreName    string         `json:"to_store_name"`
	Status         string         `json:"status"`
	Note           *string        `json:"note,omitempty"`
	SentBy         uint           `json:"sent_by"`
	SentByName     string         `json:"sent_by_name"`
	SentAt         time.Time      `json:"sent_at"`
	ReceivedBy     *uint          `json:"received_by,omitempty"`
	ReceivedByName *string        `json:"received_by_name,omitempty"`
	ReceivedAt     *time.Time     `json:"received_at,omitempty"`
	Items          []ItemResponse `json:"items,omitempty"`
}

type ListResult struct {
	Data  []*Response `json:"data"`
	Total int         `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

func (t *Transfer) ToResponse(items []ItemResponse) *Response {
	return &Response{
		ID:             t.ID,
		TransferNo:     t.TransferNo,
		FromStoreID:    t.FromStoreID,
		FromStoreName:  t.FromStoreName,
		ToStoreID:      t.ToStoreID,
		ToStoreName:    t.ToStoreName,
		Status:         t.Status,
		Note:           t.Note,
		SentBy:         t.SentBy,
		SentByName:     t.SentByName,
		SentAt:         t.SentAt,
		ReceivedBy:     t.ReceivedBy,
		ReceivedByName: t.ReceivedByName,
		ReceivedAt:     t.ReceivedAt,
		Items:          items,
	}
}
