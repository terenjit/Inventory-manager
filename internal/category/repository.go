package category

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"inventory-manager/internal/apperr"
)

type Repository interface {
	List(ctx context.Context) ([]*Category, error)
	GetByID(ctx context.Context, id uint) (*Category, error)
	Create(ctx context.Context, name string) (uint, error)
	Update(ctx context.Context, id uint, name string) error
	Delete(ctx context.Context, id uint) error
}

type sqlxRepo struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &sqlxRepo{db: db} }

func (r *sqlxRepo) List(ctx context.Context) ([]*Category, error) {
	var cats []*Category
	if err := r.db.SelectContext(ctx, &cats, "SELECT * FROM categories ORDER BY name"); err != nil {
		return nil, apperr.Internal(err)
	}
	return cats, nil
}

func (r *sqlxRepo) GetByID(ctx context.Context, id uint) (*Category, error) {
	var c Category
	err := r.db.GetContext(ctx, &c, "SELECT * FROM categories WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("category not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &c, nil
}

func (r *sqlxRepo) Create(ctx context.Context, name string) (uint, error) {
	res, err := r.db.ExecContext(ctx, "INSERT INTO categories (name) VALUES (?)", name)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return 0, apperr.Conflict("category name already exists")
		}
		return 0, apperr.Internal(err)
	}
	id, _ := res.LastInsertId()
	return uint(id), nil
}

func (r *sqlxRepo) Update(ctx context.Context, id uint, name string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE categories SET name = ? WHERE id = ?", name, id)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return apperr.Conflict("category name already exists")
		}
		return apperr.Internal(err)
	}
	return nil
}

func (r *sqlxRepo) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM categories WHERE id = ?", id)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}
