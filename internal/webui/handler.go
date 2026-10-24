package webui

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/order"
	"inventory-manager/internal/product"
	"inventory-manager/internal/report"
	"inventory-manager/internal/salesperson"
	"inventory-manager/internal/store"
	"inventory-manager/internal/supplierbill"
	"inventory-manager/internal/transfer"
)

// Services bundles the backend services the web UI needs.
type Services struct {
	Auth         auth.Service
	AuthRepo     auth.Repository
	Product      product.Service
	Order        order.Service
	Salesperson  salesperson.Service
	Transfer     transfer.Service
	SupplierBill supplierbill.Service
	Report       report.Service
	Store        store.Repository
}

// handler holds parsed templates and services.
type handler struct {
	tmpl  *template.Template
	svc   Services
	build string
}

// RegisterRoutes registers all webui routes on the gin engine.
// Call this BEFORE httpapi.New so that routes don't conflict.
func RegisterRoutes(r *gin.Engine, svc Services, build string) {
	tmpl := template.Must(
		template.New("root").Funcs(TemplateFuncs).ParseGlob("web/templates/*.html"),
	)

	h := &handler{tmpl: tmpl, svc: svc, build: build}

	// Static assets.
	r.Static("/static", "web/static")

	// Login (no auth required).
	r.GET("/login", h.loginPage)

	// Root redirect.
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/dashboard")
	})

	// Authenticated pages.
	r.GET("/dashboard", h.authPage(h.dashboardPage))
	r.GET("/kasir", h.authPage(h.kasirPage))
	r.GET("/penjualan", h.authPage(h.penjualanPage))
	r.GET("/penjualan-void", h.authPage(h.voidOrdersPage))
	r.GET("/stok-global", h.authPage(h.globalStockPage))
	r.GET("/daftar-item", h.authPage(h.itemsPage))
	r.GET("/daftar-supplier", h.authPage(h.suppliersPage))
	r.GET("/supplier/detail", h.authPage(h.supplierDetailPage))
	r.GET("/daftar-sales", h.authPage(h.salespersonPage))
	r.GET("/stock-opname", h.authPage(h.opnamePage))
	r.GET("/barang-masuk", h.authPage(h.stockInPage))
	r.GET("/barang-keluar", h.authPage(h.stockOutPage))
	r.GET("/datasheet", h.authPage(h.datasheetPage))
	r.GET("/laporan/komisi-sales", h.authPage(h.komisiSalesPage))
	r.GET("/pengaturan/daftarkan-user", h.authPage(h.adminUsersPage))
}

// ── Auth helpers ──────────────────────────────────────────────────────────────

type pageFunc func(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData)

// authPage wraps a page handler with cookie-based auth check.
func (h *handler) authPage(fn pageFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("auth_token")
		if err != nil || token == "" {
			h.redirectLogin(c)
			return
		}
		actor, err := h.svc.Auth.ValidateToken(token)
		if err != nil {
			h.redirectLogin(c)
			return
		}
		user, err := h.svc.AuthRepo.GetByID(c.Request.Context(), actor.UserID)
		if err != nil {
			h.redirectLogin(c)
			return
		}

		pd := h.basePD(c.Request.Context(), actor, user)
		fn(c, *actor, user, pd)
	}
}

func (h *handler) redirectLogin(c *gin.Context) {
	next := url.QueryEscape(c.Request.URL.RequestURI())
	c.Redirect(http.StatusFound, "/login?next="+next)
}

// basePD builds the common page data fields.
func (h *handler) basePD(ctx context.Context, actor *auth.Actor, user *auth.User) PageData {
	deptLabel := "Admin"
	if actor.IsCashier() && actor.StoreID != nil {
		if s, err := h.svc.Store.GetByID(ctx, *actor.StoreID); err == nil {
			deptLabel = s.Name
		}
	}

	now := time.Now()
	return PageData{
		Company:     "Toko Tekstil",
		DeptLabel:   deptLabel,
		Cashier:     user.FullName,
		CashierRole: user.Role,
		Build:       h.build,
		Now:         now.Format("02/01/2006 15:04"),
	}
}

// render executes the named layout template.
func (h *handler) render(c *gin.Context, pd PageData) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")
	if err := h.tmpl.ExecuteTemplate(c.Writer, "layout", pd); err != nil {
		_ = c.Error(err)
		c.String(http.StatusInternalServerError, "template error: %v", err)
	}
}

// ── Pages ─────────────────────────────────────────────────────────────────────

