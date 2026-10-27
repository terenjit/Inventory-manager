package transfer

import (
	"context"
	"errors"
	"sort"

	"github.com/jmoiron/sqlx"
	"inventory-manager/internal/apperr"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/product"
	"inventory-manager/internal/stock"
	"inventory-manager/internal/store"
)

type Service interface {
	CreateTransfer(ctx context.Context, actor auth.Actor, input CreateInput) (*Response, error)
	GetTransfer(ctx context.Context, actor auth.Actor, id uint) (*Response, error)
	ListTransfers(ctx context.Context, actor auth.Actor, f ListFilter) (*ListResult, error)
	ReceiveTransfer(ctx context.Context, actor auth.Actor, id uint) (*Response, error)
	CancelTransfer(ctx context.Context, actor auth.Actor, id uint) error
}

type service struct {
	db          *sqlx.DB
	repo        Repository
	stockWriter *stock.Writer
	productRepo product.Repository
	storeRepo   store.Repository
}

func NewService(
	db *sqlx.DB,
	repo Repository,
	stockWriter *stock.Writer,
	productRepo product.Repository,
	storeRepo store.Repository,
) Service {
	return &service{db: db, repo: repo, stockWriter: stockWriter, productRepo: productRepo, storeRepo: storeRepo}
}

