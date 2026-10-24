package opname

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"inventory-manager/internal/apperr"
)

var wib = time.FixedZone("WIB", 7*3600)

type Repository interface {
	NextDocNo(ctx context.Context, tx *sqlx.Tx, storeID uint, storeCode string) (string, error)
	Create(ctx context.Context, tx *sqlx.Tx, o *Opname) (uint, error)
	GetByID(ctx context.Context, id uint) (*Opname, error)
	GetLines(ctx context.Context, opnameID uint) ([]*OpnameLine, error)
	List(ctx context.Context, f ListFilter) ([]*Opname, int, error)
	AddLine(ctx context.Context, opnameID, productID uint, systemQty decimal.Decimal) error
	UpdateLine(ctx context.Context, opnameID, productID uint, input UpdateLineInput) error
	Finalize(ctx context.Context, tx *sqlx.Tx, id, by uint) error
	Cancel(ctx context.Context, id uint) error
}

type sqlxRepo struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &sqlxRepo{db: db} }

func (r *sqlxRepo) NextDocNo(ctx context.Context, tx *sqlx.Tx, storeID uint, storeCode string) (string, error) {
	now := time.Now().In(wib)
	period := now.Format("2006-01-02")
	_, err := tx.ExecContext(ctx,
		`INSERT INTO doc_sequences (store_id, doc_type, period, last_no) VALUES (?, 'opname', ?, 1)
		 ON DUPLICATE KEY UPDATE last_no = last_no + 1`,
		storeID, period,
	)
	if err != nil {
		return "", apperr.Internal(err)
	}
	var lastNo int
	if err := tx.GetContext(ctx, &lastNo,
		`SELECT last_no FROM doc_sequences WHERE store_id = ? AND doc_type = 'opname' AND period = ?`,
		storeID, period,
	); err != nil {
		return "", apperr.Internal(err)
	}
	return fmt.Sprintf("OPN/%s/%s/%04d", storeCode, now.Format("20060102"), lastNo), nil
}

func (r *sqlxRepo) Create(ctx context.Context, tx *sqlx.Tx, o *Opname) (uint, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO stock_opnames (opname_no, store_id, opname_date, note, started_by)
		 VALUES (?, ?, ?, ?, ?)`,
		o.OpnameNo, o.StoreID, o.OpnameDate.Format("2006-01-02"), o.Note, o.StartedBy,
	)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return 0, apperr.Conflict("a draft opname already exists for this store")
		}
		return 0, apperr.Internal(err)
	}
	id, _ := res.LastInsertId()
	return uint(id), nil
}

func (r *sqlxRepo) GetByID(ctx context.Context, id uint) (*Opname, error) {
	var o Opname
	err := r.db.GetContext(ctx, &o,
		`SELECT op.*,
		        st.name AS store_name,
		        st.code AS store_code,
		        u.full_name AS started_by_name
		   FROM stock_opnames op
		   JOIN stores st ON st.id = op.store_id
		   JOIN users  u  ON u.id  = op.started_by
		  WHERE op.id = ?`, id,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("opname not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &o, nil
}

func (r *sqlxRepo) GetLines(ctx context.Context, opnameID uint) ([]*OpnameLine, error) {
	var lines []*OpnameLine
	if err := r.db.SelectContext(ctx, &lines,
		`SELECT ol.opname_id, ol.product_id, ol.system_qty, ol.counted_qty, ol.note,
		        p.code AS product_code, p.name AS product_name, p.unit, p.cost_price
		   FROM stock_opname_lines ol
		   JOIN products p ON p.id = ol.product_id
		  WHERE ol.opname_id = ?
		  ORDER BY p.name`, opnameID,
	); err != nil {
		return nil, apperr.Internal(err)
	}
	return lines, nil
}

func (r *sqlxRepo) List(ctx context.Context, f ListFilter) ([]*Opname, int, error) {
	where, args := buildWhere(f)

	var total int
	if err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM stock_opnames op"+where, args...); err != nil {
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

	query := `SELECT op.*,
	                 st.name AS store_name,
	                 st.code AS store_code,
	                 u.full_name AS started_by_name
	            FROM stock_opnames op
	            JOIN stores st ON st.id = op.store_id
	            JOIN users  u  ON u.id  = op.started_by` +
		where + ` ORDER BY op.started_at DESC LIMIT ? OFFSET ?`

	args = append(args, limit, (page-1)*limit)
	var opnames []*Opname
	if err := r.db.SelectContext(ctx, &opnames, query, args...); err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return opnames, total, nil
}

func (r *sqlxRepo) AddLine(ctx context.Context, opnameID, productID uint, systemQty decimal.Decimal) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO stock_opname_lines (opname_id, product_id, system_qty) VALUES (?, ?, ?)",
		opnameID, productID, systemQty,
	)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return apperr.Conflict("product already added to this opname")
		}
		return apperr.Internal(err)
	}
	return nil
}

func (r *sqlxRepo) UpdateLine(ctx context.Context, opnameID, productID uint, input UpdateLineInput) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE stock_opname_lines
		    SET counted_qty = ?, note = COALESCE(?, note)
		  WHERE opname_id = ? AND product_id = ?`,
		input.CountedQty, input.Note, opnameID, productID,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (r *sqlxRepo) Finalize(ctx context.Context, tx *sqlx.Tx, id, by uint) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE stock_opnames
		    SET status = 'finalized', finalized_by = ?, finalized_at = NOW(3)
		  WHERE id = ? AND status = 'draft'`,
		by, id,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.Conflict("opname is not in draft status")
	}
	return nil
}

func (r *sqlxRepo) Cancel(ctx context.Context, id uint) error {
	res, err := r.db.ExecContext(ctx,
		"UPDATE stock_opnames SET status = 'cancelled' WHERE id = ? AND status = 'draft'",
		id,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.Conflict("opname is not in draft status")
	}
	return nil
}

func buildWhere(f ListFilter) (string, []any) {
	conditions := []string{}
	args := []any{}
	if f.StoreID > 0 {
		conditions = append(conditions, "op.store_id = ?")
		args = append(args, f.StoreID)
	}
	if f.Status != "" {
		conditions = append(conditions, "op.status = ?")
		args = append(args, f.Status)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

// Needed to satisfy import.
var _ = decimal.Zero
