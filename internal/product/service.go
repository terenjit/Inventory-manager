package product

import (
	"context"

	"github.com/shopspring/decimal"
	"inventory-manager/internal/apperr"
	"inventory-manager/internal/auth"
)

type Service interface {
	List(ctx context.Context, actor auth.Actor, f ListFilter) (*ListResult, error)
	GetByID(ctx context.Context, actor auth.Actor, id uint) (interface{}, error)
	GetByCode(ctx context.Context, code string) (*Product, error)
	Create(ctx context.Context, actor auth.Actor, input CreateInput) (interface{}, error)
	Update(ctx context.Context, actor auth.Actor, id uint, input UpdateInput) error
	Deactivate(ctx context.Context, actor auth.Actor, id uint) error
	UpsertByCode(ctx context.Context, actor auth.Actor, input CreateInput) (interface{}, error)
	DeactivateByCode(ctx context.Context, actor auth.Actor, code string) error
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) List(ctx context.Context, actor auth.Actor, f ListFilter) (*ListResult, error) {
	products, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, err
	}

	ids := make([]uint, len(products))
	for i, p := range products {
		ids[i] = p.ID
	}
	stockMap, err := s.repo.GetStock(ctx, ids)
	if err != nil {
		return nil, err
	}

	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 20
	}
	page := f.Page
	if page < 1 {
		page = 1
	}

	if actor.IsAdmin() {
		data := make([]*AdminResponse, len(products))
		for i, p := range products {
			data[i] = toAdminResponse(p, stockMap[p.ID], f.StoreID)
		}
		return &ListResult{Data: data, Total: total, Page: page, Limit: limit}, nil
	}

	data := make([]*CashierResponse, len(products))
	for i, p := range products {
		data[i] = toCashierResponse(p, stockMap[p.ID], f.StoreID)
	}
	return &ListResult{Data: data, Total: total, Page: page, Limit: limit}, nil
}

func (s *service) GetByID(ctx context.Context, actor auth.Actor, id uint) (interface{}, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var storeID uint
	if actor.StoreID != nil {
		storeID = *actor.StoreID
	}
	stockMap, err := s.repo.GetStock(ctx, []uint{id})
	if err != nil {
		return nil, err
	}
	if actor.IsAdmin() {
		return toAdminResponse(p, stockMap[id], storeID), nil
	}
	return toCashierResponse(p, stockMap[id], storeID), nil
}

func (s *service) GetByCode(ctx context.Context, code string) (*Product, error) {
	return s.repo.GetByCode(ctx, code)
}

func (s *service) Create(ctx context.Context, actor auth.Actor, input CreateInput) (interface{}, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	id, err := s.repo.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, actor, id)
}

func (s *service) Update(ctx context.Context, actor auth.Actor, id uint, input UpdateInput) error {
	if !actor.IsAdmin() {
		return apperr.Forbidden("admin only")
	}
	return s.repo.Update(ctx, id, input)
}

func (s *service) Deactivate(ctx context.Context, actor auth.Actor, id uint) error {
	if !actor.IsAdmin() {
		return apperr.Forbidden("admin only")
	}
	f := false
	return s.repo.Update(ctx, id, UpdateInput{IsActive: &f})
}

func (s *service) UpsertByCode(ctx context.Context, actor auth.Actor, input CreateInput) (interface{}, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	existing, err := s.repo.GetByCode(ctx, input.Code)
	if err == nil {
		// Product exists — update it.
		active := true
		upd := UpdateInput{
			Name:       &input.Name,
			CategoryID: input.CategoryID,
			SupplierID: input.SupplierID,
			StoreID:    input.StoreID,
			IsActive:   &active,
		}
		if input.Unit != "" {
			upd.Unit = &input.Unit
		}
		if input.SellPrice.IsPositive() {
			upd.SellPrice = &input.SellPrice
		}
		if !input.CostPrice.IsNegative() {
			upd.CostPrice = &input.CostPrice
		}
		if input.MinStock.IsPositive() {
			upd.MinStock = &input.MinStock
		}
		if err := s.repo.Update(ctx, existing.ID, upd); err != nil {
			return nil, err
		}
		return s.GetByID(ctx, actor, existing.ID)
	}
	// Product not found — create it.
	return s.Create(ctx, actor, input)
}

func (s *service) DeactivateByCode(ctx context.Context, actor auth.Actor, code string) error {
	if !actor.IsAdmin() {
		return apperr.Forbidden("admin only")
	}
	p, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return err
	}
	f := false
	return s.repo.Update(ctx, p.ID, UpdateInput{IsActive: &f})
}

func toCashierResponse(p *Product, stock []StockInfo, primaryStoreID uint) *CashierResponse {
	if stock == nil {
		stock = []StockInfo{}
	}
	return &CashierResponse{
		ID:           p.ID,
		Code:         p.Code,
		Name:         p.Name,
		CategoryID:   p.CategoryID,
		CategoryName: p.CategoryName,
		StoreID:      p.StoreID,
		StoreName:    p.StoreName,
		Unit:         p.Unit,
		SellPrice:    p.SellPrice,
		MinStock:     p.MinStock,
		IsActive:     p.IsActive,
		Stock:        stock,
		IsLowStock:   isLowStock(stock, primaryStoreID, p.MinStock),
	}
}

func toAdminResponse(p *Product, stock []StockInfo, primaryStoreID uint) *AdminResponse {
	return &AdminResponse{
		CashierResponse: *toCashierResponse(p, stock, primaryStoreID),
		SupplierID:      p.SupplierID,
		SupplierName:    p.SupplierName,
		CostPrice:       p.CostPrice,
		Margin:          p.SellPrice.Sub(p.CostPrice),
	}
}

func isLowStock(stock []StockInfo, storeID uint, minStock decimal.Decimal) bool {
	for _, s := range stock {
		if s.StoreID == storeID {
			return s.Qty.LessThan(minStock)
		}
	}
	// no inventory row = qty 0
	return minStock.IsPositive()
}
