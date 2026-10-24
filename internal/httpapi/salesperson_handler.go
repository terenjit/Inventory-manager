package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/salesperson"
)

func registerSalespersonRoutes(rg *gin.RouterGroup, svc salesperson.Service) {
	r := rg.Group("/salespeople")
	r.GET("", listSalespeopleHandler(svc))
	r.GET("/:id", getSalespersonHandler(svc))
	r.POST("", RequireAdmin(), createSalespersonHandler(svc))
	r.PUT("/:id", RequireAdmin(), updateSalespersonHandler(svc))
}

func listSalespeopleHandler(svc salesperson.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := svc.List(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

func getSalespersonHandler(svc salesperson.Service) gin.HandlerFunc {
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

func createSalespersonHandler(svc salesperson.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input salesperson.CreateInput
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

func updateSalespersonHandler(svc salesperson.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input salesperson.UpdateInput
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
