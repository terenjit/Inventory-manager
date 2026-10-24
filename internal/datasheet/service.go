package datasheet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
	"inventory-manager/internal/apperr"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/stock"
	"inventory-manager/internal/store"
)

type Service interface {
	Export(ctx context.Context, actor auth.Actor, storeID uint) ([]byte, error)
	Import(ctx context.Context, actor auth.Actor, r io.Reader, storeID uint) (*ImportResult, []RowError, error)
}

type service struct {
	db          *sqlx.DB
	stockWriter *stock.Writer
	storeRepo   store.Repository
}

func NewService(db *sqlx.DB, stockWriter *stock.Writer, storeRepo store.Repository) Service {
	return &service{db: db, stockWriter: stockWriter, storeRepo: storeRepo}
}

// ─────────────────────────── Export ───────────────────────────

func (s *service) Export(ctx context.Context, actor auth.Actor, storeID uint) ([]byte, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}

	type row struct {
		Code         string          `db:"code"`
		Name         string          `db:"name"`
		CategoryName *string         `db:"category_name"`
		Unit         string          `db:"unit"`
		CostPrice    decimal.Decimal `db:"cost_price"`
		SellPrice    decimal.Decimal `db:"sell_price"`
		StockQty     decimal.Decimal `db:"stock_qty"`
	}

	stockJoin := `LEFT JOIN inventory i ON i.product_id = p.id`
	stockCol := `COALESCE(SUM(i.qty), 0) AS stock_qty`
	groupBy := `GROUP BY p.id`

	if storeID > 0 {
		stockJoin = fmt.Sprintf(
			"LEFT JOIN inventory i ON i.product_id = p.id AND i.store_id = %d", storeID,
		)
		stockCol = `COALESCE(i.qty, 0) AS stock_qty`
		groupBy = ""
	}

	query := fmt.Sprintf(`
		SELECT p.code, p.name, c.name AS category_name, p.unit,
		       p.cost_price, p.sell_price, %s
		  FROM products p
		  LEFT JOIN categories c ON c.id = p.category_id
		  %s
		 WHERE p.is_active = TRUE
		 %s
		 ORDER BY p.code`, stockCol, stockJoin, groupBy)

	var rows []row
	if err := s.db.SelectContext(ctx, &rows, query); err != nil {
		return nil, apperr.Internal(err)
	}

	f := excelize.NewFile()
	sheet := "Sheet1"

	// Header row.
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// Data rows.
	for rowIdx, r := range rows {
		excelRow := rowIdx + 2
		category := ""
		if r.CategoryName != nil {
			category = *r.CategoryName
		}
		values := []interface{}{
			r.Code,
			r.Name,
			category,
			strings.ToUpper(r.Unit),
			r.CostPrice.InexactFloat64(),
			r.SellPrice.InexactFloat64(),
			r.StockQty.InexactFloat64(),
		}
		for col, v := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, excelRow)
			f.SetCellValue(sheet, cell, v)
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return buf.Bytes(), nil
}

// ─────────────────────────── Import ───────────────────────────

type parsedRow struct {
	code      string
	name      string
	category  string
	unit      string
	costPrice decimal.Decimal
	sellPrice decimal.Decimal
	stock     *decimal.Decimal // nil = not provided
}

