package httpapi_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/category"
	"inventory-manager/internal/config"
	"inventory-manager/internal/db"
	"inventory-manager/internal/httpapi"
	"inventory-manager/internal/product"
	"inventory-manager/internal/salesperson"
	"inventory-manager/internal/store"
	"inventory-manager/internal/supplier"
)

func newTestRouter(t *testing.T) (*httpapi.Services, func()) {
	t.Helper()
	_ = godotenv.Load("../../.env")
	cfg := config.Load()

	database, err := db.New(cfg.DBTestDSN)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}

	storeRepo := store.NewRepository(database)
	userRepo := auth.NewRepository(database)

	svc := &httpapi.Services{
		Auth:        auth.NewService(userRepo, storeRepo, cfg.JWTSecret),
		AuthRepo:    userRepo,
		Category:    category.NewService(category.NewRepository(database)),
		Supplier:    supplier.NewService(supplier.NewRepository(database)),
		Salesperson: salesperson.NewService(salesperson.NewRepository(database)),
		Product:     product.NewService(product.NewRepository(database)),
	}

	return svc, func() { database.Close() }
}

func TestHealthz(t *testing.T) {
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	database, err := db.New(cfg.DBTestDSN)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}
	defer database.Close()

	storeRepo := store.NewRepository(database)
	userRepo := auth.NewRepository(database)
	svc := httpapi.Services{
		Auth:        auth.NewService(userRepo, storeRepo, cfg.JWTSecret),
		AuthRepo:    userRepo,
		Category:    category.NewService(category.NewRepository(database)),
		Supplier:    supplier.NewService(supplier.NewRepository(database)),
		Salesperson: salesperson.NewService(salesperson.NewRepository(database)),
		Product:     product.NewService(product.NewRepository(database)),
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	router := httpapi.New(database, logger, "*", svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %q", body["status"])
	}
}
