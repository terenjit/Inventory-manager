package report_test

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/config"
	"inventory-manager/internal/db"
	"inventory-manager/internal/order"
	"inventory-manager/internal/product"
	"inventory-manager/internal/report"
	"inventory-manager/internal/stock"
	"inventory-manager/internal/store"
)

var fixtureSeq int64

type fixture struct {
	ctx          context.Context
	db           *sqlx.DB
	storeID      uint
	adminActor   auth.Actor
	cashierActor auth.Actor
	productID    uint
	orderSvc     order.Service
	reportSvc    report.Service
}

func setupFixture(t *testing.T) *fixture {
	t.Helper()
	_ = godotenv.Load("../../.env")
	cfg := config.Load()

	database, err := db.New(cfg.DBTestDSN)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}

	ctx := context.Background()
	pid := os.Getpid()
	// seq makes each setupFixture call within a process unique without relying on test names.
	seq := atomic.AddInt64(&fixtureSeq, 1)
	tag := fmt.Sprintf("%d%d", pid%10000, seq) // max ~6 chars

	storeRepo := store.NewRepository(database)
	userRepo := auth.NewRepository(database)
	productRepo := product.NewRepository(database)
	stockWriter := stock.NewWriter(database)
	orderRepo := order.NewRepository(database)

	// Unique store per test run so report filters return only this test's data.
	storeCode := fmt.Sprintf("R%s", tag) // max 7 chars, fits VARCHAR(10)
	storeID, err := storeRepo.Create(ctx, &store.Store{Code: storeCode, Name: "Report Test Store"})
	if err != nil {
		stores, _ := storeRepo.List(ctx)
		for _, s := range stores {
			if s.Code == storeCode {
				storeID = s.ID
				break
			}
		}
	}
	if storeID == 0 {
		t.Fatal("could not create or find test store")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	adminID, err := userRepo.Create(ctx, &auth.User{
		Username:     fmt.Sprintf("rpt_adm_%s", tag), // max 14 chars
		FullName:     "Report Admin",
		PasswordHash: string(hash),
		Role:         "admin",
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}

	cashierID, err := userRepo.Create(ctx, &auth.User{
		Username:     fmt.Sprintf("rpt_cas_%s", tag), // max 14 chars
		FullName:     "Report Cashier",
		PasswordHash: string(hash),
		Role:         "cashier",
		StoreID:      &storeID,
	})
	if err != nil {
		t.Fatalf("create cashier: %v", err)
	}

	prodID, err := productRepo.Create(ctx, product.CreateInput{
		Code:      fmt.Sprintf("RPT%s", tag), // max 10 chars, fits VARCHAR(50)
		Name:      "Report Test Fabric",
		Unit:      "meter",
		SellPrice: decimal.NewFromInt(10000),
		CostPrice: decimal.NewFromInt(7000),
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	// Seed 20m in inventory with a matching movement so the ledger invariant holds.
	if _, err := database.ExecContext(ctx,
		"INSERT INTO inventory (store_id, product_id, qty) VALUES (?, ?, 20) ON DUPLICATE KEY UPDATE qty = 20",
		storeID, prodID,
	); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}
	_, _ = database.ExecContext(ctx,
		`INSERT INTO stock_movements (store_id, product_id, delta, qty_after, reason, ref_type, created_by)
		 VALUES (?, ?, 20, 20, 'adjustment', 'manual', ?)`,
		storeID, prodID, adminID,
	)

	return &fixture{
		ctx:      ctx,
		db:       database,
		storeID:  storeID,
		adminActor:   auth.Actor{UserID: adminID, Role: "admin"},
		cashierActor: auth.Actor{UserID: cashierID, Role: "cashier", StoreID: &storeID},
		productID: prodID,
		orderSvc:  order.NewService(database, orderRepo, stockWriter, productRepo, storeRepo),
		reportSvc: report.NewService(database),
	}
}

// TestSalesReport_GrandTotalReconciles verifies that:
//  1. SalesReport.GrandTotal equals SUM(orders.total) from the DB for our store.
//  2. SUM(ByMethod.Total) equals SUM(payments.amount) from the DB for the same store.
//
// Three orders are created (2 fully paid, 1 partial) so we exercise both paths.
func TestSalesReport_GrandTotalReconciles(t *testing.T) {
	f := setupFixture(t)
	defer f.db.Close()

	// Order 1: 3m × Rp10 000 = Rp30 000, fully paid CASH.
	o1, err := f.orderSvc.CreateOrder(f.ctx, f.cashierActor, order.CreateOrderInput{
		Items: []order.OrderItemInput{{ProductID: f.productID, Qty: decimal.NewFromInt(3)}},
	})
	if err != nil {
		t.Fatalf("create order 1: %v", err)
	}
	if _, err := f.orderSvc.AddPayment(f.ctx, f.cashierActor, o1.ID, order.AddPaymentInput{
		Method: "CASH", Amount: decimal.NewFromInt(30000),
	}); err != nil {
		t.Fatalf("pay order 1: %v", err)
	}

	// Order 2: 2m × Rp10 000 = Rp20 000, fully paid QRIS.
	o2, err := f.orderSvc.CreateOrder(f.ctx, f.cashierActor, order.CreateOrderInput{
		Items: []order.OrderItemInput{{ProductID: f.productID, Qty: decimal.NewFromInt(2)}},
	})
	if err != nil {
		t.Fatalf("create order 2: %v", err)
	}
	if _, err := f.orderSvc.AddPayment(f.ctx, f.cashierActor, o2.ID, order.AddPaymentInput{
		Method: "QRIS", Amount: decimal.NewFromInt(20000),
	}); err != nil {
		t.Fatalf("pay order 2: %v", err)
	}

	// Order 3: 2m × Rp10 000 = Rp20 000, partially paid Rp5 000 TRANSFER.
	o3, err := f.orderSvc.CreateOrder(f.ctx, f.cashierActor, order.CreateOrderInput{
		Items: []order.OrderItemInput{{ProductID: f.productID, Qty: decimal.NewFromInt(2)}},
	})
	if err != nil {
		t.Fatalf("create order 3: %v", err)
	}
	if _, err := f.orderSvc.AddPayment(f.ctx, f.cashierActor, o3.ID, order.AddPaymentInput{
		Method: "TRANSFER", Amount: decimal.NewFromInt(5000),
	}); err != nil {
		t.Fatalf("partial pay order 3: %v", err)
	}

	// Run SalesReport scoped to our test store with today's default date range.
	result, err := f.reportSvc.SalesReport(f.ctx, f.adminActor, report.SalesFilter{
		StoreID: f.storeID,
	})
	if err != nil {
		t.Fatalf("SalesReport: %v", err)
	}

	// ── Check 1: GrandTotal == SUM(orders.total) from DB ─────────────────────
	var dbOrderTotal decimal.Decimal
	if err := f.db.GetContext(f.ctx, &dbOrderTotal,
		`SELECT COALESCE(SUM(total), 0) FROM orders
		  WHERE store_id = ?
		    AND DATE(CONVERT_TZ(created_at,'+00:00','+07:00')) = CURDATE()`,
		f.storeID,
	); err != nil {
		t.Fatalf("query db order total: %v", err)
	}

	wantGrand := decimal.NewFromInt(70000) // 30000 + 20000 + 20000
	if !result.GrandTotal.Equal(wantGrand) {
		t.Errorf("GrandTotal: want %s, got %s", wantGrand, result.GrandTotal)
	}
	if !result.GrandTotal.Equal(dbOrderTotal) {
		t.Errorf("GrandTotal %s != DB SUM(orders.total) %s", result.GrandTotal, dbOrderTotal)
	}
	t.Logf("GrandTotal reconciled with orders table: %s ✓", result.GrandTotal)

	// ── Check 2: SUM(ByMethod.Total) == SUM(payments.amount) from DB ─────────
	var dbPaymentTotal decimal.Decimal
	if err := f.db.GetContext(f.ctx, &dbPaymentTotal,
		`SELECT COALESCE(SUM(p.amount), 0)
		   FROM payments p
		   JOIN orders o ON o.id = p.order_id
		  WHERE o.store_id = ?
		    AND DATE(CONVERT_TZ(p.created_at,'+00:00','+07:00')) = CURDATE()`,
		f.storeID,
	); err != nil {
		t.Fatalf("query db payment total: %v", err)
	}

	var byMethodSum decimal.Decimal
	for _, m := range result.ByMethod {
		byMethodSum = byMethodSum.Add(m.Total)
	}

	wantPaid := decimal.NewFromInt(55000) // 30000 + 20000 + 5000
	if !byMethodSum.Equal(wantPaid) {
		t.Errorf("SUM(ByMethod.Total): want %s, got %s", wantPaid, byMethodSum)
	}
	if !byMethodSum.Equal(dbPaymentTotal) {
		t.Errorf("SUM(ByMethod.Total) %s != DB SUM(payments.amount) %s", byMethodSum, dbPaymentTotal)
	}
	t.Logf("ByMethod sum reconciled with payments table: %s ✓", byMethodSum)

	// ── Check 3: per-method breakdown matches known payments ─────────────────
	methodTotals := make(map[string]decimal.Decimal, len(result.ByMethod))
	for _, m := range result.ByMethod {
		methodTotals[m.Method] = m.Total
	}
	for method, want := range map[string]int64{"CASH": 30000, "QRIS": 20000, "TRANSFER": 5000} {
		got, ok := methodTotals[method]
		if !ok {
			t.Errorf("missing method %s in ByMethod breakdown", method)
			continue
		}
		if !got.Equal(decimal.NewFromInt(want)) {
			t.Errorf("ByMethod[%s]: want %d, got %s", method, want, got)
		}
	}
	t.Log("per-method breakdown correct ✓")
}

// TestStockReport_MovementsMatchDB verifies that StockReport (kartu stok) rows
// are consistent with the stock_movements table:
//   - row count matches DB COUNT(*)
//   - sum(delta) in report rows matches DB SUM(delta)
//   - both equal inventory.qty (ledger invariant 2)
func TestStockReport_MovementsMatchDB(t *testing.T) {
	f := setupFixture(t)
	defer f.db.Close()

	// Create 2 sale orders to produce known stock movements: -3m and -4m.
	if _, err := f.orderSvc.CreateOrder(f.ctx, f.cashierActor, order.CreateOrderInput{
		Items: []order.OrderItemInput{{ProductID: f.productID, Qty: decimal.NewFromInt(3)}},
	}); err != nil {
		t.Fatalf("create order 1: %v", err)
	}
	if _, err := f.orderSvc.CreateOrder(f.ctx, f.cashierActor, order.CreateOrderInput{
		Items: []order.OrderItemInput{{ProductID: f.productID, Qty: decimal.NewFromInt(4)}},
	}); err != nil {
		t.Fatalf("create order 2: %v", err)
	}

	result, err := f.reportSvc.StockReport(f.ctx, f.adminActor, report.StockFilter{
		ProductID: f.productID,
		StoreID:   f.storeID,
	})
	if err != nil {
		t.Fatalf("StockReport: %v", err)
	}

	// ── Check 1: row count matches DB ─────────────────────────────────────────
	var dbCount int
	if err := f.db.GetContext(f.ctx, &dbCount,
		`SELECT COUNT(*) FROM stock_movements WHERE store_id = ? AND product_id = ?`,
		f.storeID, f.productID,
	); err != nil {
		t.Fatalf("count movements: %v", err)
	}

	if result.Total != dbCount {
		t.Errorf("StockReport.Total=%d, DB COUNT=%d", result.Total, dbCount)
	}
	t.Logf("movement count reconciled: %d ✓", result.Total)

	// ── Check 2: sum(delta) in report rows matches DB ─────────────────────────
	var dbDeltaSum decimal.Decimal
	if err := f.db.GetContext(f.ctx, &dbDeltaSum,
		`SELECT COALESCE(SUM(delta), 0) FROM stock_movements WHERE store_id = ? AND product_id = ?`,
		f.storeID, f.productID,
	); err != nil {
		t.Fatalf("sum delta: %v", err)
	}

	var reportDeltaSum decimal.Decimal
	for _, row := range result.Data {
		reportDeltaSum = reportDeltaSum.Add(row.Delta)
	}

	if !reportDeltaSum.Equal(dbDeltaSum) {
		t.Errorf("report sum(delta)=%s != DB sum(delta)=%s", reportDeltaSum, dbDeltaSum)
	}
	t.Logf("sum(delta) reconciled with stock_movements: %s ✓", reportDeltaSum)

	// ── Check 3: sum(delta) matches current inventory.qty (invariant 2) ───────
	var inventoryQty decimal.Decimal
	if err := f.db.GetContext(f.ctx, &inventoryQty,
		`SELECT qty FROM inventory WHERE store_id = ? AND product_id = ?`,
		f.storeID, f.productID,
	); err != nil {
		t.Fatalf("get inventory qty: %v", err)
	}

	if !dbDeltaSum.Equal(inventoryQty) {
		t.Errorf("ledger invariant 2 broken: SUM(delta)=%s != inventory.qty=%s", dbDeltaSum, inventoryQty)
	}
	t.Logf("ledger invariant 2 holds: SUM(delta)=inventory.qty=%s ✓", inventoryQty)

	// ── Check 4: net movement matches expected (+20 seed − 3 − 4 = +13) ───────
	expected := decimal.NewFromInt(13)
	if !dbDeltaSum.Equal(expected) {
		t.Errorf("expected net delta=%s (20−3−4), got %s", expected, dbDeltaSum)
	}
	t.Logf("net movement correct: %s ✓", dbDeltaSum)
}
