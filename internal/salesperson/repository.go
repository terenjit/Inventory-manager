package salesperson

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"inventory-manager/internal/apperr"
)

type Repository interface {
	List(ctx context.Context) ([]*Salesperson, error)
	GetByID(ctx context.Context, id uint) (*Salesperson, error)
	Create(ctx context.Context, input CreateInput) (uint, error)
	Update(ctx context.Context, id uint, input UpdateInput) error
}

type sqlxRepo struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &sqlxRepo{db: db} }

func (r *sqlxRepo) List(ctx context.Context) ([]*Salesperson, error) {
	list := []*Salesperson{}
	if err := r.db.SelectContext(ctx, &list,
		"SELECT * FROM salespeople WHERE is_active = TRUE ORDER BY name"); err != nil {
		return nil, apperr.Internal(err)
	}
	return list, nil
}

func (r *sqlxRepo) GetByID(ctx context.Context, id uint) (*Salesperson, error) {
	var s Salesperson
	err := r.db.GetContext(ctx, &s, "SELECT * FROM salespeople WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("salesperson not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &s, nil
}

func (r *sqlxRepo) Create(ctx context.Context, input CreateInput) (uint, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO salespeople (name, commission_rate) VALUES (?, ?)",
		input.Name, input.CommissionRate,
	)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	id, _ := res.LastInsertId()
	return uint(id), nil
}

func (r *sqlxRepo) Update(ctx context.Context, id uint, input UpdateInput) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE salespeople
		    SET name            = COALESCE(?, name),
		        commission_rate = COALESCE(?, commission_rate),
		        is_active       = COALESCE(?, is_active)
		  WHERE id = ?`,
		input.Name, input.CommissionRate, input.IsActive, id,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}
