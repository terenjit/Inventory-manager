package auth

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"inventory-manager/internal/apperr"
)

type Repository interface {
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByID(ctx context.Context, id uint) (*User, error)
	Create(ctx context.Context, u *User) (uint, error)
	Update(ctx context.Context, id uint, input UpdateUserInput) error
	List(ctx context.Context) ([]*User, error)
}

type sqlxRepo struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &sqlxRepo{db: db}
}

func (r *sqlxRepo) GetByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := r.db.GetContext(ctx, &u, "SELECT * FROM users WHERE username = ?", username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("user not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &u, nil
}

func (r *sqlxRepo) GetByID(ctx context.Context, id uint) (*User, error) {
	var u User
	err := r.db.GetContext(ctx, &u, "SELECT * FROM users WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("user not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &u, nil
}

func (r *sqlxRepo) Create(ctx context.Context, u *User) (uint, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO users (username, full_name, password_hash, role, store_id) VALUES (?, ?, ?, ?, ?)",
		u.Username, u.FullName, u.PasswordHash, u.Role, u.StoreID,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return 0, ErrUsernameTaken
		}
		return 0, apperr.Internal(err)
	}
	id, _ := res.LastInsertId()
	return uint(id), nil
}

func (r *sqlxRepo) Update(ctx context.Context, id uint, input UpdateUserInput) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users
		    SET full_name = COALESCE(?, full_name),
		        is_active = COALESCE(?, is_active)
		  WHERE id = ?`,
		input.FullName, input.IsActive, id,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (r *sqlxRepo) List(ctx context.Context) ([]*User, error) {
	var users []*User
	if err := r.db.SelectContext(ctx, &users, "SELECT * FROM users ORDER BY created_at DESC"); err != nil {
		return nil, apperr.Internal(err)
	}
	return users, nil
}
