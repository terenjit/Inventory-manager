package supplierbill

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"inventory-manager/internal/apperr"
)

type Repository interface {
	List(ctx context.Context) ([]*Bill, error)
	GetByID(ctx context.Context, id uint) (*Bill, error)
	Create(ctx context.Context, input CreateInput, createdBy uint) (uint, error)
	Update(ctx context.Context, id uint, input UpdateInput) error
	SoftDelete(ctx context.Context, id uint) error
}

type sqlxRepo struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &sqlxRepo{db: db} }

func (r *sqlxRepo) List(ctx context.Context) ([]*Bill, error) {
	var rows []*Bill
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, supplier_name, invoice_no, invoice_date,
		        received_date, amount, due_date, status, created_at
		   FROM supplier_bills
		  WHERE deleted_at IS NULL
		  ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return rows, nil
}

func (r *sqlxRepo) GetByID(ctx context.Context, id uint) (*Bill, error) {
	var b Bill
	err := r.db.GetContext(ctx, &b,
		`SELECT id, supplier_name, invoice_no, invoice_date,
		        received_date, amount, due_date, status, created_at
		   FROM supplier_bills WHERE id = ? AND deleted_at IS NULL`, id,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("supplier bill not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &b, nil
}

func (r *sqlxRepo) Create(ctx context.Context, input CreateInput, createdBy uint) (uint, error) {
	var receivedDate, dueDate interface{}
	if input.ReceivedDate != "" {
		receivedDate = input.ReceivedDate
	}
	if input.DueDate != "" {
		dueDate = input.DueDate
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO supplier_bills
		   (supplier_name, invoice_no, invoice_date, received_date, amount, due_date, status, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Supplier, input.InvoiceNo, input.InvoiceDate,
		receivedDate, input.Amount, dueDate, dbStatus(input.Status), createdBy,
	)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	id, _ := res.LastInsertId()
	return uint(id), nil
}

func (r *sqlxRepo) Update(ctx context.Context, id uint, input UpdateInput) error {
	var receivedDate, dueDate interface{}
	if input.ReceivedDate != "" {
		receivedDate = input.ReceivedDate
	}
	if input.DueDate != "" {
		dueDate = input.DueDate
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE supplier_bills
		    SET supplier_name = ?, invoice_no = ?, invoice_date = ?,
		        received_date = ?, amount = ?, due_date = ?, status = ?
		  WHERE id = ? AND deleted_at IS NULL`,
		input.Supplier, input.InvoiceNo, input.InvoiceDate,
		receivedDate, input.Amount, dueDate, dbStatus(input.Status), id,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (r *sqlxRepo) SoftDelete(ctx context.Context, id uint) error {
	res, err := r.db.ExecContext(ctx,
		"UPDATE supplier_bills SET deleted_at = NOW(3) WHERE id = ? AND deleted_at IS NULL", id,
	)
	if err != nil {
		return apperr.Internal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("supplier bill not found")
	}
	return nil
}
