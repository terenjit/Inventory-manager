package category

import (
	"context"

	"inventory-manager/internal/apperr"
	"inventory-manager/internal/auth"
)

type Service interface {
	List(ctx context.Context) ([]*Category, error)
	GetByID(ctx context.Context, id uint) (*Category, error)
	Create(ctx context.Context, actor auth.Actor, input CreateInput) (*Category, error)
	Update(ctx context.Context, actor auth.Actor, id uint, input UpdateInput) error
	Delete(ctx context.Context, actor auth.Actor, id uint) error
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) List(ctx context.Context) ([]*Category, error) {
	return s.repo.List(ctx)
}

func (s *service) GetByID(ctx context.Context, id uint) (*Category, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) Create(ctx context.Context, actor auth.Actor, input CreateInput) (*Category, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	id, err := s.repo.Create(ctx, input.Name)
	if err != nil {
		return nil, err
	}
	return &Category{ID: id, Name: input.Name}, nil
}

func (s *service) Update(ctx context.Context, actor auth.Actor, id uint, input UpdateInput) error {
	if !actor.IsAdmin() {
		return apperr.Forbidden("admin only")
	}
	return s.repo.Update(ctx, id, input.Name)
}

func (s *service) Delete(ctx context.Context, actor auth.Actor, id uint) error {
	if !actor.IsAdmin() {
		return apperr.Forbidden("admin only")
	}
	return s.repo.Delete(ctx, id)
}