func (h *handler) loginPage(c *gin.Context) {
	pd := PageData{
		Page:    "login",
		Title:   "Masuk",
		Company: "Toko Tekstil",
		Build:   h.build,
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	// Render the login template directly — it is a standalone HTML page,
	// not a content block inside the main layout.
	if err := h.tmpl.ExecuteTemplate(c.Writer, "login", pd); err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
	}
}

func (h *handler) dashboardPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "dashboard"
	pd.Title = "Dashboard"

	var storeID uint
	if actor.StoreID != nil {
		storeID = *actor.StoreID
	}
	if stats, err := h.svc.Report.Dashboard(c.Request.Context(), actor, storeID); err == nil {
		pd.Revenue = stats.OmzetToday.InexactFloat64()
		pd.TrxCount = stats.TxCountToday

		var yday float64
		if !stats.OmzetYesterday.IsZero() {
			yday = stats.OmzetYesterday.InexactFloat64()
		}
		pd.Today = DayStat{
			Key:   "today",
			Label: "Hari Ini",
			Total: pd.Revenue,
			Count: pd.TrxCount,
		}
		pd.Yesterday = DayStat{
			Key:   "yesterday",
			Label: "Kemarin",
			Total: yday,
		}
		if yday > 0 {
			pct := (pd.Revenue - yday) / yday * 100
			pd.ChangePercent = pct
			if pct >= 0 {
				pd.ChangeClass = "positive"
				pd.ChangeText = fmt.Sprintf("+%.1f%%", pct)
			} else {
				pd.ChangeClass = "negative"
				pd.ChangeText = fmt.Sprintf("%.1f%%", pct)
			}
		}

		for _, d := range stats.Chart7Days {
			pd.Days = append(pd.Days, DayStat{
				Key:   d.Date,
				Label: d.Date,
				Total: d.Omzet.InexactFloat64(),
			})
		}

		var topMax float64
		for _, tp := range stats.TopProducts {
			rev := tp.Revenue.InexactFloat64()
			if rev > topMax {
				topMax = rev
			}
		}
		pd.TopMax = topMax
		pd.TopEmpty = len(stats.TopProducts) == 0
		for _, tp := range stats.TopProducts {
			rev := tp.Revenue.InexactFloat64()
			pct := 0.0
			if topMax > 0 {
				pct = rev / topMax * 100
			}
			pd.TopItems = append(pd.TopItems, Agg{
				Name:    tp.ProductName,
				SKU:     tp.ProductCode,
				Total:   rev,
				Pct:     pct,
				PctText: fmt.Sprintf("%.0f%%", pct),
			})
		}

		// Recent sales from dashboard.
		for _, ro := range stats.RecentOrders {
			pd.RecentSales = append(pd.RecentSales, UISale{
				No:    ro.OrderNo,
				Date:  ro.CreatedAt,
				Dept:  ro.StoreName,
				Total: ro.Total.InexactFloat64(),
			})
		}
	}

	h.render(c, pd)
}

func (h *handler) kasirPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "kasir"
	pd.Title = "Kasir"
	h.loadProducts(c, actor, &pd)
	h.render(c, pd)
}

func (h *handler) penjualanPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "penjualan"
	pd.Title = "Penjualan"
	h.render(c, pd)
}

func (h *handler) globalStockPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "stok-global"
	pd.Title = "Stok Global"
	h.render(c, pd)
}

func (h *handler) voidOrdersPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "void-orders"
	pd.Title = "Transaksi Dibatalkan"
	h.render(c, pd)
}

func (h *handler) itemsPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "items"
	pd.Title = "Daftar Item"
	h.loadProducts(c, actor, &pd)
	h.render(c, pd)
}

func (h *handler) suppliersPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "suppliers"
	pd.Title = "Daftar Supplier"
	h.loadSupplierBills(c, actor, &pd)
	h.render(c, pd)
}

func (h *handler) supplierDetailPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "supplier-detail"
	nama := c.Query("nama")
	pd.Title = "Supplier: " + nama
	pd.SupplierName = nama
	h.loadSupplierBills(c, actor, &pd)

	// Reset all counters — loadSupplierBills totals every supplier.
	// Recompute from only this supplier's bills.
	pd.SupplierBillCount = 0
	pd.SupplierPaidCount = 0
	pd.SupplierPaidTotal = 0
	pd.SupplierUnpaidCount = 0
	pd.SupplierUnpaidTotal = 0
	pd.SupplierOverdueCount = 0
	pd.SupplierOverdueTotal = 0

	today := time.Now().Format("2006-01-02")
	filtered := make([]UISupplierBill, 0, len(pd.SupplierBills))
	for _, b := range pd.SupplierBills {
		if !strings.EqualFold(b.Supplier, nama) {
			continue
		}
		filtered = append(filtered, b)
		pd.SupplierBillCount++
		if b.Status == "Lunas" {
			pd.SupplierPaidCount++
			pd.SupplierPaidTotal += b.Amount
		} else {
			pd.SupplierUnpaidCount++
			pd.SupplierUnpaidTotal += b.Amount
			if b.DueDate != "" && b.DueDate < today {
				pd.SupplierOverdueCount++
				pd.SupplierOverdueTotal += b.Amount
			}
		}
	}
	pd.SupplierBills = filtered
	h.render(c, pd)
}