func (s *service) CreateTransfer(ctx context.Context, actor auth.Actor, input CreateInput) (*Response, error) {
	fromStoreID, err := resolveFromStore(actor, input.FromStoreID)
	if err != nil {
		return nil, err
	}
	if input.ToStoreID == fromStoreID {
		return nil, apperr.BadRequest("from and to store must be different")
	}
	if _, err := s.storeRepo.GetByID(ctx, input.ToStoreID); err != nil {
		return nil, apperr.BadRequest("to_store_id not found")
	}

	fromStore, err := s.storeRepo.GetByID(ctx, fromStoreID)
	if err != nil {
		return nil, err
	}

	// Validate items and snapshot product info.
	dbItems := make([]*TransferItem, 0, len(input.Items))
	for _, item := range input.Items {
		if !item.Qty.IsPositive() {
			return nil, apperr.BadRequest("item quantity must be greater than zero")
		}
		p, err := s.productRepo.GetByID(ctx, item.ProductID)
		if err != nil {
			return nil, apperr.BadRequest("product not found")
		}
		if !p.IsActive {
			return nil, apperr.BadRequest("product is inactive: " + p.Code)
		}
		dbItems = append(dbItems, &TransferItem{
			ProductID: item.ProductID,
			Qty:       item.Qty,
		})
	}

	// Sort by product_id to prevent deadlocks.
	sort.Slice(dbItems, func(i, j int) bool {
		return dbItems[i].ProductID < dbItems[j].ProductID
	})

	var transferID uint
	err = s.stockWriter.WithRetry(ctx, func(tx *sqlx.Tx) error {
		docNo, err := s.repo.NextDocNo(ctx, tx, fromStoreID, fromStore.Code)
		if err != nil {
			return err
		}

		t := &Transfer{
			TransferNo:  docNo,
			FromStoreID: fromStoreID,
			ToStoreID:   input.ToStoreID,
			Note:        input.Note,
			SentBy:      actor.UserID,
		}

		id, err := s.repo.Create(ctx, tx, t, dbItems)
		if err != nil {
			return err
		}
		transferID = id

		// Decrement source stock for each item.
		for _, item := range dbItems {
			if err := s.stockWriter.Decrement(
				ctx, tx, fromStoreID, item.ProductID, item.Qty,
				stock.ReasonTransferOut, stock.RefTransfer, transferID,
				"", actor.UserID,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetTransfer(ctx, actor, transferID)
}

func (s *service) GetTransfer(ctx context.Context, actor auth.Actor, id uint) (*Response, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := assertAccess(actor, t); err != nil {
		return nil, err
	}
	dbItems, err := s.repo.GetItems(ctx, id)
	if err != nil {
		return nil, err
	}
	items := make([]ItemResponse, len(dbItems))
	for i, it := range dbItems {
		items[i] = ItemResponse{
			ProductID:   it.ProductID,
			ProductCode: it.ProductCode,
			ProductName: it.ProductName,
			Unit:        it.Unit,
			Qty:         it.Qty,
		}
	}
	return t.ToResponse(items), nil
}

func (s *service) ListTransfers(ctx context.Context, actor auth.Actor, f ListFilter) (*ListResult, error) {
	// Cashier always scoped to their own store.
	if actor.IsCashier() && actor.StoreID != nil {
		f.StoreID = *actor.StoreID
	}

	transfers, total, err := s.repo.List(ctx, f)
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

	ids := make([]uint, len(transfers))
	for i, t := range transfers {
		ids[i] = t.ID
	}
	itemsByTransfer, err := s.repo.GetItemsByTransferIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	data := make([]*Response, len(transfers))
	for i, t := range transfers {
		dbItems := itemsByTransfer[t.ID]
		items := make([]ItemResponse, len(dbItems))
		for j, it := range dbItems {
			items[j] = ItemResponse{
				ProductID:   it.ProductID,
				ProductCode: it.ProductCode,
				ProductName: it.ProductName,
				Unit:        it.Unit,
				Qty:         it.Qty,
			}
		}
		data[i] = t.ToResponse(items)
	}
	return &ListResult{Data: data, Total: total, Page: page, Limit: limit}, nil
}

func (s *service) ReceiveTransfer(ctx context.Context, actor auth.Actor, id uint) (*Response, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Cashier can only receive into their own store.
	if actor.IsCashier() && actor.StoreID != nil && t.ToStoreID != *actor.StoreID {
		return nil, apperr.Forbidden("you can only receive transfers into your own store")
	}

	items, err := s.repo.GetItems(ctx, id)
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ProductID < items[j].ProductID
	})

	err = s.stockWriter.WithRetry(ctx, func(tx *sqlx.Tx) error {
		// Atomic status update — returns 409 if already received/cancelled.
		if err := s.repo.UpdateStatus(ctx, tx, id, "received", actor.UserID); err != nil {
			return err
		}
		for _, item := range items {
			if err := s.stockWriter.Increment(
				ctx, tx, t.ToStoreID, item.ProductID, item.Qty,
				stock.ReasonTransferIn, stock.RefTransfer, id,
				"", actor.UserID,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetTransfer(ctx, actor, id)
}

func (s *service) CancelTransfer(ctx context.Context, actor auth.Actor, id uint) error {
	if !actor.IsAdmin() {
		return apperr.Forbidden("only admin can cancel transfers")
	}
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	items, err := s.repo.GetItems(ctx, id)
	if err != nil {
		return err
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ProductID < items[j].ProductID
	})

	return s.stockWriter.WithRetry(ctx, func(tx *sqlx.Tx) error {
		if err := s.repo.UpdateStatus(ctx, tx, id, "cancelled", actor.UserID); err != nil {
			return err
		}
		for _, item := range items {
			if err := s.stockWriter.Increment(
				ctx, tx, t.FromStoreID, item.ProductID, item.Qty,
				stock.ReasonTransferCancel, stock.RefTransfer, id,
				"", actor.UserID,
			); err != nil {
				return err
			}
		}
		return nil
	})
}

func resolveFromStore(actor auth.Actor, requestedFromStoreID *uint) (uint, error) {
	if actor.IsAdmin() {
		if requestedFromStoreID == nil || *requestedFromStoreID == 0 {
			return 0, apperr.BadRequest("from_store_id is required")
		}
		return *requestedFromStoreID, nil
	}
	if actor.StoreID == nil {
		return 0, apperr.Internal(errors.New("cashier has no store assigned"))
	}
	return *actor.StoreID, nil
}

func assertAccess(actor auth.Actor, t *Transfer) error {
	if actor.IsAdmin() {
		return nil
	}
	if actor.StoreID == nil {
		return apperr.Forbidden("access denied")
	}
	if t.FromStoreID != *actor.StoreID && t.ToStoreID != *actor.StoreID {
		return apperr.Forbidden("access denied")
	}
	return nil
}
