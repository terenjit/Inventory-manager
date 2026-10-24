package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"inventory-manager/internal/apperr"
)

type Repository interface {
	GetByID(ctx context.Context, id uint) (*Store, error)
	GetByName(ctx context.Context, name string) (*Store, error)
	List(ctx context.Context) ([]*Store, error)
	Create(ctx context.Context, s *Store) (uint, error)
}

type sqlxRepo struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &sqlxRepo{db: db}
}

func (r *sqlxRepo) GetByID(ctx context.Context, id uint) (*Store, error) {
	var s Store
	err := r.db.GetContext(ctx, &s, "SELECT * FROM stores WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("store not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &s, nil
}

func (r *sqlxRepo) GetByName(ctx context.Context, name string) (*Store, error) {
	var s Store
	err := r.db.GetContext(ctx, &s, "SELECT * FROM stores WHERE name = ? AND is_active = TRUE", name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("store not found: " + name)
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &s, nil
}

func (r *sqlxRepo) List(ctx context.Context) ([]*Store, error) {
	var stores []*Store
	if err := r.db.SelectContext(ctx, &stores, "SELECT * FROM stores WHERE is_active = TRUE ORDER BY name"); err != nil {
		return nil, apperr.Internal(err)
	}
	return stores, nil
}

func (r *sqlxRepo) Create(ctx context.Context, s *Store) (uint, error) {
	res, err := r.db.ExecContext(ctx, "INSERT INTO stores (code, name) VALUES (?, ?)", s.Code, s.Name)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	id, _ := res.LastInsertId()
	return uint(id), nil
}
