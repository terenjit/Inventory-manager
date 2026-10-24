package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"inventory-manager/internal/auth"
	"inventory-manager/internal/category"
	"inventory-manager/internal/config"
	"inventory-manager/internal/datasheet"
	"inventory-manager/internal/db"
	"inventory-manager/internal/httpapi"
	"inventory-manager/internal/opname"
	"inventory-manager/internal/order"
	"inventory-manager/internal/product"
	"inventory-manager/internal/purchase"
	"inventory-manager/internal/report"
	"inventory-manager/internal/salesperson"
	"inventory-manager/internal/stock"
	"inventory-manager/internal/store"
	"inventory-manager/internal/supplier"
	"inventory-manager/internal/supplierbill"
	"inventory-manager/internal/transfer"
	"inventory-manager/internal/webui"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := config.Load()

	database, err := db.New(cfg.DBDSN)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	logger.Info("database connected")

	// Repositories
	storeRepo := store.NewRepository(database)
	userRepo := auth.NewRepository(database)
	categoryRepo := category.NewRepository(database)
	supplierRepo := supplier.NewRepository(database)
	salespersonRepo := salesperson.NewRepository(database)
	productRepo := product.NewRepository(database)

	stockWriter := stock.NewWriter(database)
	orderRepo := order.NewRepository(database)
	transferRepo := transfer.NewRepository(database)

	// Services
	svc := httpapi.Services{
		Auth:         auth.NewService(userRepo, storeRepo, cfg.JWTSecret),
		AuthRepo:     userRepo,
		Category:     category.NewService(categoryRepo),
		Supplier:     supplier.NewService(supplierRepo),
		Salesperson:  salesperson.NewService(salespersonRepo),
		Product:      product.NewService(productRepo),
		Order:        order.NewService(database, orderRepo, stockWriter, productRepo, storeRepo),
		Transfer:     transfer.NewService(database, transferRepo, stockWriter, productRepo, storeRepo),
		Purchase:     purchase.NewService(purchase.NewRepository(database), productRepo, storeRepo, supplierRepo),
		Opname:       opname.NewService(database, opname.NewRepository(database), stockWriter, productRepo, storeRepo),
		Datasheet:    datasheet.NewService(database, stockWriter, storeRepo),
		Report:       report.NewService(database),
		SupplierBill: supplierbill.NewService(supplierbill.NewRepository(database)),
		StoreRepo:    storeRepo,
	}

	// Build the engine: web UI (HTML pages) first, then API routes.
	router := gin.New()
	router.UseRawPath = true       // keep %2F encoded in path params (order numbers contain /)
	router.UnescapePathValues = true // still decode the value when reading c.Param()
	router.Use(httpapi.RequestID())
	router.Use(httpapi.Logger(logger))
	router.Use(gin.Recovery())

	webui.RegisterRoutes(router, webui.Services{
		Auth:         svc.Auth,
		AuthRepo:     svc.AuthRepo,
		Product:      svc.Product,
		Order:        svc.Order,
		Salesperson:  svc.Salesperson,
		Transfer:     svc.Transfer,
		SupplierBill: svc.SupplierBill,
		Report:       svc.Report,
		Store:        storeRepo,
	}, "1.0.0")

	httpapi.Register(router, database, svc)

	srv := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server starting", "port", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("forced shutdown", "error", err)
	}
}
