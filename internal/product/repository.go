package product

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"inventory-manager/internal/apperr"
)

type Repository interface {
	List(ctx context.Context, f ListFilter) ([]*Product, int, error)
	GetByID(ctx context.Context, id uint) (*Product, error)
	GetByCode(ctx context.Context, code string) (*Product, error)
	Create(ctx context.Context, input CreateInput) (uint, error)
	Update(ctx context.Context, id uint, input UpdateInput) error
	GetStock(ctx context.Context, productIDs []uint) (map[uint][]StockInfo, error)
}

type sqlxRepo struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &sqlxRepo{db: db} }

func (r *sqlxRepo) List(ctx context.Context, f ListFilter) ([]*Product, int, error) {
	where, args := buildWhere(f)

	var total int
	if err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM products p"+where, args...); err != nil {
		return nil, 0, apperr.Internal(err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	query := `SELECT p.*,
	                 c.name  AS category_name,
	                 s.name  AS supplier_name,
	                 st.name AS store_name
	            FROM products p
	            LEFT JOIN categories c  ON c.id  = p.category_id
	            LEFT JOIN suppliers  s  ON s.id  = p.supplier_id
	            LEFT JOIN stores     st ON st.id = p.store_id` +
		where + ` ORDER BY p.name LIMIT ? OFFSET ?`

	args = append(args, limit, offset)
	var products []*Product
	if err := r.db.SelectContext(ctx, &products, query, args...); err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return products, total, nil
}

func (r *sqlxRepo) GetByID(ctx context.Context, id uint) (*Product, error) {
	var p Product
	err := r.db.GetContext(ctx, &p,
		`SELECT p.*,
		        c.name  AS category_name,
		        s.name  AS supplier_name,
		        st.name AS store_name
		   FROM products p
		   LEFT JOIN categories c  ON c.id  = p.category_id
		   LEFT JOIN suppliers  s  ON s.id  = p.supplier_id
		   LEFT JOIN stores     st ON st.id = p.store_id
		  WHERE p.id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("product not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &p, nil
}

func (r *sqlxRepo) GetByCode(ctx context.Context, code string) (*Product, error) {
	var p Product
	err := r.db.GetContext(ctx, &p,
		`SELECT p.*,
		        c.name  AS category_name,
		        s.name  AS supplier_name,
		        st.name AS store_name
		   FROM products p
		   LEFT JOIN categories c  ON c.id  = p.category_id
		   LEFT JOIN suppliers  s  ON s.id  = p.supplier_id
		   LEFT JOIN stores     st ON st.id = p.store_id
		  WHERE UPPER(p.code) = UPPER(?)`, code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("product not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &p, nil
}

func (r *sqlxRepo) Create(ctx context.Context, input CreateInput) (uint, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO products (code, name, category_id, supplier_id, store_id, unit, sell_price, cost_price, min_stock)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.ToUpper(input.Code), input.Name, input.CategoryID, input.SupplierID,
		input.StoreID, input.Unit, input.SellPrice, input.CostPrice, input.MinStock,
	)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return 0, apperr.Conflict("product code already exists")
		}
		return 0, apperr.Internal(err)
	}
	id, _ := res.LastInsertId()
	return uint(id), nil
}

func (r *sqlxRepo) Update(ctx context.Context, id uint, input UpdateInput) error {
	setParts := []string{}
	args := []any{}

	if input.Name != nil {
		setParts = append(setParts, "name = ?")
		args = append(args, *input.Name)
	}
	if input.CategoryID != nil {
		setParts = append(setParts, "category_id = ?")
		args = append(args, *input.CategoryID)
	}
	if input.SupplierID != nil {
		setParts = append(setParts, "supplier_id = ?")
		args = append(args, *input.SupplierID)
	}
	if input.StoreID != nil {
		setParts = append(setParts, "store_id = ?")
		args = append(args, *input.StoreID)
	}
	if input.Unit != nil {
		setParts = append(setParts, "unit = ?")
		args = append(args, *input.Unit)
	}
	if input.SellPrice != nil {
		setParts = append(setParts, "sell_price = ?")
		args = append(args, *input.SellPrice)
	}
	if input.CostPrice != nil {
		setParts = append(setParts, "cost_price = ?")
		args = append(args, *input.CostPrice)
	}
	if input.MinStock != nil {
		setParts = append(setParts, "min_stock = ?")
		args = append(args, *input.MinStock)
	}
	if input.IsActive != nil {
		setParts = append(setParts, "is_active = ?")
		args = append(args, *input.IsActive)
	}
	if len(setParts) == 0 {
		return nil
	}
	args = append(args, id)
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf("UPDATE products SET %s WHERE id = ?", strings.Join(setParts, ", ")),
		args...,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (r *sqlxRepo) GetStock(ctx context.Context, productIDs []uint) (map[uint][]StockInfo, error) {
	if len(productIDs) == 0 {
		return map[uint][]StockInfo{}, nil
	}
	query, args, err := sqlx.In(
		`SELECT i.product_id, i.store_id, st.name AS store_name, i.qty
		   FROM inventory i
		   JOIN stores st ON st.id = i.store_id
		  WHERE i.product_id IN (?)
		  ORDER BY i.store_id`, productIDs,
	)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	type stockRow struct {
		ProductID uint `db:"product_id"`
		StockInfo
	}
	var rows []stockRow
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return nil, apperr.Internal(err)
	}

	result := make(map[uint][]StockInfo)
	for _, row := range rows {
		result[row.ProductID] = append(result[row.ProductID], row.StockInfo)
	}
	return result, nil
}

func buildWhere(f ListFilter) (string, []any) {
	conditions := []string{"p.is_active = TRUE"}
	args := []any{}

	if f.Search != "" {
		conditions = append(conditions, "(UPPER(p.code) LIKE ? OR UPPER(p.name) LIKE ?)")
		pattern := "%" + strings.ToUpper(f.Search) + "%"
		args = append(args, pattern, pattern)
	}
	if f.CategoryID > 0 {
		conditions = append(conditions, "p.category_id = ?")
		args = append(args, f.CategoryID)
	}
	if f.StoreID > 0 {
		conditions = append(conditions, "p.store_id = ?")
		args = append(args, f.StoreID)
	}
	if f.InStockOnly && f.StoreID > 0 {
		conditions = append(conditions,
			"COALESCE((SELECT qty FROM inventory WHERE product_id = p.id AND store_id = ?), 0) > 0")
		args = append(args, f.StoreID)
	}
	if f.LowStock && f.StoreID > 0 {
		conditions = append(conditions,
			"COALESCE((SELECT qty FROM inventory WHERE product_id = p.id AND store_id = ?), 0) < p.min_stock")
		args = append(args, f.StoreID)
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
}
