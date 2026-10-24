package httpapi

// Compatibility routes at /api/* that match the shapes used by web/static/app.js.

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/category"
	"inventory-manager/internal/order"
	"inventory-manager/internal/product"
	"inventory-manager/internal/report"
	"inventory-manager/internal/stock"
	"inventory-manager/internal/store"
	"inventory-manager/internal/supplier"
	"inventory-manager/internal/supplierbill"
	"inventory-manager/internal/transfer"
)

func registerCompatRoutes(
	rg *gin.RouterGroup,
	orderSvc order.Service,
	productSvc product.Service,
	supplierSvc supplier.Service,
	billSvc supplierbill.Service,
	transferSvc transfer.Service,
	storeRepo store.Repository,
	categorySvc category.Service,
	reportSvc report.Service,
	db *sqlx.DB,
) {
	api := rg.Group("/api")

	// ── Sales (orders) ────────────────────────────────────────────────────────
	api.POST("/sales", compatCreateSale(orderSvc))
	api.GET("/sales", compatListSales(orderSvc))
	api.GET("/sales/:no", compatGetSale(orderSvc))
	api.PATCH("/sales/:no", compatUpdateSale(orderSvc))
	api.DELETE("/sales/:no", compatDeleteSale(orderSvc))
	api.POST("/sales/:no/continue", compatContinueSale(orderSvc))

	// ── Items (products) ──────────────────────────────────────────────────────
	api.GET("/items", compatListItems(productSvc))
	api.POST("/items", compatSaveItem(productSvc, categorySvc, storeRepo, stock.NewWriter(db)))
	api.DELETE("/items/:sku", compatDeleteItem(productSvc))

	// ── Suppliers ─────────────────────────────────────────────────────────────
	api.GET("/suppliers", compatListSupplierNames(supplierSvc))
	api.POST("/suppliers", compatAddSupplier(supplierSvc))

	// ── Supplier bills (nota supplier) ────────────────────────────────────────
	api.GET("/supplier-bills", compatListBills(billSvc))
	api.POST("/supplier-bills", compatCreateBill(billSvc))
	api.PATCH("/supplier-bills/:id", compatUpdateBill(billSvc))
	api.DELETE("/supplier-bills/:id", compatDeleteBill(billSvc))

	// ── Stock transfers ───────────────────────────────────────────────────────
	api.POST("/stock-transfers", compatCreateTransfer(transferSvc, storeRepo))

	// ── Commission report ─────────────────────────────────────────────────────
	api.GET("/commission", compatCommissionReport(reportSvc))
}

// ── Sales ─────────────────────────────────────────────────────────────────────

