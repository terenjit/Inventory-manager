package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/supplier"
)

func registerSupplierRoutes(rg *gin.RouterGroup, svc supplier.Service) {
	r := rg.Group("/suppliers")
	r.Use(RequireAdmin())
	r.GET("", listSuppliersHandler(svc))
	r.GET("/:id", getSupplierHandler(svc))
	r.POST("", createSupplierHandler(svc))
	r.PUT("/:id", updateSupplierHandler(svc))
}

func listSuppliersHandler(svc supplier.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := svc.List(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

func getSupplierHandler(svc supplier.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		s, err := svc.GetByID(c.Request.Context(), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, s)
	}
}

func createSupplierHandler(svc supplier.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input supplier.CreateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s, err := svc.Create(c.Request.Context(), GetActor(c), input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, s)
	}
}

func updateSupplierHandler(svc supplier.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input supplier.UpdateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := svc.Update(c.Request.Context(), GetActor(c), id, input); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "updated"})
	}
}
