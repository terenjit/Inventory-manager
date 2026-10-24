package supplierbill

import (
	"context"

	"inventory-manager/internal/apperr"
	"inventory-manager/internal/auth"
)

type Service interface {
	List(ctx context.Context) ([]Response, error)
	GetByID(ctx context.Context, id uint) (Response, error)
	Create(ctx context.Context, actor auth.Actor, input CreateInput) (Response, error)
	Update(ctx context.Context, actor auth.Actor, id uint, input UpdateInput) (Response, error)
	Delete(ctx context.Context, actor auth.Actor, id uint) error
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) List(ctx context.Context) ([]Response, error) {
	bills, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Response, len(bills))
	for i, b := range bills {
		out[i] = toResponse(b)
	}
	return out, nil
}

func (s *service) GetByID(ctx context.Context, id uint) (Response, error) {
	b, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Response{}, err
	}
	return toResponse(b), nil
}

func (s *service) Create(ctx context.Context, actor auth.Actor, input CreateInput) (Response, error) {
	if input.Supplier == "" || input.InvoiceNo == "" || input.InvoiceDate == "" {
		return Response{}, apperr.BadRequest("supplier, invoiceNo, and invoiceDate are required")
	}
	if input.Amount < 0 {
		return Response{}, apperr.BadRequest("amount must be >= 0")
	}
	id, err := s.repo.Create(ctx, input, actor.UserID)
	if err != nil {
		return Response{}, err
	}
	return s.GetByID(ctx, id)
}

func (s *service) Update(ctx context.Context, actor auth.Actor, id uint, input UpdateInput) (Response, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return Response{}, err
	}
	if input.Supplier == "" || input.InvoiceNo == "" || input.InvoiceDate == "" {
		return Response{}, apperr.BadRequest("supplier, invoiceNo, and invoiceDate are required")
	}
	if input.Amount < 0 {
		return Response{}, apperr.BadRequest("amount must be >= 0")
	}
	if err := s.repo.Update(ctx, id, input); err != nil {
		return Response{}, err
	}
	return s.GetByID(ctx, id)
}

func (s *service) Delete(ctx context.Context, actor auth.Actor, id uint) error {
	return s.repo.SoftDelete(ctx, id)
}