func (s *service) Import(ctx context.Context, actor auth.Actor, r io.Reader, storeID uint) (*ImportResult, []RowError, error) {
	if !actor.IsAdmin() {
		return nil, nil, apperr.Forbidden("admin only")
	}

	// Validate store if stock import is requested.
	if storeID > 0 {
		if _, err := s.storeRepo.GetByID(ctx, storeID); err != nil {
			return nil, nil, apperr.BadRequest("store not found")
		}
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, apperr.BadRequest("failed to read file")
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, nil, apperr.BadRequest("invalid Excel file")
	}

	sheet := f.GetSheetName(0)
	allRows, err := f.GetRows(sheet)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}
	if len(allRows) < 2 {
		return nil, nil, apperr.BadRequest("file has no data rows")
	}

	// ── Pass 1: parse and validate all rows ──────────────────────
	var errs []RowError
	parsed := make([]parsedRow, 0, len(allRows)-1)

	for i, cols := range allRows[1:] { // skip header
		excelRow := i + 2 // 1-based, +1 for header
		get := func(colIdx int) string {
			if colIdx-1 < len(cols) {
				return strings.TrimSpace(cols[colIdx-1])
			}
			return ""
		}

		code := strings.ToUpper(get(colCode))
		name := get(colName)
		category := get(colCategory)
		unitRaw := strings.ToUpper(get(colUnit))
		costStr := get(colCostPrice)
		sellStr := get(colSellPrice)
		stockStr := get(colStock)

		if code == "" {
			errs = append(errs, RowError{Row: excelRow, Column: "KodeItem", Message: "required"})
		}
		if name == "" {
			errs = append(errs, RowError{Row: excelRow, Column: "NamaItem", Message: "required"})
		}

		unit := ""
		switch unitRaw {
		case "PCS":
			unit = "pcs"
		case "METER", "MTR", "M":
			unit = "meter"
		default:
			if unitRaw != "" {
				errs = append(errs, RowError{Row: excelRow, Column: "Satuan", Message: "must be PCS or METER"})
			} else {
				errs = append(errs, RowError{Row: excelRow, Column: "Satuan", Message: "required"})
			}
		}

		costPrice, cpErr := decimal.NewFromString(costStr)
		if cpErr != nil || costPrice.IsNegative() {
			errs = append(errs, RowError{Row: excelRow, Column: "HargaPokok", Message: "invalid or negative number"})
		}
		sellPrice, spErr := decimal.NewFromString(sellStr)
		if spErr != nil || sellPrice.IsNegative() {
			errs = append(errs, RowError{Row: excelRow, Column: "HargaJual", Message: "invalid or negative number"})
		}

		var stockQty *decimal.Decimal
		if stockStr != "" {
			qty, qErr := decimal.NewFromString(stockStr)
			if qErr != nil || qty.IsNegative() {
				errs = append(errs, RowError{Row: excelRow, Column: "Stok", Message: "invalid or negative number"})
			} else {
				stockQty = &qty
			}
		}

		// Only append if row is parseable (no critical field errors).
		if code != "" && name != "" && unit != "" && cpErr == nil && spErr == nil {
			parsed = append(parsed, parsedRow{
				code: code, name: name, category: category,
				unit: unit, costPrice: costPrice, sellPrice: sellPrice,
				stock: stockQty,
			})
		}
	}

	// ── If any validation error → return all errors, import nothing ──
	if len(errs) > 0 {
		return nil, errs, nil
	}

	// ── Pass 2: execute all upserts in one transaction ────────────
	var upserted int
	err = s.stockWriter.WithRetry(ctx, func(tx *sqlx.Tx) error {
		// Pre-resolve categories (find or create).
		categoryIDs := make(map[string]*uint)
		for _, row := range parsed {
			if row.category == "" {
				continue
			}
			if _, ok := categoryIDs[row.category]; ok {
				continue
			}
			res, err := tx.ExecContext(ctx,
				"INSERT INTO categories (name) VALUES (?) ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)",
				row.category,
			)
			if err != nil {
				return apperr.Internal(err)
			}
			id, _ := res.LastInsertId()
			uid := uint(id)
			categoryIDs[row.category] = &uid
		}

		// Upsert products, collect IDs for stock update.
		type productResult struct {
			id    uint
			stock *decimal.Decimal
		}
		results := make([]productResult, 0, len(parsed))

		for _, row := range parsed {
			var catID interface{}
			if row.category != "" {
				catID = categoryIDs[row.category]
			}
			res, err := tx.ExecContext(ctx,
				`INSERT INTO products (code, name, category_id, unit, cost_price, sell_price)
				 VALUES (?, ?, ?, ?, ?, ?)
				 ON DUPLICATE KEY UPDATE
				   id          = LAST_INSERT_ID(id),
				   name        = VALUES(name),
				   category_id = VALUES(category_id),
				   unit        = VALUES(unit),
				   cost_price  = VALUES(cost_price),
				   sell_price  = VALUES(sell_price),
				   updated_at  = NOW(3)`,
				row.code, row.name, catID, row.unit, row.costPrice, row.sellPrice,
			)
			if err != nil {
				return apperr.Internal(fmt.Errorf("upsert %s: %w", row.code, err))
			}
			id, _ := res.LastInsertId()
			results = append(results, productResult{id: uint(id), stock: row.stock})
			upserted++
		}

		// Apply stock adjustments if store_id provided.
		if storeID > 0 {
			for _, result := range results {
				if result.stock == nil {
					continue
				}
				var current decimal.Decimal
				s.db.GetContext(ctx, &current,
					"SELECT COALESCE((SELECT qty FROM inventory WHERE store_id=? AND product_id=?), 0)",
					storeID, result.id,
				)
				delta := result.stock.Sub(current)
				if delta.IsZero() {
					continue
				}
				if err := s.stockWriter.Adjust(
					ctx, tx, storeID, result.id, delta,
					0, "import", actor.UserID,
				); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return &ImportResult{Upserted: upserted}, nil, nil
}
