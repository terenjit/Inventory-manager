package datasheet_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/config"
	"inventory-manager/internal/datasheet"
	"inventory-manager/internal/db"
	"inventory-manager/internal/stock"
	"inventory-manager/internal/store"
)

var adminActor = auth.Actor{UserID: 1, Role: "admin"}

func buildXLSX(rows [][]interface{}) *bytes.Buffer {
	f := excelize.NewFile()
	sheet := "Sheet1"
	headers := []interface{}{"KodeItem", "NamaItem", "Jenis", "Satuan", "HargaPokok", "HargaJual", "Stok"}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for rowIdx, row := range rows {
		for col, v := range row {
			cell, _ := excelize.CoordinatesToCellName(col+1, rowIdx+2)
			f.SetCellValue(sheet, cell, v)
		}
	}
	buf, _ := f.WriteToBuffer()
	return buf
}

func TestImport_OneBadRowImportsNothing(t *testing.T) {
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	database, err := db.New(cfg.DBTestDSN)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}
	defer database.Close()

	stockWriter := stock.NewWriter(database)
	storeRepo := store.NewRepository(database)
	svc := datasheet.NewService(database, stockWriter, storeRepo)

	ctx := context.Background()

	// File with 2 valid rows and 1 bad row (invalid unit and negative price).
	buf := buildXLSX([][]interface{}{
		{"DS-GOOD1", "Good Product 1", "TestCat", "PCS", 5000, 8000, nil},
		{"DS-GOOD2", "Good Product 2", "TestCat", "METER", 3000, 5000, nil},
		{"DS-BAD1", "Bad Product", "TestCat", "INVALID_UNIT", -100, 8000, nil}, // bad row
	})

	result, rowErrs, err := svc.Import(ctx, adminActor, bytes.NewReader(buf.Bytes()), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must return row errors.
	if len(rowErrs) == 0 {
		t.Fatal("expected row errors, got none")
	}
	t.Logf("got %d row error(s): %+v", len(rowErrs), rowErrs)

	// Must NOT have imported anything.
	if result != nil {
		t.Fatalf("expected nil result (no import), got %+v", result)
	}

	// Verify DS-GOOD1 was NOT inserted.
	var count int
	database.GetContext(ctx, &count, "SELECT COUNT(*) FROM products WHERE code = 'DS-GOOD1'")
	if count > 0 {
		t.Error("DS-GOOD1 was inserted despite bad row in file — all-or-nothing violated")
	}
	t.Log("all-or-nothing import ✓")
}

func TestImport_AllValidRowsUpsert(t *testing.T) {
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	database, err := db.New(cfg.DBTestDSN)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}
	defer database.Close()

	stockWriter := stock.NewWriter(database)
	storeRepo := store.NewRepository(database)
	svc := datasheet.NewService(database, stockWriter, storeRepo)
	ctx := context.Background()

	buf := buildXLSX([][]interface{}{
		{"DS-V1", "Valid Product 1", "ValidCat", "PCS", 5000, 8000, nil},
		{"DS-V2", "Valid Product 2", "ValidCat", "METER", 3000, 5000, nil},
	})

	result, rowErrs, err := svc.Import(ctx, adminActor, bytes.NewReader(buf.Bytes()), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rowErrs) > 0 {
		t.Fatalf("unexpected row errors: %+v", rowErrs)
	}
	if result.Upserted != 2 {
		t.Errorf("expected 2 upserted, got %d", result.Upserted)
	}

	// Run again — should upsert (update) existing rows.
	buf2 := buildXLSX([][]interface{}{
		{"DS-V1", "Valid Product 1 Updated", "ValidCat", "PCS", 5500, 9000, nil},
	})
	result2, _, _ := svc.Import(ctx, adminActor, bytes.NewReader(buf2.Bytes()), 0)
	if result2.Upserted != 1 {
		t.Errorf("expected 1 upserted on re-import, got %d", result2.Upserted)
	}

	// Verify name was updated.
	var name string
	database.GetContext(ctx, &name, "SELECT name FROM products WHERE code = 'DS-V1'")
	if name != "Valid Product 1 Updated" {
		t.Errorf("expected updated name, got %q", name)
	}
	t.Log("upsert import ✓")
}

func TestImport_StockColumn(t *testing.T) {
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	database, err := db.New(cfg.DBTestDSN)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}
	defer database.Close()

	stockWriter := stock.NewWriter(database)
	storeRepo := store.NewRepository(database)
	svc := datasheet.NewService(database, stockWriter, storeRepo)
	ctx := context.Background()

	// Get a valid store ID.
	stores, _ := storeRepo.List(ctx)
	if len(stores) == 0 {
		t.Skip("no stores in test DB")
	}
	testStoreID := stores[0].ID

	buf := buildXLSX([][]interface{}{
		{"DS-STK1", "Stock Import Product", "", "PCS", 4000, 7000, 25},
	})

	_, rowErrs, err := svc.Import(ctx, adminActor, bytes.NewReader(buf.Bytes()), testStoreID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rowErrs) > 0 {
		t.Fatalf("unexpected row errors: %+v", rowErrs)
	}

	// Verify stock was set to 25.
	var prodID uint
	database.GetContext(ctx, &prodID, "SELECT id FROM products WHERE code = 'DS-STK1'")

	var qty decimal.Decimal
	database.GetContext(ctx, &qty,
		"SELECT COALESCE(qty, 0) FROM inventory WHERE store_id=? AND product_id=?",
		testStoreID, prodID,
	)
	if !qty.Equal(decimal.NewFromInt(25)) {
		t.Errorf("expected stock=25, got %s", qty)
	}
	t.Log("stock import ✓")
}