func (h *handler) salespersonPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "salespeople"
	pd.Title = "Daftar Sales"
	h.loadSalespeople(c, &pd)
	h.render(c, pd)
}

func (h *handler) opnamePage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "opname"
	pd.Title = "Stock Opname"
	h.loadProducts(c, actor, &pd)
	h.render(c, pd)
}

func (h *handler) stockInPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "stock-in"
	pd.Title = "Barang Masuk"
	pd.TransferType = "in"
	h.loadTransfers(c, actor, "incoming", &pd)
	h.render(c, pd)
}

func (h *handler) stockOutPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "stock-out"
	pd.Title = "Barang Keluar"
	pd.TransferType = "out"
	h.loadTransfers(c, actor, "outgoing", &pd)
	h.render(c, pd)
}

func (h *handler) datasheetPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "datasheet"
	pd.Title = "Datasheet"
	h.loadProducts(c, actor, &pd)
	h.render(c, pd)
}

func (h *handler) komisiSalesPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "komisi-sales"
	pd.Title = "Komisi Sales"
	h.render(c, pd)
}

func (h *handler) adminUsersPage(c *gin.Context, actor auth.Actor, user *auth.User, pd PageData) {
	pd.Page = "admin-users"
	pd.Title = "Daftarkan User"
	// Admin-only guard.
	if !actor.IsAdmin() {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}
	h.render(c, pd)
}

// ── Data loaders ──────────────────────────────────────────────────────────────

func (h *handler) loadProducts(c *gin.Context, actor auth.Actor, pd *PageData) {
	var storeID uint
	if actor.StoreID != nil {
		storeID = *actor.StoreID
	}

	result, err := h.svc.Product.List(c.Request.Context(), actor, product.ListFilter{
		Limit:   100,
		StoreID: storeID,
	})
	if err != nil {
		return
	}

	catSet := map[string]struct{}{}

	convertProduct := func(p *product.CashierResponse, costPrice float64) UIProduct {
		cat := ""
		if p.CategoryName != nil {
			cat = *p.CategoryName
		}
		if cat != "" {
			catSet[cat] = struct{}{}
		}

		var stockQty float64
		for _, s := range p.Stock {
			if storeID == 0 || s.StoreID == storeID {
				stockQty = s.Qty.InexactFloat64()
				break
			}
		}
		if storeID == 0 && len(p.Stock) > 0 {
			// For admin with no store filter, sum all stock.
			stockQty = 0
			for _, s := range p.Stock {
				stockQty += s.Qty.InexactFloat64()
			}
		}

		var storeID uint
		var storeName string
		if p.StoreID != nil {
			storeID = *p.StoreID
		}
		if p.StoreName != nil {
			storeName = *p.StoreName
		}

		return UIProduct{
			ID:        p.ID,
			SKU:       p.Code,
			Name:      p.Name,
			Category:  cat,
			StoreID:   storeID,
			StoreName: storeName,
			Unit:      p.Unit,
			Price:     p.SellPrice.InexactFloat64(),
			Cost:      costPrice,
			Stock:     stockQty,
			MinStock:  p.MinStock.InexactFloat64(),
		}
	}

	if actor.IsAdmin() {
		if data, ok := result.Data.([]*product.AdminResponse); ok {
			for _, p := range data {
				uip := convertProduct(&p.CashierResponse, p.CostPrice.InexactFloat64())
				pd.Products = append(pd.Products, uip)
				if uip.Low() {
					pd.LowStock = append(pd.LowStock, uip)
				}
			}
		}
	} else {
		if data, ok := result.Data.([]*product.CashierResponse); ok {
			for _, p := range data {
				uip := convertProduct(p, 0)
				pd.Products = append(pd.Products, uip)
				if uip.Low() {
					pd.LowStock = append(pd.LowStock, uip)
				}
			}
		}
	}

	for cat := range catSet {
		pd.Categories = append(pd.Categories, cat)
	}

	// Compute derived stats.
	for _, p := range pd.Products {
		pd.StockValue += p.Stock * p.Cost
	}
}

