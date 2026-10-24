package auth

import "time"

type User struct {
	ID           uint      `db:"id"`
	Username     string    `db:"username"`
	FullName     string    `db:"full_name"`
	PasswordHash string    `db:"password_hash"`
	Role         string    `db:"role"`
	StoreID      *uint     `db:"store_id"`
	IsActive     bool      `db:"is_active"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// Actor is attached to every authenticated request context.
type Actor struct {
	UserID  uint
	Role    string
	StoreID *uint
}

func (a Actor) IsAdmin() bool   { return a.Role == "admin" }
func (a Actor) IsCashier() bool { return a.Role == "cashier" }

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type CreateUserInput struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	FullName string `json:"full_name" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"required,oneof=admin cashier"`
	StoreID  *uint  `json:"store_id"`
}

type UpdateUserInput struct {
	FullName *string `json:"full_name"`
	IsActive *bool   `json:"is_active"`
}

type UserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	StoreID   *uint     `json:"store_id,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		FullName:  u.FullName,
		Role:      u.Role,
		StoreID:   u.StoreID,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
	}
}
