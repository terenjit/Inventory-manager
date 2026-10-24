package supplier

import "time"

type Supplier struct {
	ID        uint      `db:"id"         json:"id"`
	Name      string    `db:"name"       json:"name"`
	Phone     *string   `db:"phone"      json:"phone,omitempty"`
	Address   *string   `db:"address"    json:"address,omitempty"`
	Note      *string   `db:"note"       json:"note,omitempty"`
	IsActive  bool      `db:"is_active"  json:"is_active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type CreateInput struct {
	Name    string  `json:"name" binding:"required,min=1,max=120"`
	Phone   *string `json:"phone"`
	Address *string `json:"address"`
	Note    *string `json:"note"`
}

type UpdateInput struct {
	Name    *string `json:"name"`
	Phone   *string `json:"phone"`
	Address *string `json:"address"`
	Note    *string `json:"note"`
	IsActive *bool  `json:"is_active"`
}
