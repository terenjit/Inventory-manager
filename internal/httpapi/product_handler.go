package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/product"
)

func registerProductRoutes(rg *gin.RouterGroup, svc product.Service) {
	r := rg.Group("/products")
	r.GET("", listProductsHandler(svc))
	r.POST("", RequireAdmin(), createProductHandler(svc))
	r.POST("/upsert", RequireAdmin(), upsertProductHandler(svc))
	r.DELETE("/by-code/:code", RequireAdmin(), deactivateProductByCodeHandler(svc))
	r.GET("/:id", getProductHandler(svc))
	r.PUT("/:id", RequireAdmin(), updateProductHandler(svc))
	r.DELETE("/:id", RequireAdmin(), deactivateProductHandler(svc))
}

func listProductsHandler(svc product.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)

		var storeID uint
		if actor.StoreID != nil {
			storeID = *actor.StoreID
		}
		if actor.IsAdmin() {
			if v, err := strconv.ParseUint(c.Query("store_id"), 10, 64); err == nil {
				storeID = uint(v)
			}
		}

		categoryID, _ := strconv.ParseUint(c.Query("category_id"), 10, 64)
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

		f := product.ListFilter{
			Search:      c.Query("search"),
			CategoryID:  uint(categoryID),
			LowStock:    c.Query("low_stock") == "true",
			InStockOnly: c.Query("in_stock") == "true",
			StoreID:     storeID,
			Page:        page,
			Limit:       limit,
		}

		result, err := svc.List(c.Request.Context(), actor, f)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func getProductHandler(svc product.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		p, err := svc.GetByID(c.Request.Context(), GetActor(c), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

func createProductHandler(svc product.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input product.CreateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		p, err := svc.Create(c.Request.Context(), GetActor(c), input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, p)
	}
}

func updateProductHandler(svc product.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input product.UpdateInput
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

func upsertProductHandler(svc product.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input product.CreateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		p, err := svc.UpsertByCode(c.Request.Context(), GetActor(c), input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

func deactivateProductByCodeHandler(svc product.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "code required"})
			return
		}
		if err := svc.DeactivateByCode(c.Request.Context(), GetActor(c), code); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deactivated"})
	}
}

func deactivateProductHandler(svc product.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := svc.Deactivate(c.Request.Context(), GetActor(c), id); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deactivated"})
	}
}
