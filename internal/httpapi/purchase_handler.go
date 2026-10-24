package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/purchase"
)

func registerPurchaseRoutes(rg *gin.RouterGroup, svc purchase.Service) {
	r := rg.Group("/purchases")
	r.Use(RequireAdmin())
	r.POST("", createInvoiceHandler(svc))
	r.GET("", listInvoicesHandler(svc))
	r.GET("/:id", getInvoiceHandler(svc))
	r.POST("/:id/mark-received", markReceivedHandler(svc))
	r.PATCH("/:id/payment", updatePaymentHandler(svc))
}

func createInvoiceHandler(svc purchase.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input purchase.CreateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := svc.CreateInvoice(c.Request.Context(), GetActor(c), input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	}
}

func listInvoicesHandler(svc purchase.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		f := purchase.ListFilter{
			Page:  atoi(c.DefaultQuery("page", "1")),
			Limit: atoi(c.DefaultQuery("limit", "20")),
		}
		if v := c.Query("supplier_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			f.SupplierID = uint(id)
		}
		if v := c.Query("store_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			f.StoreID = uint(id)
		}
		if v := c.Query("status"); v != "" {
			f.Status = v
		}
		if v := c.Query("due_before"); v != "" {
			if d, err := time.Parse("2006-01-02", v); err == nil {
				f.DueBefore = &d
			}
		}
		result, err := svc.ListInvoices(c.Request.Context(), f)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func getInvoiceHandler(svc purchase.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		resp, err := svc.GetInvoice(c.Request.Context(), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// markReceivedHandler records the goods arrival date — no stock movement.
func markReceivedHandler(svc purchase.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var body struct {
			Date *string `json:"date"` // YYYY-MM-DD, defaults to today
		}
		_ = c.ShouldBindJSON(&body) // optional body
		resp, err := svc.MarkReceived(c.Request.Context(), GetActor(c), id, body.Date)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func updatePaymentHandler(svc purchase.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input purchase.UpdatePaymentInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := svc.UpdatePayment(c.Request.Context(), GetActor(c), id, input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}
