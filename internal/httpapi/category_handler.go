package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/category"
)

func registerCategoryRoutes(rg *gin.RouterGroup, svc category.Service) {
	r := rg.Group("/categories")
	r.GET("", listCategoriesHandler(svc))
	r.GET("/:id", getCategoryHandler(svc))
	r.POST("", RequireAdmin(), createCategoryHandler(svc))
	r.PUT("/:id", RequireAdmin(), updateCategoryHandler(svc))
	r.DELETE("/:id", RequireAdmin(), deleteCategoryHandler(svc))
}

func listCategoriesHandler(svc category.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		cats, err := svc.List(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, cats)
	}
}

func getCategoryHandler(svc category.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		cat, err := svc.GetByID(c.Request.Context(), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, cat)
	}
}

func createCategoryHandler(svc category.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input category.CreateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cat, err := svc.Create(c.Request.Context(), GetActor(c), input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, cat)
	}
}

func updateCategoryHandler(svc category.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input category.UpdateInput
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

func deleteCategoryHandler(svc category.Service) gin.HandlerFunc {
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
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}
