package order

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"inventory-manager/internal/apperr"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/product"
	"inventory-manager/internal/stock"
	"inventory-manager/internal/store"
)

type Service interface {
	CreateOrder(ctx context.Context, actor auth.Actor, input CreateOrderInput) (*Response, error)
	GetOrder(ctx context.Context, actor auth.Actor, id uint) (*Response, error)
	GetByOrderNo(ctx context.Context, actor auth.Actor, orderNo string) (*Response, error)
	ListOrders(ctx context.Context, actor auth.Actor, f ListFilter) (*ListResult, error)
	AddPayment(ctx context.Context, actor auth.Actor, orderID uint, input AddPaymentInput) (*Response, error)
	VoidOrder(ctx context.Context, actor auth.Actor, orderID uint, input VoidInput) error
	UpdateMeta(ctx context.Context, actor auth.Actor, orderID uint, input UpdateMetaInput) (*Response, error)
	ContinueOrder(ctx context.Context, actor auth.Actor, orderID uint, input ContinueInput) (*Response, error)
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
	return &service{
		db:          db,
		repo:        repo,
		stockWriter: stockWriter,
		productRepo: productRepo,
		storeRepo:   storeRepo,
	}
}

func (s *service) CreateOrder(ctx context.Context, actor auth.Actor, input CreateOrderInput) (*Response, error) {
	// Validate discount: only one of pct or rp may be set.
	if input.DiscountPct.IsPositive() && input.DiscountRp.IsPositive() {
		return nil, apperr.BadRequest("use either discount_pct or discount_rp, not both")
	}

	// Resolve store.
	storeID, err := resolveStore(actor, input.StoreID)
	if err != nil {
		return nil, err
	}

	st, err := s.storeRepo.GetByID(ctx, storeID)
	if err != nil {
		return nil, err
	}

	// Merge duplicate product lines and validate quantities.
	merged := make(map[uint]decimal.Decimal)
	for _, item := range input.Items {
		if !item.Qty.IsPositive() {
			return nil, apperr.BadRequest("item quantity must be greater than zero")
		}
		merged[item.ProductID] = merged[item.ProductID].Add(item.Qty)
	}

	// Snapshot product prices outside the transaction (minimises lock time).
	type productSnapshot struct {
		id        uint
		code      string
		name      string
		unit      string
		sellPrice decimal.Decimal
		costPrice decimal.Decimal
		qty       decimal.Decimal
	}
	snapshots := make([]productSnapshot, 0, len(merged))
	for productID, qty := range merged {
		p, err := s.productRepo.GetByID(ctx, productID)
		if err != nil {
			return nil, apperr.BadRequest("product not found: " + err.Error())
		}
		if !p.IsActive {
			return nil, apperr.BadRequest("product is inactive: " + p.Code)
		}
		snapshots = append(snapshots, productSnapshot{
			id: p.ID, code: p.Code, name: p.Name, unit: p.Unit,
			sellPrice: p.SellPrice, costPrice: p.CostPrice, qty: qty,
		})
	}

	// Sort by product_id to prevent deadlocks on SELECT FOR UPDATE.
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].id < snapshots[j].id
	})

	// Calculate totals.
	subtotal := decimal.Zero
	for _, snap := range snapshots {
		subtotal = subtotal.Add(snap.sellPrice.Mul(snap.qty))
	}
	discountAmount := calcDiscount(subtotal, input.DiscountPct, input.DiscountRp)
	total := subtotal.Sub(discountAmount)

	var orderID uint
	err = s.stockWriter.WithRetry(ctx, func(tx *sqlx.Tx) error {
		// Generate document number.
		docNo, err := s.repo.NextDocNo(ctx, tx, storeID, st.Code)
		if err != nil {
			return err
		}

		// Build order and items.
		o := &Order{
			OrderNo:        docNo,
			StoreID:        storeID,
			CashierID:      actor.UserID,
			SalesID:        input.SalesID,
			CustomerName:   input.CustomerName,
			Note:           input.Note,
			Subtotal:       subtotal,
			Tax:            decimal.Zero,
			DiscountPct:    input.DiscountPct,
			DiscountRp:     input.DiscountRp,
			DiscountAmount: discountAmount,
			Total:          total,
		}

		items := make([]*OrderItem, len(snapshots))
		for i, snap := range snapshots {
			items[i] = &OrderItem{
				ProductID: snap.id,
				Qty:       snap.qty,
				UnitPrice: snap.sellPrice,
				UnitCost:  snap.costPrice,
				LineTotal: snap.sellPrice.Mul(snap.qty),
			}
		}

		// Insert order + items first to get order ID.
		id, err := s.repo.Create(ctx, tx, o, items)
		if err != nil {
			return err
		}
		orderID = id

		// Decrement stock for each item (already sorted by product_id).
		for _, snap := range snapshots {
			if err := s.stockWriter.Decrement(
				ctx, tx, storeID, snap.id, snap.qty,
				stock.ReasonSale, stock.RefOrder, orderID,
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

	return s.GetOrder(ctx, actor, orderID)
}

func (s *service) GetOrder(ctx context.Context, actor auth.Actor, id uint) (*Response, error) {
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Cashier can only see orders from their own store.
	if actor.IsCashier() && actor.StoreID != nil && o.StoreID != *actor.StoreID {
		return nil, apperr.Forbidden("access denied")
	}

	dbItems, err := s.repo.GetItems(ctx, id)
	if err != nil {
		return nil, err
	}

	items := make([]ItemResponse, len(dbItems))
	for i, it := range dbItems {
		items[i] = toItemResponse(it)
	}
	return o.ToResponse(items), nil
}

func (s *service) ListOrders(ctx context.Context, actor auth.Actor, f ListFilter) (*ListResult, error) {
	// Cashier always scoped to their store.
	if actor.IsCashier() && actor.StoreID != nil {
		f.StoreID = *actor.StoreID
	}

	orders, total, err := s.repo.List(ctx, f)
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

	// Batch-load items for all orders in one query.
	orderIDs := make([]uint, len(orders))
	for i, o := range orders {
		orderIDs[i] = o.ID
	}
	allItems, err := s.repo.GetItemsByOrderIDs(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	itemsByOrder := make(map[uint][]ItemResponse, len(orders))
	for _, it := range allItems {
		itemsByOrder[it.OrderID] = append(itemsByOrder[it.OrderID], toItemResponse(it))
	}

	data := make([]*Response, len(orders))
	for i, o := range orders {
		data[i] = o.ToResponse(itemsByOrder[o.ID])
	}
	return &ListResult{Data: data, Total: total, Page: page, Limit: limit}, nil
}

func (s *service) AddPayment(ctx context.Context, actor auth.Actor, orderID uint, input AddPaymentInput) (*Response, error) {
	if !input.Amount.IsPositive() {
		return nil, apperr.BadRequest("payment amount must be greater than zero")
	}

	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if actor.IsCashier() && actor.StoreID != nil && o.StoreID != *actor.StoreID {
		return nil, apperr.Forbidden("access denied")
	}
	if o.Status == "void" {
		return nil, apperr.Conflict("cannot add payment to a voided order")
	}
	if o.Status == "paid" {
		return nil, apperr.Conflict("order is already fully paid")
	}

	remaining := o.Total.Sub(o.PaidAmount)
	amount := input.Amount
	// For CASH: apply only up to the remaining balance (change is not stored).
	if amount.GreaterThan(remaining) {
		amount = remaining
	}
	newPaid := o.PaidAmount.Add(amount)

	err = s.stockWriter.WithRetry(ctx, func(tx *sqlx.Tx) error {
		p := &Payment{
			OrderID:    orderID,
			Method:     input.Method,
			Amount:     amount,
			Reference:  input.Reference,
			ReceivedBy: actor.UserID,
		}
		if err := s.repo.AddPayment(ctx, tx, p); err != nil {
			return err
		}
		return s.repo.UpdateOrderPayment(ctx, tx, orderID, newPaid, o.Total)
	})
	if err != nil {
		return nil, err
	}
	return s.GetOrder(ctx, actor, orderID)
}

func (s *service) VoidOrder(ctx context.Context, actor auth.Actor, orderID uint, input VoidInput) error {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if actor.IsCashier() && actor.StoreID != nil && o.StoreID != *actor.StoreID {
		return apperr.Forbidden("access denied")
	}
	if !actor.IsAdmin() {
		return apperr.Forbidden("only admin can void orders")
	}

	items, err := s.repo.GetItems(ctx, orderID)
	if err != nil {
		return err
	}

	// Sort items by product_id to prevent deadlocks.
	sort.Slice(items, func(i, j int) bool {
		return items[i].ProductID < items[j].ProductID
	})

	return s.stockWriter.WithRetry(ctx, func(tx *sqlx.Tx) error {
		// Mark void first — if already voided, this returns 409 and we never touch stock.
		if err := s.repo.VoidOrder(ctx, tx, orderID, input.Reason, actor.UserID); err != nil {
			return err
		}
		// Restore stock for each item.
		for _, item := range items {
			if err := s.stockWriter.Increment(
				ctx, tx, o.StoreID, item.ProductID, item.Qty,
				stock.ReasonSaleVoid, stock.RefOrder, orderID,
				"void: "+input.Reason, actor.UserID,
			); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *service) GetByOrderNo(ctx context.Context, actor auth.Actor, orderNo string) (*Response, error) {
	o, err := s.repo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if actor.IsCashier() && actor.StoreID != nil && o.StoreID != *actor.StoreID {
		return nil, apperr.Forbidden("access denied")
	}
	dbItems, err := s.repo.GetItems(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	items := make([]ItemResponse, len(dbItems))
	for i, it := range dbItems {
		items[i] = toItemResponse(it)
	}
	return o.ToResponse(items), nil
}

func (s *service) UpdateMeta(ctx context.Context, actor auth.Actor, orderID uint, input UpdateMetaInput) (*Response, error) {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if actor.IsCashier() && actor.StoreID != nil && o.StoreID != *actor.StoreID {
		return nil, apperr.Forbidden("access denied")
	}
	if o.Status == "void" {
		return nil, apperr.Conflict("cannot edit a voided order")
	}
	if err := s.repo.UpdateMeta(ctx, orderID, input); err != nil {
		return nil, err
	}
	return s.GetOrder(ctx, actor, orderID)
}

func (s *service) ContinueOrder(ctx context.Context, actor auth.Actor, orderID uint, input ContinueInput) (*Response, error) {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if actor.IsCashier() && actor.StoreID != nil && o.StoreID != *actor.StoreID {
		return nil, apperr.Forbidden("access denied")
	}
	if o.Status == "void" {
		return nil, apperr.Conflict("cannot modify a voided order")
	}
	if o.Status == "paid" && len(input.Items) > 0 {
		return nil, apperr.Conflict("cannot add items to a fully paid order")
	}

	// Snapshot prices for new items.
	type snap struct {
		id        uint
		qty       decimal.Decimal
		sellPrice decimal.Decimal
		costPrice decimal.Decimal
	}
	snaps := make([]snap, 0, len(input.Items))
	for _, it := range input.Items {
		if !it.Qty.IsPositive() {
			return nil, apperr.BadRequest("item quantity must be greater than zero")
		}
		p, err := s.productRepo.GetByID(ctx, it.ProductID)
		if err != nil {
			return nil, apperr.BadRequest("product not found")
		}
		if !p.IsActive {
			return nil, apperr.BadRequest("product is inactive: " + p.Code)
		}
		snaps = append(snaps, snap{id: p.ID, qty: it.Qty, sellPrice: p.SellPrice, costPrice: p.CostPrice})
	}

	// Sort by product_id to prevent deadlocks.
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].id < snaps[j].id })

	addedSubtotal := decimal.Zero
	for _, sn := range snaps {
		addedSubtotal = addedSubtotal.Add(sn.sellPrice.Mul(sn.qty))
	}
	newSubtotal := o.Subtotal.Add(addedSubtotal)
	newDiscountAmount := calcDiscount(newSubtotal, o.DiscountPct, o.DiscountRp)
	newTotal := newSubtotal.Sub(newDiscountAmount)

	err = s.stockWriter.WithRetry(ctx, func(tx *sqlx.Tx) error {
		// Update order meta if provided.
		metaInput := UpdateMetaInput{
			CustomerName: input.CustomerName,
			SalesID:      input.SalesID,
			Note:         input.Note,
		}
		if err := s.repo.UpdateMeta(ctx, orderID, metaInput); err != nil {
			return err
		}

		// Append new items and decrement stock.
		if len(snaps) > 0 {
			newItems := make([]*OrderItem, len(snaps))
			for i, sn := range snaps {
				newItems[i] = &OrderItem{
					ProductID: sn.id,
					Qty:       sn.qty,
					UnitPrice: sn.sellPrice,
					UnitCost:  sn.costPrice,
					LineTotal: sn.sellPrice.Mul(sn.qty),
				}
			}
			if err := s.repo.AppendItems(ctx, tx, orderID, newItems); err != nil {
				return err
			}
			for _, sn := range snaps {
				if err := s.stockWriter.Decrement(
					ctx, tx, o.StoreID, sn.id, sn.qty,
					stock.ReasonSale, stock.RefOrder, orderID,
					"", actor.UserID,
				); err != nil {
					return err
				}
			}
			if err := s.repo.UpdateTotals(ctx, tx, orderID, newSubtotal, newDiscountAmount, newTotal); err != nil {
				return err
			}
		}

		// Apply payment if provided.
		if input.Payment != nil && input.Payment.Amount.IsPositive() {
			remaining := newTotal.Sub(o.PaidAmount)
			amount := input.Payment.Amount
			if amount.GreaterThan(remaining) {
				amount = remaining
			}
			if amount.IsPositive() {
				p := &Payment{
					OrderID:    orderID,
					Method:     input.Payment.Method,
					Amount:     amount,
					Reference:  input.Payment.Reference,
					ReceivedBy: actor.UserID,
				}
				if err := s.repo.AddPayment(ctx, tx, p); err != nil {
					return err
				}
				return s.repo.UpdateOrderPayment(ctx, tx, orderID, o.PaidAmount.Add(amount), newTotal)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetOrder(ctx, actor, orderID)
}

// resolveStore returns the correct store ID for this request.
func resolveStore(actor auth.Actor, requestedStoreID *uint) (uint, error) {
	if actor.IsAdmin() {
		if requestedStoreID == nil || *requestedStoreID == 0 {
			return 0, apperr.BadRequest("store_id is required")
		}
		return *requestedStoreID, nil
	}
	if actor.StoreID == nil {
		return 0, apperr.Internal(errors.New("cashier has no store assigned"))
	}
	return *actor.StoreID, nil
}

func calcDiscount(subtotal, pct, rp decimal.Decimal) decimal.Decimal {
	if pct.IsPositive() {
		return subtotal.Mul(pct).Div(decimal.NewFromInt(100)).Floor()
	}
	if rp.IsPositive() {
		return rp
	}
	return decimal.Zero
}

// ParseWIBDate parses a date string (YYYY-MM-DD) in WIB and returns start/end UTC times.
func ParseWIBDate(dateStr string) (start, end time.Time, err error) {
	d, err := time.ParseInLocation("2006-01-02", dateStr, wib)
	if err != nil {
		return
	}
	start = d
	end = d.Add(24*time.Hour - time.Nanosecond)
	return
}
