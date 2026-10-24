package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"inventory-manager/internal/apperr"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/category"
	"inventory-manager/internal/datasheet"
	"inventory-manager/internal/opname"
	"inventory-manager/internal/order"
	"inventory-manager/internal/product"
	"inventory-manager/internal/purchase"
	"inventory-manager/internal/report"
	"inventory-manager/internal/salesperson"
	"inventory-manager/internal/store"
	"inventory-manager/internal/supplier"
	"inventory-manager/internal/supplierbill"
	"inventory-manager/internal/transfer"
)

type Services struct {
	Auth         auth.Service
	AuthRepo     auth.Repository
	Category     category.Service
	Supplier     supplier.Service
	Salesperson  salesperson.Service
	Product      product.Service
	Order        order.Service
	Transfer     transfer.Service
	Purchase     purchase.Service
	Opname       opname.Service
	Datasheet    datasheet.Service
	Report       report.Service
	SupplierBill supplierbill.Service
	StoreRepo    store.Repository
}

// New creates a fully configured Gin engine (used when running standalone).
func New(db *sqlx.DB, logger *slog.Logger, corsOrigin string, svc Services) *gin.Engine {
	r := gin.New()
	r.UseRawPath = true
	r.UnescapePathValues = true
	r.Use(RequestID())
	r.Use(Logger(logger))
	r.Use(gin.Recovery())
	r.Use(cors(corsOrigin))
	Register(r, db, svc)
	return r
}

// Register mounts all API routes onto an existing engine.
// Call this when the engine is created externally (e.g. with web UI routes already registered).
func Register(r *gin.Engine, db *sqlx.DB, svc Services) {

	limiter := newLoginRateLimiter()

	// Public
	r.GET("/healthz", healthz(db))
	r.POST("/auth/login", loginHandler(svc.Auth, limiter))
	r.POST("/auth/logout", logoutHandler())

	// Protected
	protected := r.Group("/")
	protected.Use(AuthRequired(svc.Auth, svc.AuthRepo))
	{
		protected.GET("/auth/me", meHandler(svc.Auth))
		protected.GET("/stores", listStoresHandler(db))

		registerUserRoutes(protected, svc.Auth)
		registerCategoryRoutes(protected, svc.Category)
		registerSupplierRoutes(protected, svc.Supplier)
		registerSalespersonRoutes(protected, svc.Salesperson)
		registerProductRoutes(protected, svc.Product)
		registerOrderRoutes(protected, svc.Order)
		registerTransferRoutes(protected, svc.Transfer)
		registerPurchaseRoutes(protected, svc.Purchase)
		registerOpnameRoutes(protected, svc.Opname)
		registerDatasheetRoutes(protected, svc.Datasheet)
		registerReportRoutes(protected, svc.Report, svc.Order)
		registerCompatRoutes(protected, svc.Order, svc.Product, svc.Supplier, svc.SupplierBill, svc.Transfer, svc.StoreRepo, svc.Category, svc.Report, db)
	}
}

func cors(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID")
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func respondError(c *gin.Context, err error) {
	c.JSON(apperr.HTTPCode(err), gin.H{"error": apperr.HTTPMessage(err)})
}
