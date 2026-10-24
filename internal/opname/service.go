package opname

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
	StartOpname(ctx context.Context, actor auth.Actor, input CreateInput) (*Response, error)
	GetOpname(ctx context.Context, actor auth.Actor, id uint) (*Response, error)
	ListOpnames(ctx context.Context, actor auth.Actor, f ListFilter) (*ListResult, error)
	AddLine(ctx context.Context, actor auth.Actor, opnameID uint, input AddLineInput) (*Response, error)
	UpdateLine(ctx context.Context, actor auth.Actor, opnameID uint, input UpdateLineInput, productID uint) error
	Finalize(ctx context.Context, actor auth.Actor, id uint) (*Response, error)
	Cancel(ctx context.Context, actor auth.Actor, id uint) error
	Summary(ctx context.Context, actor auth.Actor, id uint) (*SummaryResponse, error)
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

func (s *service) StartOpname(ctx context.Context, actor auth.Actor, input CreateInput) (*Response, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}

	storeID, err := resolveStore(actor, input.StoreID)
	if err != nil {
		return nil, err
	}
	st, err := s.storeRepo.GetByID(ctx, storeID)
	if err != nil {
		return nil, apperr.BadRequest("store not found")
	}

	var opnameDate time.Time
	if input.OpnameDate != nil {
		opnameDate, err = time.Parse("2006-01-02", *input.OpnameDate)
		if err != nil {
			return nil, apperr.BadRequest("invalid opname_date, use YYYY-MM-DD")
		}
	} else {
		opnameDate = time.Now().In(wib).Truncate(24 * time.Hour)
	}

	var opnameID uint
	err = s.stockWriter.WithRetry(ctx, func(tx *sqlx.Tx) error {
		docNo, err := s.repo.NextDocNo(ctx, tx, storeID, st.Code)
		if err != nil {
			return err
		}
		id, err := s.repo.Create(ctx, tx, &Opname{
			OpnameNo:   docNo,
			StoreID:    storeID,
			OpnameDate: opnameDate,
			Note:       input.Note,
			StartedBy:  actor.UserID,
		})
		if err != nil {
			return err
		}
		opnameID = id
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetOpname(ctx, actor, opnameID)
}

func (s *service) GetOpname(ctx context.Context, actor auth.Actor, id uint) (*Response, error) {
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := assertAccess(actor, o); err != nil {
		return nil, err
	}
	dbLines, err := s.repo.GetLines(ctx, id)
	if err != nil {
		return nil, err
	}
	lines := toLineResponses(dbLines)
	return o.ToResponse(lines), nil
}

func (s *service) ListOpnames(ctx context.Context, actor auth.Actor, f ListFilter) (*ListResult, error) {
	if actor.IsCashier() && actor.StoreID != nil {
		f.StoreID = *actor.StoreID
	}
	opnames, total, err := s.repo.List(ctx, f)
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
	data := make([]*Response, len(opnames))
	for i, o := range opnames {
		data[i] = o.ToResponse(nil)
	}
	return &ListResult{Data: data, Total: total, Page: page, Limit: limit}, nil
}

func (s *service) AddLine(ctx context.Context, actor auth.Actor, opnameID uint, input AddLineInput) (*Response, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	o, err := s.repo.GetByID(ctx, opnameID)
	if err != nil {
		return nil, err
	}
	if o.Status != "draft" {
		return nil, apperr.Conflict("opname is not in draft status")
	}
	p, err := s.productRepo.GetByID(ctx, input.ProductID)
	if err != nil {
		return nil, apperr.BadRequest("product not found")
	}

	// Snapshot current stock for this store.
	var systemQty decimal.Decimal
	err = s.db.GetContext(ctx, &systemQty,
		"SELECT COALESCE((SELECT qty FROM inventory WHERE store_id=? AND product_id=?), 0)",
		o.StoreID, p.ID,
	)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	if err := s.repo.AddLine(ctx, opnameID, input.ProductID, systemQty); err != nil {
		return nil, err
	}
	return s.GetOpname(ctx, actor, opnameID)
}

func (s *service) UpdateLine(ctx context.Context, actor auth.Actor, opnameID uint, input UpdateLineInput, productID uint) error {
	if !actor.IsAdmin() {
		return apperr.Forbidden("admin only")
	}
	o, err := s.repo.GetByID(ctx, opnameID)
	if err != nil {
		return err
	}
	if o.Status != "draft" {
		return apperr.Conflict("opname is not in draft status")
	}
	if input.CountedQty.IsNegative() {
		return apperr.BadRequest("counted_qty cannot be negative")
	}
	return s.repo.UpdateLine(ctx, opnameID, productID, input)
}

func (s *service) Finalize(ctx context.Context, actor auth.Actor, id uint) (*Response, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if o.Status != "draft" {
		return nil, apperr.Conflict("opname is not in draft status")
	}

	lines, err := s.repo.GetLines(ctx, id)
	if err != nil {
		return nil, err
	}

	// Only process lines that have been counted.
	counted := make([]*OpnameLine, 0, len(lines))
	for _, l := range lines {
		if l.CountedQty != nil {
			counted = append(counted, l)
		}
	}

	// Sort by product_id to prevent deadlocks.
	sort.Slice(counted, func(i, j int) bool {
		return counted[i].ProductID < counted[j].ProductID
	})

	err = s.stockWriter.WithRetry(ctx, func(tx *sqlx.Tx) error {
		if err := s.repo.Finalize(ctx, tx, id, actor.UserID); err != nil {
			return err
		}
		for _, line := range counted {
			delta := line.CountedQty.Sub(line.SystemQty)
			if err := s.stockWriter.Adjust(
				ctx, tx, o.StoreID, line.ProductID, delta,
				id, "", actor.UserID,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetOpname(ctx, actor, id)
}

func (s *service) Cancel(ctx context.Context, actor auth.Actor, id uint) error {
	if !actor.IsAdmin() {
		return apperr.Forbidden("admin only")
	}
	return s.repo.Cancel(ctx, id)
}

func (s *service) Summary(ctx context.Context, actor auth.Actor, id uint) (*SummaryResponse, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	lines, err := s.repo.GetLines(ctx, id)
	if err != nil {
		return nil, err
	}

	var total decimal.Decimal
	summaryLines := make([]SummaryLine, 0)

	for _, l := range lines {
		if l.CountedQty == nil {
			continue
		}
		selisih := l.CountedQty.Sub(l.SystemQty)
		if selisih.IsZero() {
			continue
		}
		nilaiSelisih := selisih.Mul(l.CostPrice)
		total = total.Add(nilaiSelisih)
		summaryLines = append(summaryLines, SummaryLine{
			ProductID:    l.ProductID,
			ProductCode:  l.ProductCode,
			ProductName:  l.ProductName,
			Unit:         l.Unit,
			SystemQty:    l.SystemQty,
			CountedQty:   *l.CountedQty,
			Selisih:      selisih,
			CostPrice:    l.CostPrice,
			NilaiSelisih: nilaiSelisih,
		})
	}

	return &SummaryResponse{
		OpnameID:          o.ID,
		OpnameNo:          o.OpnameNo,
		StoreName:         o.StoreName,
		OpnameDate:        o.OpnameDate,
		TotalNilaiSelisih: total,
		Lines:             summaryLines,
	}, nil
}

func resolveStore(actor auth.Actor, storeID *uint) (uint, error) {
	if actor.IsAdmin() {
		if storeID == nil || *storeID == 0 {
			return 0, apperr.BadRequest("store_id is required")
		}
		return *storeID, nil
	}
	if actor.StoreID == nil {
		return 0, apperr.Internal(errors.New("cashier has no store"))
	}
	return *actor.StoreID, nil
}

func assertAccess(actor auth.Actor, o *Opname) error {
	if actor.IsAdmin() {
		return nil
	}
	if actor.StoreID == nil || o.StoreID != *actor.StoreID {
		return apperr.Forbidden("access denied")
	}
	return nil
}

func toLineResponses(lines []*OpnameLine) []LineResponse {
	resp := make([]LineResponse, len(lines))
	for i, l := range lines {
		lr := LineResponse{
			ProductID:   l.ProductID,
			ProductCode: l.ProductCode,
			ProductName: l.ProductName,
			Unit:        l.Unit,
			SystemQty:   l.SystemQty,
			CountedQty:  l.CountedQty,
			Note:        l.Note,
		}
		if l.CountedQty != nil {
			s := l.CountedQty.Sub(l.SystemQty)
			lr.Selisih = &s
		}
		resp[i] = lr
	}
	return resp
}
