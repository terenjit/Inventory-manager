package opname

import (
	"time"

	"github.com/shopspring/decimal"
)

type Opname struct {
	ID            uint       `db:"id"`
	OpnameNo      string     `db:"opname_no"`
	StoreID       uint       `db:"store_id"`
	StoreName     string     `db:"store_name"`
	StoreCode     string     `db:"store_code"`
	OpnameDate    time.Time  `db:"opname_date"`
	Status        string     `db:"status"` // draft, finalized, cancelled
	Note          *string    `db:"note"`
	StartedBy     uint       `db:"started_by"`
	StartedByName string     `db:"started_by_name"`
	StartedAt     time.Time  `db:"started_at"`
	FinalizedBy   *uint      `db:"finalized_by"`
	FinalizedAt   *time.Time `db:"finalized_at"`
	DraftStoreID  *uint      `db:"draft_store_id"` // generated column, ignored in app logic
}

type OpnameLine struct {
	OpnameID    uint             `db:"opname_id"`
	ProductID   uint             `db:"product_id"`
	ProductCode string           `db:"product_code"`
	ProductName string           `db:"product_name"`
	Unit        string           `db:"unit"`
	CostPrice   decimal.Decimal  `db:"cost_price"`
	SystemQty   decimal.Decimal  `db:"system_qty"`
	CountedQty  *decimal.Decimal `db:"counted_qty"` // NULL = not counted yet
	Note        *string          `db:"note"`
}

type CreateInput struct {
	StoreID    *uint   `json:"store_id"`
	OpnameDate *string `json:"opname_date"` // YYYY-MM-DD, defaults to today
	Note       *string `json:"note"`
}

type AddLineInput struct {
	ProductID uint `json:"product_id" binding:"required"`
}

type UpdateLineInput struct {
	CountedQty decimal.Decimal `json:"counted_qty"`
	Note       *string         `json:"note"`
}

type ListFilter struct {
	StoreID uint
	Status  string
	Page    int
	Limit   int
}

// SummaryLine shows selisih (difference) for products with discrepancies.
type SummaryLine struct {
	ProductID    uint            `json:"product_id"`
	ProductCode  string          `json:"product_code"`
	ProductName  string          `json:"product_name"`
	Unit         string          `json:"unit"`
	SystemQty    decimal.Decimal `json:"system_qty"`
	CountedQty   decimal.Decimal `json:"counted_qty"`
	Selisih      decimal.Decimal `json:"selisih"`       // counted - system
	CostPrice    decimal.Decimal `json:"cost_price"`
	NilaiSelisih decimal.Decimal `json:"nilai_selisih"` // |selisih| * cost_price (negative = loss)
}

type LineResponse struct {
	ProductID   uint             `json:"product_id"`
	ProductCode string           `json:"product_code"`
	ProductName string           `json:"product_name"`
	Unit        string           `json:"unit"`
	SystemQty   decimal.Decimal  `json:"system_qty"`
	CountedQty  *decimal.Decimal `json:"counted_qty,omitempty"`
	Selisih     *decimal.Decimal `json:"selisih,omitempty"` // nil if not counted
	Note        *string          `json:"note,omitempty"`
}

type Response struct {
	ID            uint           `json:"id"`
	OpnameNo      string         `json:"opname_no"`
	StoreID       uint           `json:"store_id"`
	StoreName     string         `json:"store_name"`
	OpnameDate    time.Time      `json:"opname_date"`
	Status        string         `json:"status"`
	Note          *string        `json:"note,omitempty"`
	StartedBy     uint           `json:"started_by"`
	StartedByName string         `json:"started_by_name"`
	StartedAt     time.Time      `json:"started_at"`
	FinalizedBy   *uint          `json:"finalized_by,omitempty"`
	FinalizedAt   *time.Time     `json:"finalized_at,omitempty"`
	Lines         []LineResponse `json:"lines,omitempty"`
}

type SummaryResponse struct {
	OpnameID         uint            `json:"opname_id"`
	OpnameNo         string          `json:"opname_no"`
	StoreName        string          `json:"store_name"`
	OpnameDate       time.Time       `json:"opname_date"`
	TotalNilaiSelisih decimal.Decimal `json:"total_nilai_selisih"`
	Lines            []SummaryLine   `json:"lines"`
}

type ListResult struct {
	Data  []*Response `json:"data"`
	Total int         `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

func (o *Opname) ToResponse(lines []LineResponse) *Response {
	return &Response{
		ID:            o.ID,
		OpnameNo:      o.OpnameNo,
		StoreID:       o.StoreID,
		StoreName:     o.StoreName,
		OpnameDate:    o.OpnameDate,
		Status:        o.Status,
		Note:          o.Note,
		StartedBy:     o.StartedBy,
		StartedByName: o.StartedByName,
		StartedAt:     o.StartedAt,
		FinalizedBy:   o.FinalizedBy,
		FinalizedAt:   o.FinalizedAt,
		Lines:         lines,
	}
}