// compatCreateSale maps the frontend's sale payload → CreateOrderInput.
// Frontend sends: {customer, sales, payment, paid, note, discountPercent,
//                  discountAmount, items:[{product_id, qty}]}
func compatCreateSale(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Customer        string  `json:"customer"`
			Sales           string  `json:"sales"`
			SalesID         *uint   `json:"sales_id"`
			StoreID         *uint   `json:"store_id"`
			Payment         string  `json:"payment"`
			Paid            float64 `json:"paid"`
			Note            string  `json:"note"`
			DiscountPercent float64 `json:"discountPercent"`
			DiscountAmount  float64 `json:"discountAmount"`
			Items           []struct {
				ProductID uint    `json:"product_id"`
				Qty       float64 `json:"qty"`
			} `json:"items"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(body.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "keranjang masih kosong"})
			return
		}

		items := make([]order.OrderItemInput, len(body.Items))
		for i, it := range body.Items {
			items[i] = order.OrderItemInput{
				ProductID: it.ProductID,
				Qty:       decimal.NewFromFloat(it.Qty),
			}
		}

		input := order.CreateOrderInput{
			StoreID:      body.StoreID,
			SalesID:      body.SalesID,
			CustomerName: nullableString(body.Customer),
			Note:         nullableString(body.Note),
			Items:        items,
			DiscountPct:  decimal.NewFromFloat(body.DiscountPercent),
			DiscountRp:   decimal.NewFromFloat(body.DiscountAmount),
		}

		actor := GetActor(c)
		resp, err := svc.CreateOrder(c.Request.Context(), actor, input)
		if err != nil {
			respondError(c, err)
			return
		}

		// Add payment immediately if provided.
		if body.Paid > 0 && body.Payment != "" {
			method := normalizePaymentMethod(body.Payment)
			paid := decimal.NewFromFloat(body.Paid)
			resp, err = svc.AddPayment(c.Request.Context(), actor, resp.ID, order.AddPaymentInput{
				Method: method,
				Amount: paid,
			})
			if err != nil {
				respondError(c, err)
				return
			}
		}

		paidGiven := decimal.NewFromFloat(body.Paid)
		change := paidGiven.Sub(resp.Total)
		if change.IsNegative() {
			change = decimal.Zero
		}
		c.JSON(http.StatusOK, gin.H{
			"no":              resp.OrderNo,
			"subtotal":        resp.Subtotal,
			"discountPercent": resp.DiscountPct,
			"discountAmount":  resp.DiscountAmount,
			"total":           resp.Total,
			"paid":            resp.PaidAmount,
			"balance":         resp.Total.Sub(resp.PaidAmount),
			"pending":         resp.Status != "paid",
			"change":          change,
		})
	}
}

func compatListSales(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)
		f := order.ListFilter{
			Search: c.Query("q"),
			Status: c.Query("status"),
			Page:   atoi(c.DefaultQuery("page", "1")),
			Limit:  atoi(c.DefaultQuery("limit", "500")),
		}
		if sid := atoi(c.Query("store_id")); sid > 0 {
			f.StoreID = uint(sid)
		}
		if v := c.Query("date_from"); v != "" {
			if start, _, err := order.ParseWIBDate(v); err == nil {
				f.DateFrom = start
			}
		}
		if v := c.Query("date_to"); v != "" {
			if _, end, err := order.ParseWIBDate(v); err == nil {
				f.DateTo = end
			}
		}
		if f.DateFrom.IsZero() && f.DateTo.IsZero() {
			wib := time.FixedZone("WIB", 7*3600)
			today := time.Now().In(wib).Format("2006-01-02")
			f.DateFrom, f.DateTo, _ = order.ParseWIBDate(today)
		}
		result, err := svc.ListOrders(c.Request.Context(), actor, f)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func compatGetSale(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		no := c.Param("no")
		resp, err := svc.GetByOrderNo(c.Request.Context(), GetActor(c), no)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func compatUpdateSale(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		no := c.Param("no")
		existing, err := svc.GetByOrderNo(c.Request.Context(), GetActor(c), no)
		if err != nil {
			respondError(c, err)
			return
		}
		var body struct {
			Customer string `json:"customer"`
			SalesID  *uint  `json:"sales_id"`
			Note     string `json:"note"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		input := order.UpdateMetaInput{
			CustomerName: &body.Customer,
			SalesID:      body.SalesID,
			Note:         &body.Note,
		}
		resp, err := svc.UpdateMeta(c.Request.Context(), GetActor(c), existing.ID, input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func compatDeleteSale(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		no := c.Param("no")
		existing, err := svc.GetByOrderNo(c.Request.Context(), GetActor(c), no)
		if err != nil {
			respondError(c, err)
			return
		}
		if err := svc.VoidOrder(c.Request.Context(), GetActor(c), existing.ID, order.VoidInput{
			Reason: "voided from kasir",
		}); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

// compatContinueSale maps the frontend's continue-bill payload.
// Frontend sends: {customer, sales, note, method, amount, items:[{product_id,qty}]}
func compatContinueSale(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		no := c.Param("no")
		existing, err := svc.GetByOrderNo(c.Request.Context(), GetActor(c), no)
		if err != nil {
			respondError(c, err)
			return
		}
		var body struct {
			Customer string  `json:"customer"`
			Sales    string  `json:"sales"`
			Note     string  `json:"note"`
			Method   string  `json:"method"`
			Amount   float64 `json:"amount"`
			Items    []struct {
				ProductID uint    `json:"product_id"`
				Qty       float64 `json:"qty"`
			} `json:"items"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		input := order.ContinueInput{
			CustomerName: nullableString(body.Customer),
			Note:         nullableString(body.Note),
		}

		for _, it := range body.Items {
			input.Items = append(input.Items, order.OrderItemInput{
				ProductID: it.ProductID,
				Qty:       decimal.NewFromFloat(it.Qty),
			})
		}

		if body.Amount > 0 && body.Method != "" {
			input.Payment = &order.AddPaymentInput{
				Method: normalizePaymentMethod(body.Method),
				Amount: decimal.NewFromFloat(body.Amount),
			}
		}

		resp, err := svc.ContinueOrder(c.Request.Context(), GetActor(c), existing.ID, input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"no":      resp.OrderNo,
			"total":   resp.Total,
			"paid":    resp.PaidAmount,
			"balance": resp.Total.Sub(resp.PaidAmount),
			"pending": resp.Status != "paid",
		})
	}
}

// ── Items (products) ──────────────────────────────────────────────────────────

func compatListItems(svc product.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)
		f := product.ListFilter{
			Search:  c.Query("search"),
			Page:    atoi(c.DefaultQuery("page", "1")),
			Limit:   atoi(c.DefaultQuery("limit", "100")),
			StoreID: uint(atoi(c.Query("store_id"))),
		}
		result, err := svc.List(c.Request.Context(), actor, f)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// compatSaveItem maps the frontend's item save → product upsert by code.
// compatSaveItem maps the frontend's item save → product upsert by code.
// Frontend sends: {name, sku, category, unit, price, cost, stock}
// Category is a free-text name — resolved to ID via find-or-create.
// Stock is seeded on new product creation only.
func compatSaveItem(svc product.Service, categorySvc category.Service, storeRepo store.Repository, stockWriter *stock.Writer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name     string  `json:"name"`
			SKU      string  `json:"sku"`
			Category string  `json:"category"`
			Unit     string  `json:"unit"`
			Price    float64 `json:"price"`
			Cost     float64 `json:"cost"`
			Stock    float64 `json:"stock"`
			StoreID  *uint   `json:"store_id"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if body.SKU == "" || body.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name and sku are required"})
			return
		}
		ctx := c.Request.Context()
		actor := GetActor(c)

		unit := body.Unit
		if unit != "pcs" && unit != "meter" {
			unit = "meter"
		}

		// Resolve category name → ID (find existing or create new).
		var categoryID *uint
		if name := strings.TrimSpace(body.Category); name != "" {
			catID := findOrCreateCategory(ctx, actor, categorySvc, name)
			if catID > 0 {
				categoryID = &catID
			}
		}

		// Detect whether this is a new product before upsert.
		_, err := svc.GetByCode(ctx, body.SKU)
		isNew := err != nil // NotFound error means it's new

		// Apply min_stock default of 20 for new products only.
		var minStock decimal.Decimal
		if isNew {
			minStock = decimal.NewFromInt(20)
		}

		input := product.CreateInput{
			Code:       strings.ToUpper(strings.TrimSpace(body.SKU)),
			Name:       strings.TrimSpace(body.Name),
			CategoryID: categoryID,
			StoreID:    body.StoreID,
			Unit:       unit,
			SellPrice:  decimal.NewFromFloat(body.Price),
			CostPrice:  decimal.NewFromFloat(body.Cost),
			MinStock:   minStock,
		}
		resp, err := svc.UpsertByCode(ctx, actor, input)
		if err != nil {
			respondError(c, err)
			return
		}

		// Seed initial stock for new products when stock > 0.
		if isNew && body.Stock > 0 {
			seedInitialStock(ctx, actor, resp, body.Stock, storeRepo, stockWriter)
			// Re-fetch so the response includes the seeded stock.
			var productID uint
			switch v := resp.(type) {
			case *product.AdminResponse:
				productID = v.ID
			case *product.CashierResponse:
				productID = v.ID
			}
			if productID > 0 {
				if fresh, err := svc.GetByID(ctx, actor, productID); err == nil {
					resp = fresh
				}
			}
		}

		c.JSON(http.StatusOK, resp)
	}
}

// findOrCreateCategory returns the ID of a category matching name, creating it if absent.
func findOrCreateCategory(ctx context.Context, actor auth.Actor, svc category.Service, name string) uint {
	cats, err := svc.List(ctx)
	if err != nil {
		return 0
	}
	nameLower := strings.ToLower(name)
	for _, c := range cats {
		if strings.ToLower(c.Name) == nameLower {
			return c.ID
		}
	}
	// Not found — create it.
	created, err := svc.Create(ctx, actor, category.CreateInput{Name: name})
	if err != nil {
		return 0
	}
	return created.ID
}

// seedInitialStock writes an adjustment movement for a newly created product.
// For admin actors: uses the first active store.
// For cashier actors: uses their own store.
func seedInitialStock(ctx context.Context, actor auth.Actor, resp interface{}, qty float64, storeRepo store.Repository, stockWriter *stock.Writer) {
	// Extract numeric product ID from the upsert response.
	type hasID interface{ GetID() uint }
	var productID uint
	switch v := resp.(type) {
	case *product.AdminResponse:
		productID = v.ID
	case *product.CashierResponse:
		productID = v.ID
	}
	if productID == 0 {
		return
	}

	var storeID uint
	if actor.StoreID != nil {
		storeID = *actor.StoreID
	} else {
		// Admin: seed the first active store.
		stores, err := storeRepo.List(ctx)
		if err != nil || len(stores) == 0 {
			return
		}
		storeID = stores[0].ID
	}

	_ = stockWriter.WithRetry(ctx, func(tx *sqlx.Tx) error {
		return stockWriter.Increment(
			ctx, tx, storeID, productID,
			decimal.NewFromFloat(qty),
			stock.ReasonAdjustment, stock.RefType("manual"), 0,
			"initial stock", actor.UserID,
		)
	})
}

func compatDeleteItem(svc product.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		sku := c.Param("sku")
		if err := svc.DeactivateByCode(c.Request.Context(), GetActor(c), sku); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

// ── Suppliers ─────────────────────────────────────────────────────────────────

// compatListSupplierNames returns a plain []string of supplier names.
// The frontend (supplierBillsPage) iterates over this array expecting strings, not objects.
func compatListSupplierNames(svc supplier.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := svc.List(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		names := make([]string, len(list))
		for i, s := range list {
			names[i] = s.Name
		}
		c.JSON(http.StatusOK, names)
	}
}

// compatAddSupplier accepts {name} and creates a supplier.
// The supplier package's CreateInput only requires Name — no code field — so this is direct.
func compatAddSupplier(svc supplier.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s, err := svc.Create(c.Request.Context(), GetActor(c), supplier.CreateInput{Name: body.Name})
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": s.Name})
	}
}

// ── Supplier bills ────────────────────────────────────────────────────────────

func compatListBills(svc supplierbill.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		bills, err := svc.List(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, bills)
	}
}

func compatCreateBill(svc supplierbill.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input supplierbill.CreateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := svc.Create(c.Request.Context(), GetActor(c), input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func compatUpdateBill(svc supplierbill.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input supplierbill.UpdateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := svc.Update(c.Request.Context(), GetActor(c), id, input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func compatDeleteBill(svc supplierbill.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := svc.Delete(c.Request.Context(), GetActor(c), id); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

// ── Stock transfers ───────────────────────────────────────────────────────────

// compatCreateTransfer maps the frontend's transfer payload.
// Frontend sends: {type:"out"|"in", fromStore:"Toko A", toStore:"Toko B",
//                  note, items:[{product_id, qty}]}
// For "out": actor sends from their store (fromStore) to toStore.
// For "in":  actor is receiving — this creates a transfer record from fromStore
//            to toStore (the actor's store must be toStore).
func compatCreateTransfer(svc transfer.Service, storeRepo store.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Type      string  `json:"type"`
			FromStore string  `json:"fromStore"`
			ToStore   string  `json:"toStore"`
			Note      string  `json:"note"`
			Items     []struct {
				ProductID uint    `json:"product_id"`
				Qty       float64 `json:"qty"`
			} `json:"items"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if body.FromStore == "" || body.ToStore == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "fromStore and toStore are required"})
			return
		}
		if len(body.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "minimal satu barang diperlukan"})
			return
		}

		ctx := c.Request.Context()
		toStore, err := storeRepo.GetByName(ctx, body.ToStore)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "toko tujuan tidak ditemukan: " + body.ToStore})
			return
		}

		fromStore, err := storeRepo.GetByName(ctx, body.FromStore)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "toko asal tidak ditemukan: " + body.FromStore})
			return
		}

		items := make([]transfer.ItemInput, len(body.Items))
		for i, it := range body.Items {
			items[i] = transfer.ItemInput{
				ProductID: it.ProductID,
				Qty:       decimal.NewFromFloat(it.Qty),
			}
		}

		input := transfer.CreateInput{
			FromStoreID: &fromStore.ID,
			ToStoreID:   toStore.ID,
			Note:        nullableString(body.Note),
			Items:       items,
		}

		actor := GetActor(c)
		resp, err := svc.CreateTransfer(ctx, actor, input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// ── Commission Report ─────────────────────────────────────────────────────────

func compatCommissionReport(svc report.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)
		wib := time.FixedZone("WIB", 7*3600)
		f := report.CommissionFilter{}
		if v := c.Query("date_from"); v != "" {
			if t, err := time.ParseInLocation("2006-01-02", v, wib); err == nil {
				f.DateFrom = t
			}
		}
		if v := c.Query("date_to"); v != "" {
			if t, err := time.ParseInLocation("2006-01-02", v, wib); err == nil {
				f.DateTo = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			}
		}
		if sid := atoi(c.Query("salesperson_id")); sid > 0 {
			f.SalespersonID = uint(sid)
		}
		result, err := svc.CommissionFull(c.Request.Context(), actor, f)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func normalizePaymentMethod(m string) string {
	switch strings.ToUpper(m) {
	case "QRIS", "TRANSFER":
		return strings.ToUpper(m)
	default:
		return "CASH"
	}
}

func nullableString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

