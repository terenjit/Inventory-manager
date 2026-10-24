package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/category"
	"inventory-manager/internal/config"
	"inventory-manager/internal/db"
	"inventory-manager/internal/httpapi"
	"inventory-manager/internal/order"
	"inventory-manager/internal/product"
	"inventory-manager/internal/salesperson"
	"inventory-manager/internal/stock"
	"inventory-manager/internal/store"
	"inventory-manager/internal/supplier"
)

func buildTestRouter(t *testing.T) (http.Handler, func()) {
	t.Helper()
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	database, err := db.New(cfg.DBTestDSN)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}

	storeRepo := store.NewRepository(database)
	userRepo := auth.NewRepository(database)
	stockWriter := stock.NewWriter(database)
	orderRepo := order.NewRepository(database)
	productRepo := product.NewRepository(database)

	svc := httpapi.Services{
		Auth:        auth.NewService(userRepo, storeRepo, cfg.JWTSecret),
		AuthRepo:    userRepo,
		Category:    category.NewService(category.NewRepository(database)),
		Supplier:    supplier.NewService(supplier.NewRepository(database)),
		Salesperson: salesperson.NewService(salesperson.NewRepository(database)),
		Product:     product.NewService(productRepo),
		Order:       order.NewService(database, orderRepo, stockWriter, productRepo, storeRepo),
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	return httpapi.New(database, logger, "*", svc), func() { database.Close() }
}

func TestDeactivatedUserRejected(t *testing.T) {
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	database, err := db.New(cfg.DBTestDSN)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}
	defer database.Close()

	storeRepo := store.NewRepository(database)
	userRepo := auth.NewRepository(database)
	stockWriter := stock.NewWriter(database)
	orderRepo := order.NewRepository(database)
	productRepo := product.NewRepository(database)

	svc := httpapi.Services{
		Auth:        auth.NewService(userRepo, storeRepo, cfg.JWTSecret),
		AuthRepo:    userRepo,
		Category:    category.NewService(category.NewRepository(database)),
		Supplier:    supplier.NewService(supplier.NewRepository(database)),
		Salesperson: salesperson.NewService(salesperson.NewRepository(database)),
		Product:     product.NewService(productRepo),
		Order:       order.NewService(database, orderRepo, stockWriter, productRepo, storeRepo),
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	router := httpapi.New(database, logger, "*", svc)

	ctx := context.Background()

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	username := fmt.Sprintf("deact_%d", os.Getpid())
	uid, err := userRepo.Create(ctx, &auth.User{
		Username:     username,
		FullName:     "Test User",
		PasswordHash: string(hash),
		Role:         "admin",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"username": username, "password": "pass123"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]

	f := false
	if err := userRepo.Update(ctx, uid, auth.UpdateUserInput{IsActive: &f}); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(cookie)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for deactivated user, got %d", w.Code)
	}
}

func TestCashierBlockedFromUserEndpoints(t *testing.T) {
	router, cleanup := buildTestRouter(t)
	defer cleanup()

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

	storeCode := fmt.Sprintf("X%d", os.Getpid()%100)
	storeID, err := storeRepo.Create(ctx, &store.Store{Code: storeCode, Name: "Test Store"})
	if err != nil {
		t.Fatalf("create test store: %v", err)
	}

	cashierName := fmt.Sprintf("cashier_%d", os.Getpid())
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	_, err = userRepo.Create(ctx, &auth.User{
		Username:     cashierName,
		FullName:     "Cashier",
		PasswordHash: string(hash),
		Role:         "cashier",
		StoreID:      &storeID,
	})
	if err != nil {
		t.Fatalf("create cashier: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"username": cashierName, "password": "pass123"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/users", nil)
	req.AddCookie(cookie)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cashier on /users, got %d", w.Code)
	}
}
