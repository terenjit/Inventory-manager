package product

import (
	"time"

	"github.com/shopspring/decimal"
)

type Product struct {
	ID           uint            `db:"id"`
	Code         string          `db:"code"`
	Name         string          `db:"name"`
	CategoryID   *uint           `db:"category_id"`
	CategoryName *string         `db:"category_name"`
	SupplierID   *uint           `db:"supplier_id"`
	SupplierName *string         `db:"supplier_name"`
	StoreID      *uint           `db:"store_id"`
	StoreName    *string         `db:"store_name"`
	Unit         string          `db:"unit"`
	SellPrice    decimal.Decimal `db:"sell_price"`
	CostPrice    decimal.Decimal `db:"cost_price"`
	MinStock     decimal.Decimal `db:"min_stock"`
	IsActive     bool            `db:"is_active"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

type StockInfo struct {
	StoreID   uint            `db:"store_id" json:"store_id"`
	StoreName string          `db:"store_name" json:"store_name"`
	Qty       decimal.Decimal `db:"qty"       json:"qty"`
}

// CashierResponse excludes cost_price, margin, supplier — enforced server-side.
type CashierResponse struct {
	ID           uint            `json:"id"`
	Code         string          `json:"code"`
	Name         string          `json:"name"`
	CategoryID   *uint           `json:"category_id,omitempty"`
	CategoryName *string         `json:"category_name,omitempty"`
	StoreID      *uint           `json:"store_id,omitempty"`
	StoreName    *string         `json:"store_name,omitempty"`
	Unit         string          `json:"unit"`
	SellPrice    decimal.Decimal `json:"sell_price"`
	MinStock     decimal.Decimal `json:"min_stock"`
	IsActive     bool            `json:"is_active"`
	Stock        []StockInfo     `json:"stock"`
	IsLowStock   bool            `json:"is_low_stock"`
}

type AdminResponse struct {
	CashierResponse
	SupplierID   *uint           `json:"supplier_id,omitempty"`
	SupplierName *string         `json:"supplier_name,omitempty"`
	CostPrice    decimal.Decimal `json:"cost_price"`
	Margin       decimal.Decimal `json:"margin"`
}

type CreateInput struct {
	Code       string          `json:"code"       binding:"required,min=1,max=50"`
	Name       string          `json:"name"       binding:"required,min=1,max=150"`
	CategoryID *uint           `json:"category_id"`
	SupplierID *uint           `json:"supplier_id"`
	StoreID    *uint           `json:"store_id"`
	Unit       string          `json:"unit"       binding:"required,oneof=pcs meter"`
	SellPrice  decimal.Decimal `json:"sell_price"`
	CostPrice  decimal.Decimal `json:"cost_price"`
	MinStock   decimal.Decimal `json:"min_stock"`
}

type UpdateInput struct {
	Name       *string          `json:"name"`
	CategoryID *uint            `json:"category_id"`
	SupplierID *uint            `json:"supplier_id"`
	StoreID    *uint            `json:"store_id"`
	Unit       *string          `json:"unit" binding:"omitempty,oneof=pcs meter"`
	SellPrice  *decimal.Decimal `json:"sell_price"`
	CostPrice  *decimal.Decimal `json:"cost_price"`
	MinStock   *decimal.Decimal `json:"min_stock"`
	IsActive   *bool            `json:"is_active"`
}

type ListFilter struct {
	Search      string
	CategoryID  uint
	LowStock    bool
	InStockOnly bool // when true, only products with qty > 0 at StoreID
	StoreID     uint // for low_stock filter and in_stock_only filter
	Page        int
	Limit       int
}

type ListResult struct {
	Data  interface{} `json:"data"`
	Total int         `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}
