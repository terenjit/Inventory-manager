package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/order"
)

func registerOrderRoutes(rg *gin.RouterGroup, svc order.Service) {
	r := rg.Group("/orders")
	r.POST("", createOrderHandler(svc))
	r.GET("", listOrdersHandler(svc))
	r.GET("/:id", getOrderHandler(svc))
	r.PATCH("/:id", updateOrderMetaHandler(svc))
	r.POST("/:id/payments", addPaymentHandler(svc))
	r.POST("/:id/void", voidOrderHandler(svc))
	r.POST("/:id/continue", continueOrderHandler(svc))
}

func createOrderHandler(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input order.CreateOrderInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := svc.CreateOrder(c.Request.Context(), GetActor(c), input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	}
}

func getOrderHandler(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		resp, err := svc.GetOrder(c.Request.Context(), GetActor(c), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func listOrdersHandler(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)

		f := order.ListFilter{
			Page:  atoi(c.DefaultQuery("page", "1")),
			Limit: atoi(c.DefaultQuery("limit", "20")),
		}

		if v := c.Query("store_id"); v != "" && actor.IsAdmin() {
			id, _ := strconv.ParseUint(v, 10, 64)
			f.StoreID = uint(id)
		}
		if v := c.Query("cashier_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			f.CashierID = uint(id)
		}
		if v := c.Query("status"); v != "" {
			f.Status = v
		}
		if v := c.Query("date_from"); v != "" {
			start, _, err := order.ParseWIBDate(v)
			if err == nil {
				f.DateFrom = start
			}
		}
		if v := c.Query("date_to"); v != "" {
			_, end, err := order.ParseWIBDate(v)
			if err == nil {
				f.DateTo = end
			}
		}
		// Default to today (WIB) if no date filter provided.
		if f.DateFrom.IsZero() && f.DateTo.IsZero() {
			wib := time.FixedZone("WIB", 7*3600)
			now := time.Now().In(wib)
			today := now.Format("2006-01-02")
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

func addPaymentHandler(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input order.AddPaymentInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := svc.AddPayment(c.Request.Context(), GetActor(c), id, input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func voidOrderHandler(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input order.VoidInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := svc.VoidOrder(c.Request.Context(), GetActor(c), id, input); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "order voided"})
	}
}

func updateOrderMetaHandler(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input order.UpdateMetaInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := svc.UpdateMeta(c.Request.Context(), GetActor(c), id, input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func continueOrderHandler(svc order.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input order.ContinueInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := svc.ContinueOrder(c.Request.Context(), GetActor(c), id, input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