func (h *handler) loadSalespeople(c *gin.Context, pd *PageData) {
	sps, err := h.svc.Salesperson.List(c.Request.Context())
	if err != nil {
		return
	}
	for _, sp := range sps {
		if sp.IsActive {
			pd.SalesPeople = append(pd.SalesPeople, sp.Name)
		}
	}
}

func (h *handler) loadOrders(c *gin.Context, actor auth.Actor, pd *PageData) {
	var storeID uint
	if actor.StoreID != nil {
		storeID = *actor.StoreID
	}

	wib := time.FixedZone("WIB", 7*3600)
	now := time.Now().In(wib)
	// Load last 90 days so date-range filter in the UI has useful data to work with.
	rangeStart := time.Date(now.Year(), now.Month(), now.Day()-89, 0, 0, 0, 0, wib).UTC()
	rangeEnd := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, wib).UTC()

	result, err := h.svc.Order.ListOrders(c.Request.Context(), actor, order.ListFilter{
		StoreID:  storeID,
		DateFrom: rangeStart,
		DateTo:   rangeEnd,
		Page:     1,
		Limit:    500,
	})
	if err != nil {
		return
	}

	for _, o := range result.Data {
		sale := UISale{
			No:              o.OrderNo,
			Date:            o.CreatedAt,
			Dept:            o.StoreName,
			Cashier:         o.CashierName,
			Subtotal:        o.Subtotal.InexactFloat64(),
			DiscountPercent: o.DiscountPct.InexactFloat64(),
			DiscountAmount:  o.DiscountAmount.InexactFloat64(),
			Total:           o.Total.InexactFloat64(),
			Paid:            o.PaidAmount.InexactFloat64(),
		}
		if o.CustomerName != nil {
			sale.Customer = *o.CustomerName
		}
		if o.SalesName != nil {
			sale.Sales = *o.SalesName
		}
		if o.Note != nil {
			sale.Note = *o.Note
		}

		// Load items for the detail panel.
		if full, err := h.svc.Order.GetOrder(c.Request.Context(), actor, o.ID); err == nil {
			for _, it := range full.Items {
				sale.Items = append(sale.Items, UISaleItem{
					SKU:   it.ProductCode,
					Name:  it.ProductName,
					Qty:   it.Qty.InexactFloat64(),
					Unit:  it.Unit,
					Price: it.UnitPrice.InexactFloat64(),
				})
			}
		}

		pd.Sales = append(pd.Sales, sale)
	}
}

func (h *handler) loadSalesAndProducts(c *gin.Context, actor auth.Actor, pd *PageData) {
	h.loadProducts(c, actor, pd)
	h.loadOrders(c, actor, pd)
}

func (h *handler) loadTransfers(c *gin.Context, actor auth.Actor, direction string, pd *PageData) {
	result, err := h.svc.Transfer.ListTransfers(c.Request.Context(), actor, transfer.ListFilter{
		Direction: direction,
		Page:      1,
		Limit:     100,
	})
	if err != nil {
		return
	}

	for _, t := range result.Data {
		transferType := "out"
		if direction == "incoming" {
			transferType = "in"
		}
		uit := UITransfer{
			No:      t.TransferNo,
			Date:    t.SentAt,
			Type:    transferType,
			From:    t.FromStoreName,
			To:      t.ToStoreName,
			Cashier: t.SentByName,
		}
		if t.Note != nil {
			uit.Note = *t.Note
		}
		pd.Transfers = append(pd.Transfers, uit)
	}
}

func (h *handler) loadSupplierBills(c *gin.Context, actor auth.Actor, pd *PageData) {
	bills, err := h.svc.SupplierBill.List(c.Request.Context())
	if err != nil {
		return
	}

	today := time.Now().Format("2006-01-02")

	for _, b := range bills {
		uib := UISupplierBill{
			ID:           fmt.Sprintf("%d", b.ID),
			Supplier:     b.Supplier,
			InvoiceNo:    b.InvoiceNo,
			InvoiceDate:  b.InvoiceDate,  // already "YYYY-MM-DD" from toResponse
			ReceivedDate: b.ReceivedDate,
			Amount:       b.Amount,
			DueDate:      b.DueDate,
			Status:       b.Status,
		}
		pd.SupplierBills = append(pd.SupplierBills, uib)
		pd.SupplierBillCount++
		if b.Status == "Lunas" {
			pd.SupplierPaidCount++
			pd.SupplierPaidTotal += b.Amount
		} else {
			pd.SupplierUnpaidCount++
			pd.SupplierUnpaidTotal += b.Amount
			if b.DueDate != "" && b.DueDate < today {
				pd.SupplierOverdueCount++
				pd.SupplierOverdueTotal += b.Amount
			}
		}
	}
}

// Compile-time interface check.
var _ salesperson.Service = (salesperson.Service)(nil)
