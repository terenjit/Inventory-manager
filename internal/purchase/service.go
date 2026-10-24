package purchase

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"inventory-manager/internal/apperr"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/product"
	"inventory-manager/internal/store"
	"inventory-manager/internal/supplier"
)

type Service interface {
	CreateInvoice(ctx context.Context, actor auth.Actor, input CreateInput) (*Response, error)
	GetInvoice(ctx context.Context, id uint) (*Response, error)
	ListInvoices(ctx context.Context, f ListFilter) (*ListResult, error)
	MarkReceived(ctx context.Context, actor auth.Actor, id uint, date *string) (*Response, error)
	UpdatePayment(ctx context.Context, actor auth.Actor, id uint, input UpdatePaymentInput) (*Response, error)
}

type service struct {
	repo         Repository
	productRepo  product.Repository
	storeRepo    store.Repository
	supplierRepo supplier.Repository
}

func NewService(
	repo Repository,
	productRepo product.Repository,
	storeRepo store.Repository,
	supplierRepo supplier.Repository,
) Service {
	return &service{repo: repo, productRepo: productRepo, storeRepo: storeRepo, supplierRepo: supplierRepo}
}

func (s *service) CreateInvoice(ctx context.Context, actor auth.Actor, input CreateInput) (*Response, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	if _, err := s.supplierRepo.GetByID(ctx, input.SupplierID); err != nil {
		return nil, apperr.BadRequest("supplier not found")
	}

	storeID, err := resolveStore(actor, input.StoreID)
	if err != nil {
		return nil, err
	}
	if _, err := s.storeRepo.GetByID(ctx, storeID); err != nil {
		return nil, apperr.BadRequest("store not found")
	}

	invoiceDate, err := time.Parse("2006-01-02", input.InvoiceDate)
	if err != nil {
		return nil, apperr.BadRequest("invalid invoice_date, use YYYY-MM-DD")
	}
	var dueDate *time.Time
	if input.DueDate != nil {
		d, err := time.Parse("2006-01-02", *input.DueDate)
		if err != nil {
			return nil, apperr.BadRequest("invalid due_date, use YYYY-MM-DD")
		}
		dueDate = &d
	}
	var goodsReceivedDate *time.Time
	if input.GoodsReceivedDate != nil {
		d, err := time.Parse("2006-01-02", *input.GoodsReceivedDate)
		if err != nil {
			return nil, apperr.BadRequest("invalid goods_received_date, use YYYY-MM-DD")
		}
		goodsReceivedDate = &d
	}

	items := make([]*InvoiceItem, 0, len(input.Items))
	total := decimal.Zero
	for _, item := range input.Items {
		if !item.Qty.IsPositive() || !item.UnitCost.IsPositive() {
			return nil, apperr.BadRequest("item qty and unit_cost must be greater than zero")
		}
		p, err := s.productRepo.GetByID(ctx, item.ProductID)
		if err != nil {
			return nil, apperr.BadRequest("product not found")
		}
		if !p.IsActive {
			return nil, apperr.BadRequest("product is inactive: " + p.Code)
		}
		total = total.Add(item.UnitCost.Mul(item.Qty))
		items = append(items, &InvoiceItem{ProductID: item.ProductID, Qty: item.Qty, UnitCost: item.UnitCost})
	}

	inv := &Invoice{
		SupplierID:        input.SupplierID,
		StoreID:           storeID,
		InvoiceNo:         input.InvoiceNo,
		InvoiceDate:       invoiceDate,
		GoodsReceivedDate: goodsReceivedDate,
		DueDate:           dueDate,
		Amount:            total,
		Note:              input.Note,
		CreatedBy:         actor.UserID,
	}
	id, err := s.repo.Create(ctx, inv, items)
	if err != nil {
		return nil, err
	}
	return s.GetInvoice(ctx, id)
}

func (s *service) GetInvoice(ctx context.Context, id uint) (*Response, error) {
	inv, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	dbItems, err := s.repo.GetItems(ctx, id)
	if err != nil {
		return nil, err
	}
	items := make([]ItemResponse, len(dbItems))
	for i, it := range dbItems {
		items[i] = ItemResponse{
			ProductID: it.ProductID, ProductCode: it.ProductCode,
			ProductName: it.ProductName, Unit: it.Unit, Qty: it.Qty, UnitCost: it.UnitCost,
		}
	}
	return inv.ToResponse(items), nil
}

func (s *service) ListInvoices(ctx context.Context, f ListFilter) (*ListResult, error) {
	invoices, total, totalUnpaid, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, err
	}
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	data := make([]*Response, len(invoices))
	for i, inv := range invoices {
		data[i] = inv.ToResponse(nil)
	}
	return &ListResult{Data: data, Total: total, TotalUnpaid: totalUnpaid, Page: page, Limit: limit}, nil
}

// MarkReceived records when goods physically arrived — informational only, no stock movement.
func (s *service) MarkReceived(ctx context.Context, actor auth.Actor, id uint, dateStr *string) (*Response, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	var d time.Time
	if dateStr != nil {
		parsed, err := time.Parse("2006-01-02", *dateStr)
		if err != nil {
			return nil, apperr.BadRequest("invalid date, use YYYY-MM-DD")
		}
		d = parsed
	} else {
		d = time.Now().UTC().Truncate(24 * time.Hour)
	}
	if err := s.repo.SetGoodsReceivedDate(ctx, id, d); err != nil {
		return nil, err
	}
	return s.GetInvoice(ctx, id)
}

func (s *service) UpdatePayment(ctx context.Context, actor auth.Actor, id uint, input UpdatePaymentInput) (*Response, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	inv, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.PaidAmount.LessThan(decimal.Zero) {
		return nil, apperr.BadRequest("paid_amount cannot be negative")
	}
	if input.PaidAmount.GreaterThan(inv.Amount) {
		return nil, apperr.BadRequest("paid_amount cannot exceed invoice amount")
	}
	if err := s.repo.UpdatePayment(ctx, id, input.PaidAmount, inv.Amount); err != nil {
		return nil, err
	}
	return s.GetInvoice(ctx, id)
}

func resolveStore(actor auth.Actor, storeID *uint) (uint, error) {
	if actor.IsAdmin() {
		if storeID == nil || *storeID == 0 {
			return 0, apperr.BadRequest("store_id is required")
		}
		return *storeID, nil
	}
	if actor.StoreID == nil {
		return 0, apperr.Forbidden("cashier has no store assigned")
	}
	return *actor.StoreID, nil
}
