package salesperson

import (
	"time"

	"github.com/shopspring/decimal"
)

type Salesperson struct {
	ID             uint            `db:"id"              json:"id"`
	Name           string          `db:"name"            json:"name"`
	CommissionRate decimal.Decimal `db:"commission_rate" json:"commission_rate"`
	IsActive       bool            `db:"is_active"       json:"is_active"`
	CreatedAt      time.Time       `db:"created_at"      json:"created_at"`
}

type CreateInput struct {
	Name           string          `json:"name" binding:"required,min=1,max=100"`
	CommissionRate decimal.Decimal `json:"commission_rate"`
}

type UpdateInput struct {
	Name           *string          `json:"name"`
	CommissionRate *decimal.Decimal `json:"commission_rate"`
	IsActive       *bool            `json:"is_active"`
}
