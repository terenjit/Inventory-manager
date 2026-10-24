package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/transfer"
)

func registerTransferRoutes(rg *gin.RouterGroup, svc transfer.Service) {
	r := rg.Group("/transfers")
	r.POST("", createTransferHandler(svc))
	r.GET("", listTransfersHandler(svc))
	r.GET("/:id", getTransferHandler(svc))
	r.POST("/:id/receive", receiveTransferHandler(svc))
	r.POST("/:id/cancel", RequireAdmin(), cancelTransferHandler(svc))
}

func createTransferHandler(svc transfer.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input transfer.CreateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := svc.CreateTransfer(c.Request.Context(), GetActor(c), input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	}
}

func listTransfersHandler(svc transfer.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		f := transfer.ListFilter{
			Direction: c.Query("direction"),
			Status:    c.Query("status"),
			Page:      atoi(c.DefaultQuery("page", "1")),
			Limit:     atoi(c.DefaultQuery("limit", "20")),
		}
		actor := GetActor(c)
		if actor.IsAdmin() {
			if v := c.Query("store_id"); v != "" {
				id, _ := parseUintParam(c, "store_id")
				f.StoreID = id
			}
		}
		result, err := svc.ListTransfers(c.Request.Context(), actor, f)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func getTransferHandler(svc transfer.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		resp, err := svc.GetTransfer(c.Request.Context(), GetActor(c), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func receiveTransferHandler(svc transfer.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		resp, err := svc.ReceiveTransfer(c.Request.Context(), GetActor(c), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func cancelTransferHandler(svc transfer.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := svc.CancelTransfer(c.Request.Context(), GetActor(c), id); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "transfer cancelled"})
	}
}
