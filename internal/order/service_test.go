package order_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
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

func TestConcurrentOrders_ExactlyTenSucceed(t *testing.T) {
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

	// Create test store.
	testStoreID, err := storeRepo.Create(ctx, &store.Store{Code: "CT", Name: "Concurrent Test Store"})
	if err != nil {
		stores, _ := storeRepo.List(ctx)
		for _, s := range stores {
			if s.Code == "CT" {
				testStoreID = s.ID
				break
			}
		}
	}
	if testStoreID == 0 {
		t.Fatal("could not create or find test store")
	}

	// Create test cashier — needed to satisfy orders.cashier_id FK.
	hash, _ := bcrypt.GenerateFromPassword([]byte("test123"), bcrypt.MinCost)
	cashierID, err := userRepo.Create(ctx, &auth.User{
		Username:     fmt.Sprintf("cashier_conc_%d", os.Getpid()),
		FullName:     "Test Cashier",
		PasswordHash: string(hash),
		Role:         "cashier",
		StoreID:      &testStoreID,
	})
	if err != nil {
		t.Fatalf("create test cashier: %v", err)
	}

	// Create test product.
	testProductID, err := productRepo.Create(ctx, product.CreateInput{
		Code:      fmt.Sprintf("CONC-%d", os.Getpid()),
		Name:      "Concurrent Test Product",
		Unit:      "pcs",
		SellPrice: decimal.NewFromInt(10000),
		CostPrice: decimal.NewFromInt(8000),
	})
	if err != nil {
		t.Fatalf("create test product: %v", err)
	}

	// Seed exactly 10 units in inventory.
	_, err = database.ExecContext(ctx,
		"INSERT INTO inventory (store_id, product_id, qty) VALUES (?, ?, 10) ON DUPLICATE KEY UPDATE qty = 10",
		testStoreID, testProductID,
	)
	if err != nil {
		t.Fatalf("seed inventory: %v", err)
	}
	_, _ = database.ExecContext(ctx,
		`INSERT INTO stock_movements (store_id, product_id, delta, qty_after, reason, ref_type, created_by)
		 VALUES (?, ?, 10, 10, 'adjustment', 'manual', ?)`,
		testStoreID, testProductID, cashierID,
	)

	actor := auth.Actor{UserID: cashierID, Role: "cashier", StoreID: &testStoreID}

	// Fire 20 goroutines simultaneously — only 10 should succeed.
	const goroutines = 20
	var wg sync.WaitGroup
	var succeeded, failed atomic.Int32
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.CreateOrder(ctx, actor, order.CreateOrderInput{
				Items: []order.OrderItemInput{
					{ProductID: testProductID, Qty: decimal.NewFromInt(1)},
				},
			})
			if err != nil {
				t.Logf("order error: %v", err)
				failed.Add(1)
			} else {
				succeeded.Add(1)
			}
		}()
	}

	close(start)
	wg.Wait()

	t.Logf("succeeded=%d  failed=%d", succeeded.Load(), failed.Load())

	if succeeded.Load() != 10 {
		t.Errorf("expected exactly 10 successful orders, got %d", succeeded.Load())
	}
	if failed.Load() != 10 {
		t.Errorf("expected exactly 10 failed orders, got %d", failed.Load())
	}

	// Inventory must be 0.
	var qtyLeft decimal.Decimal
	if err := database.GetContext(ctx, &qtyLeft,
		"SELECT qty FROM inventory WHERE store_id = ? AND product_id = ?",
		testStoreID, testProductID,
	); err != nil {
		t.Fatalf("check inventory: %v", err)
	}
	if !qtyLeft.IsZero() {
		t.Errorf("inventory not zero: got %s", qtyLeft)
	}

	// Ledger invariant: SUM(sale delta) must equal -10.
	var sumDelta decimal.Decimal
	if err := database.GetContext(ctx, &sumDelta,
		`SELECT COALESCE(SUM(delta), 0) FROM stock_movements
		  WHERE store_id = ? AND product_id = ? AND reason = 'sale'`,
		testStoreID, testProductID,
	); err != nil {
		t.Fatalf("check ledger: %v", err)
	}
	if !sumDelta.Equal(decimal.NewFromInt(-10)) {
		t.Errorf("ledger invariant broken: expected sum(delta)=-10, got %s", sumDelta)
	}
	t.Log("ledger invariant ✓")
}
