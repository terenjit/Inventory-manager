package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/opname"
)

func registerOpnameRoutes(rg *gin.RouterGroup, svc opname.Service) {
	r := rg.Group("/opnames")
	r.POST("", startOpnameHandler(svc))
	r.GET("", listOpnamesHandler(svc))
	r.GET("/:id", getOpnameHandler(svc))
	r.POST("/:id/lines", addLineHandler(svc))
	r.PUT("/:id/lines/:product_id", updateLineHandler(svc))
	r.POST("/:id/finalize", finalizeOpnameHandler(svc))
	r.POST("/:id/cancel", RequireAdmin(), cancelOpnameHandler(svc))
	r.GET("/:id/summary", RequireAdmin(), summaryHandler(svc))
}

func startOpnameHandler(svc opname.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input opname.CreateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := svc.StartOpname(c.Request.Context(), GetActor(c), input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	}
}

func listOpnamesHandler(svc opname.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		f := opname.ListFilter{
			Status: c.Query("status"),
			Page:   atoi(c.DefaultQuery("page", "1")),
			Limit:  atoi(c.DefaultQuery("limit", "20")),
		}
		actor := GetActor(c)
		if actor.IsAdmin() {
			if v := c.Query("store_id"); v != "" {
				id, _ := strconv.ParseUint(v, 10, 64)
				f.StoreID = uint(id)
			}
		}
		result, err := svc.ListOpnames(c.Request.Context(), actor, f)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func getOpnameHandler(svc opname.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		resp, err := svc.GetOpname(c.Request.Context(), GetActor(c), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func addLineHandler(svc opname.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input opname.AddLineInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := svc.AddLine(c.Request.Context(), GetActor(c), id, input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func updateLineHandler(svc opname.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		productID, err := parseUintParam(c, "product_id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id"})
			return
		}
		var input opname.UpdateLineInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := svc.UpdateLine(c.Request.Context(), GetActor(c), id, input, productID); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "updated"})
	}
}

func finalizeOpnameHandler(svc opname.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		resp, err := svc.Finalize(c.Request.Context(), GetActor(c), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func cancelOpnameHandler(svc opname.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := svc.Cancel(c.Request.Context(), GetActor(c), id); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "cancelled"})
	}
}

func summaryHandler(svc opname.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		resp, err := svc.Summary(c.Request.Context(), GetActor(c), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}
