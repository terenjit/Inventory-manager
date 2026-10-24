package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/order"
	"inventory-manager/internal/report"
)

func registerReportRoutes(rg *gin.RouterGroup, svc report.Service, orderSvc order.Service) {
	rep := rg.Group("/reports")
	rep.GET("/dashboard", dashboardHandler(svc))
	rep.GET("/sales", salesReportHandler(svc))
	rep.GET("/commissions", RequireAdmin(), commissionReportHandler(svc))
	rep.GET("/stock/movements", stockReportHandler(svc))

	// Receipt lives under /orders
	rg.GET("/orders/:id/receipt", receiptHandler(svc))
}

func dashboardHandler(svc report.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var storeID uint
		if v := c.Query("store_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			storeID = uint(id)
		}
		stats, err := svc.Dashboard(c.Request.Context(), GetActor(c), storeID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}

func salesReportHandler(svc report.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		f := report.SalesFilter{
			Page:  atoi(c.DefaultQuery("page", "1")),
			Limit: atoi(c.DefaultQuery("limit", "20")),
		}
		if v := c.Query("store_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			f.StoreID = uint(id)
		}
		if v := c.Query("cashier_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			f.CashierID = uint(id)
		}
		if v := c.Query("sales_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			f.SalesID = uint(id)
		}
		if v := c.Query("status"); v != "" {
			f.Status = v
		}
		if v := c.Query("date_from"); v != "" {
			if d, err := time.ParseInLocation("2006-01-02", v, time.FixedZone("WIB", 7*3600)); err == nil {
				f.DateFrom = d
			}
		}
		if v := c.Query("date_to"); v != "" {
			if d, err := time.ParseInLocation("2006-01-02", v, time.FixedZone("WIB", 7*3600)); err == nil {
				f.DateTo = d.Add(24*time.Hour - time.Nanosecond)
			}
		}
		result, err := svc.SalesReport(c.Request.Context(), GetActor(c), f)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func commissionReportHandler(svc report.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		f := report.CommissionFilter{}
		if v := c.Query("salesperson_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			f.SalespersonID = uint(id)
		}
		if v := c.Query("date_from"); v != "" {
			if d, err := time.ParseInLocation("2006-01-02", v, time.FixedZone("WIB", 7*3600)); err == nil {
				f.DateFrom = d
			}
		}
		if v := c.Query("date_to"); v != "" {
			if d, err := time.ParseInLocation("2006-01-02", v, time.FixedZone("WIB", 7*3600)); err == nil {
				f.DateTo = d.Add(24*time.Hour - time.Nanosecond)
			}
		}
		rows, err := svc.CommissionReport(c.Request.Context(), GetActor(c), f)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	}
}

func stockReportHandler(svc report.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		f := report.StockFilter{
			Page:  atoi(c.DefaultQuery("page", "1")),
			Limit: atoi(c.DefaultQuery("limit", "50")),
		}
		if v := c.Query("product_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			f.ProductID = uint(id)
		}
		if v := c.Query("store_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			f.StoreID = uint(id)
		}
		if v := c.Query("date_from"); v != "" {
			if d, err := time.ParseInLocation("2006-01-02", v, time.FixedZone("WIB", 7*3600)); err == nil {
				f.DateFrom = d
			}
		}
		if v := c.Query("date_to"); v != "" {
			if d, err := time.ParseInLocation("2006-01-02", v, time.FixedZone("WIB", 7*3600)); err == nil {
				f.DateTo = d.Add(24*time.Hour - time.Nanosecond)
			}
		}
		result, err := svc.StockReport(c.Request.Context(), GetActor(c), f)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func receiptHandler(svc report.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		receipt, err := svc.GetReceipt(c.Request.Context(), GetActor(c), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, receipt)
	}
}
