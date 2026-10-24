package salesperson

import (
	"context"

	"inventory-manager/internal/apperr"
	"inventory-manager/internal/auth"
)

type Service interface {
	List(ctx context.Context) ([]*Salesperson, error)
	GetByID(ctx context.Context, id uint) (*Salesperson, error)
	Create(ctx context.Context, actor auth.Actor, input CreateInput) (*Salesperson, error)
	Update(ctx context.Context, actor auth.Actor, id uint, input UpdateInput) error
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) List(ctx context.Context) ([]*Salesperson, error) {
	return s.repo.List(ctx)
}

func (s *service) GetByID(ctx context.Context, id uint) (*Salesperson, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) Create(ctx context.Context, actor auth.Actor, input CreateInput) (*Salesperson, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	id, err := s.repo.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *service) Update(ctx context.Context, actor auth.Actor, id uint, input UpdateInput) error {
	if !actor.IsAdmin() {
		return apperr.Forbidden("admin only")
	}
	return s.repo.Update(ctx, id, input)
}
