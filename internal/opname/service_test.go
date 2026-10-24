package opname_test

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
	"inventory-manager/internal/opname"
	"inventory-manager/internal/order"
	"inventory-manager/internal/product"
	"inventory-manager/internal/stock"
	"inventory-manager/internal/store"
)

func TestFinalize_SaleDuringOpnameNotLost(t *testing.T) {
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	database, err := db.New(cfg.DBTestDSN)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	pid := os.Getpid()

	storeRepo := store.NewRepository(database)
	userRepo := auth.NewRepository(database)
	productRepo := product.NewRepository(database)
	stockWriter := stock.NewWriter(database)
	opnameRepo := opname.NewRepository(database)
	opnameSvc := opname.NewService(database, opnameRepo, stockWriter, productRepo, storeRepo)
	orderRepo := order.NewRepository(database)
	orderSvc := order.NewService(database, orderRepo, stockWriter, productRepo, storeRepo)

	// Setup: store, admin, cashier, product.
	storeCode := fmt.Sprintf("OP%d", pid%10)
	testStoreID, err := storeRepo.Create(ctx, &store.Store{Code: storeCode, Name: "Opname Test Store"})
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
	adminID, _ := userRepo.Create(ctx, &auth.User{
		Username: fmt.Sprintf("op_admin_%d", pid), FullName: "Admin",
		PasswordHash: string(hash), Role: "admin",
	})
	cashierID, _ := userRepo.Create(ctx, &auth.User{
		Username: fmt.Sprintf("op_cashier_%d", pid), FullName: "Cashier",
		PasswordHash: string(hash), Role: "cashier", StoreID: &testStoreID,
	})

	prodID, _ := productRepo.Create(ctx, product.CreateInput{
		Code: fmt.Sprintf("OP-%d", pid), Name: "Opname Test Product",
		Unit: "pcs", SellPrice: decimal.NewFromInt(10000), CostPrice: decimal.NewFromInt(7000),
	})

	// Clean up any leftover draft opname for this store from prior runs.
	database.ExecContext(ctx,
		"UPDATE stock_opnames SET status = 'cancelled' WHERE store_id = ? AND status = 'draft'",
		testStoreID,
	)

	// Seed 100 units.
	database.ExecContext(ctx,
		"INSERT INTO inventory (store_id, product_id, qty) VALUES (?, ?, 100) ON DUPLICATE KEY UPDATE qty=100",
		testStoreID, prodID,
	)
	database.ExecContext(ctx,
		`INSERT INTO stock_movements (store_id, product_id, delta, qty_after, reason, ref_type, created_by)
		 VALUES (?, ?, 100, 100, 'adjustment', 'manual', ?)`,
		testStoreID, prodID, adminID,
	)

	adminActor := auth.Actor{UserID: adminID, Role: "admin"}
	cashierActor := auth.Actor{UserID: cashierID, Role: "cashier", StoreID: &testStoreID}

	// Step 1: Start opname. system_qty snapshot = 100.
	storeIDPtr := testStoreID
	op, err := opnameSvc.StartOpname(ctx, adminActor, opname.CreateInput{StoreID: &storeIDPtr})
	if err != nil {
		t.Fatalf("start opname: %v", err)
	}
	opID := op.ID

	// Step 2: Add product line (snapshots qty=100).
	_, err = opnameSvc.AddLine(ctx, adminActor, opID, opname.AddLineInput{ProductID: prodID})
	if err != nil {
		t.Fatalf("add line: %v", err)
	}

	// Step 3: Make a sale of 5 units DURING the open opname. Stock: 100 → 95.
	_, err = orderSvc.CreateOrder(ctx, cashierActor, order.CreateOrderInput{
		Items: []order.OrderItemInput{{ProductID: prodID, Qty: decimal.NewFromInt(5)}},
	})
	if err != nil {
		t.Fatalf("sale during opname: %v", err)
	}

	checkQty := func(want int64) {
		t.Helper()
		var qty decimal.Decimal
		database.GetContext(ctx, &qty,
			"SELECT qty FROM inventory WHERE store_id=? AND product_id=?", testStoreID, prodID)
		if !qty.Equal(decimal.NewFromInt(want)) {
			t.Errorf("expected qty=%d, got %s", want, qty)
		}
	}
	checkQty(95) // sale took 5

	// Step 4: Admin counts 98 (found 98 physically; 2 are missing/damaged).
	// system_snapshot=100, counted=98, delta=-2
	// Apply delta to CURRENT stock (95): 95 + (-2) = 93
	err = opnameSvc.UpdateLine(ctx, adminActor, opID, opname.UpdateLineInput{
		CountedQty: decimal.NewFromInt(98),
	}, prodID)
	if err != nil {
		t.Fatalf("update line: %v", err)
	}

	// Step 5: Finalize.
	_, err = opnameSvc.Finalize(ctx, adminActor, opID)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}

	// The sale (-5) must NOT be undone. Only the -2 discrepancy is applied.
	// Expected: 100 - 5 (sale) - 2 (opname correction) = 93
	checkQty(93)

	// Verify the sale movement still exists.
	var saleCount int
	database.GetContext(ctx, &saleCount,
		`SELECT COUNT(*) FROM stock_movements WHERE store_id=? AND product_id=? AND reason='sale'`,
		testStoreID, prodID,
	)
	if saleCount == 0 {
		t.Error("sale movement was lost after finalize")
	}

	// Verify opname movement = -2.
	var opnameDelta decimal.Decimal
	database.GetContext(ctx, &opnameDelta,
		`SELECT COALESCE(SUM(delta), 0) FROM stock_movements WHERE store_id=? AND product_id=? AND reason='opname'`,
		testStoreID, prodID,
	)
	if !opnameDelta.Equal(decimal.NewFromInt(-2)) {
		t.Errorf("expected opname delta=-2, got %s", opnameDelta)
	}

	t.Logf("sale during opname not lost ✓  final_qty=93 (100 - 5 sale - 2 opname)")
}
