package order_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/config"
	"inventory-manager/internal/db"
	"inventory-manager/internal/order"
	"inventory-manager/internal/product"
	"inventory-manager/internal/stock"
	"inventory-manager/internal/store"
)

func TestVoidOrder_RestoresStockExactlyOnce(t *testing.T) {
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	database, err := db.New(cfg.DBTestDSN)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	storeRepo := store.NewRepository(database)
	userRepo := auth.NewRepository(database)
	productRepo := product.NewRepository(database)
	stockWriter := stock.NewWriter(database)
	orderRepo := order.NewRepository(database)
	svc := order.NewService(database, orderRepo, stockWriter, productRepo, storeRepo)

	// Setup: store, admin, cashier, product.
	pid := os.Getpid()
	storeCode := fmt.Sprintf("VD%d", pid%100)
	testStoreID, err := storeRepo.Create(ctx, &store.Store{Code: storeCode, Name: "Void Test Store"})
	if err != nil {
		stores, _ := storeRepo.List(ctx)
		for _, s := range stores {
			if s.Code == storeCode {
				testStoreID = s.ID
				break
			}
		}
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	adminID, err := userRepo.Create(ctx, &auth.User{
		Username:     fmt.Sprintf("vdadmin_%d", pid),
		FullName:     "Void Admin",
		PasswordHash: string(hash),
		Role:         "admin",
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	cashierID, err := userRepo.Create(ctx, &auth.User{
		Username:     fmt.Sprintf("vdcashier_%d", pid),
		FullName:     "Void Cashier",
		PasswordHash: string(hash),
		Role:         "cashier",
		StoreID:      &testStoreID,
	})
	if err != nil {
		t.Fatalf("create cashier: %v", err)
	}

	prodID, err := productRepo.Create(ctx, product.CreateInput{
		Code:      fmt.Sprintf("VD-%d", pid),
		Name:      "Void Test Product",
		Unit:      "pcs",
		SellPrice: decimal.NewFromInt(5000),
		CostPrice: decimal.NewFromInt(3000),
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	// Seed 10 units.
	_, _ = database.ExecContext(ctx,
		"INSERT INTO inventory (store_id, product_id, qty) VALUES (?, ?, 10) ON DUPLICATE KEY UPDATE qty = 10",
		testStoreID, prodID,
	)
	_, _ = database.ExecContext(ctx,
		`INSERT INTO stock_movements (store_id, product_id, delta, qty_after, reason, ref_type, created_by)
		 VALUES (?, ?, 10, 10, 'adjustment', 'manual', ?)`,
		testStoreID, prodID, adminID,
	)

	cashierActor := auth.Actor{UserID: cashierID, Role: "cashier", StoreID: &testStoreID}
	adminActor := auth.Actor{UserID: adminID, Role: "admin"}

	// Create an order for 3 units → stock becomes 7.
	resp, err := svc.CreateOrder(ctx, cashierActor, order.CreateOrderInput{
		Items: []order.OrderItemInput{
			{ProductID: prodID, Qty: decimal.NewFromInt(3)},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	orderID := resp.ID

	checkQty := func(want int64) {
		t.Helper()
		var qty decimal.Decimal
		_ = database.GetContext(ctx, &qty,
			"SELECT qty FROM inventory WHERE store_id = ? AND product_id = ?",
			testStoreID, prodID,
		)
		if !qty.Equal(decimal.NewFromInt(want)) {
			t.Errorf("expected qty=%d, got %s", want, qty)
		}
	}
	checkQty(7)

	// First void → stock restored to 10.
	if err := svc.VoidOrder(ctx, adminActor, orderID, order.VoidInput{Reason: "test void"}); err != nil {
		t.Fatalf("first void: %v", err)
	}
	checkQty(10)

	// Second void → must return 409, stock stays at 10.
	err = svc.VoidOrder(ctx, adminActor, orderID, order.VoidInput{Reason: "double void"})
	if err == nil {
		t.Fatal("expected error on double void, got nil")
	}
	checkQty(10)
	t.Logf("double-void correctly rejected: %v", err)

	// Verify ledger: sale(-3) + sale_void(+3) = 0 net for this product.
	var sumDelta decimal.Decimal
	_ = database.GetContext(ctx, &sumDelta,
		`SELECT COALESCE(SUM(delta), 0) FROM stock_movements
		  WHERE store_id = ? AND product_id = ? AND reason IN ('sale','sale_void')`,
		testStoreID, prodID,
	)
	if !sumDelta.IsZero() {
		t.Errorf("ledger not balanced after void: sum(delta)=%s", sumDelta)
	}
	t.Log("ledger balanced after void ✓")
}
