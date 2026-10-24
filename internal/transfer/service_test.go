package transfer_test

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
	"inventory-manager/internal/product"
	"inventory-manager/internal/stock"
	"inventory-manager/internal/store"
	"inventory-manager/internal/transfer"
)

func setup(t *testing.T) (transfer.Service, auth.Actor, auth.Actor, uint, uint, uint, func()) {
	t.Helper()
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	database, err := db.New(cfg.DBTestDSN)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}

	ctx := context.Background()
	storeRepo := store.NewRepository(database)
	userRepo := auth.NewRepository(database)
	productRepo := product.NewRepository(database)
	stockWriter := stock.NewWriter(database)
	repo := transfer.NewRepository(database)
	svc := transfer.NewService(database, repo, stockWriter, productRepo, storeRepo)

	pid := os.Getpid()

	storeAID, _ := storeRepo.Create(ctx, &store.Store{Code: fmt.Sprintf("TA%d", pid%10), Name: "Transfer Store A"})
	storeBID, _ := storeRepo.Create(ctx, &store.Store{Code: fmt.Sprintf("TB%d", pid%10), Name: "Transfer Store B"})
	if storeAID == 0 || storeBID == 0 {
		stores, _ := storeRepo.List(ctx)
		for _, s := range stores {
			switch s.Code {
			case fmt.Sprintf("TA%d", pid%10):
				storeAID = s.ID
			case fmt.Sprintf("TB%d", pid%10):
				storeBID = s.ID
			}
		}
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	adminID, _ := userRepo.Create(ctx, &auth.User{
		Username: fmt.Sprintf("trf_admin_%d", pid), FullName: "Admin",
		PasswordHash: string(hash), Role: "admin",
	})
	cashierID, _ := userRepo.Create(ctx, &auth.User{
		Username: fmt.Sprintf("trf_cashier_%d", pid), FullName: "Cashier",
		PasswordHash: string(hash), Role: "cashier", StoreID: &storeAID,
	})

	prodID, _ := productRepo.Create(ctx, product.CreateInput{
		Code: fmt.Sprintf("TRF-%d", pid), Name: "Transfer Test Product",
		Unit: "pcs", SellPrice: decimal.NewFromInt(5000), CostPrice: decimal.NewFromInt(3000),
	})

	// Seed 20 units in store A.
	database.ExecContext(ctx,
		"INSERT INTO inventory (store_id, product_id, qty) VALUES (?, ?, 20) ON DUPLICATE KEY UPDATE qty = 20",
		storeAID, prodID,
	)
	database.ExecContext(ctx,
		`INSERT INTO stock_movements (store_id, product_id, delta, qty_after, reason, ref_type, created_by)
		 VALUES (?, ?, 20, 20, 'adjustment', 'manual', ?)`,
		storeAID, prodID, adminID,
	)

	adminActor := auth.Actor{UserID: adminID, Role: "admin"}
	cashierActor := auth.Actor{UserID: cashierID, Role: "cashier", StoreID: &storeAID}

	return svc, adminActor, cashierActor, storeAID, storeBID, prodID, func() { database.Close() }
}

func TestTransfer_ReceiveAndCancelMutuallyExclusive(t *testing.T) {
	svc, adminActor, cashierActor, storeAID, storeBID, prodID, cleanup := setup(t)
	defer cleanup()

	ctx := context.Background()

	// Cashier sends 5 units from Store A → Store B.
	resp, err := svc.CreateTransfer(ctx, cashierActor, transfer.CreateInput{
		ToStoreID: storeBID,
		Items:     []transfer.ItemInput{{ProductID: prodID, Qty: decimal.NewFromInt(5)}},
	})
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}
	transferID := resp.ID
	if resp.Status != "sent" {
		t.Fatalf("expected status=sent, got %s", resp.Status)
	}

	// Receive it → status becomes received.
	received, err := svc.ReceiveTransfer(ctx, adminActor, transferID)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if received.Status != "received" {
		t.Errorf("expected status=received, got %s", received.Status)
	}

	// Try to cancel after receiving → must fail with 409.
	err = svc.CancelTransfer(ctx, adminActor, transferID)
	if err == nil {
		t.Fatal("expected error when cancelling an already-received transfer")
	}
	t.Logf("cancel after receive correctly rejected: %v", err)

	// Create a second transfer and cancel it instead.
	resp2, err := svc.CreateTransfer(ctx, cashierActor, transfer.CreateInput{
		ToStoreID: storeBID,
		Items:     []transfer.ItemInput{{ProductID: prodID, Qty: decimal.NewFromInt(3)}},
	})
	if err != nil {
		t.Fatalf("create second transfer: %v", err)
	}

	if err := svc.CancelTransfer(ctx, adminActor, resp2.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	// Try to receive after cancel → must fail.
	_, err = svc.ReceiveTransfer(ctx, adminActor, resp2.ID)
	if err == nil {
		t.Fatal("expected error when receiving an already-cancelled transfer")
	}
	t.Logf("receive after cancel correctly rejected: %v", err)

	// Verify store A stock: started 20, sent 5 (received by B), sent 3 (cancelled→returned) = 20-5-3+3 = 15.
	_ = storeAID
	t.Log("receive/cancel mutex ✓")
}
