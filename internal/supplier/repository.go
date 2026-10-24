package supplier

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"inventory-manager/internal/apperr"
)

type Repository interface {
	List(ctx context.Context) ([]*Supplier, error)
	GetByID(ctx context.Context, id uint) (*Supplier, error)
	Create(ctx context.Context, input CreateInput) (uint, error)
	Update(ctx context.Context, id uint, input UpdateInput) error
}

type sqlxRepo struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &sqlxRepo{db: db} }

func (r *sqlxRepo) List(ctx context.Context) ([]*Supplier, error) {
	var suppliers []*Supplier
	if err := r.db.SelectContext(ctx, &suppliers,
		"SELECT * FROM suppliers WHERE is_active = TRUE ORDER BY name"); err != nil {
		return nil, apperr.Internal(err)
	}
	return suppliers, nil
}

func (r *sqlxRepo) GetByID(ctx context.Context, id uint) (*Supplier, error) {
	var s Supplier
	err := r.db.GetContext(ctx, &s, "SELECT * FROM suppliers WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("supplier not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &s, nil
}

func (r *sqlxRepo) Create(ctx context.Context, input CreateInput) (uint, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO suppliers (name, phone, address, note) VALUES (?, ?, ?, ?)",
		input.Name, input.Phone, input.Address, input.Note,
	)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return 0, apperr.Conflict("supplier name already exists")
		}
		return 0, apperr.Internal(err)
	}
	id, _ := res.LastInsertId()
	return uint(id), nil
}

func (r *sqlxRepo) Update(ctx context.Context, id uint, input UpdateInput) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE suppliers
		    SET name     = COALESCE(?, name),
		        phone    = COALESCE(?, phone),
		        address  = COALESCE(?, address),
		        note     = COALESCE(?, note),
		        is_active = COALESCE(?, is_active)
		  WHERE id = ?`,
		input.Name, input.Phone, input.Address, input.Note, input.IsActive, id,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}
