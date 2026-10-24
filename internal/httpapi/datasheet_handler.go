package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/datasheet"
)

func registerDatasheetRoutes(rg *gin.RouterGroup, svc datasheet.Service) {
	r := rg.Group("/datasheet")
	r.Use(RequireAdmin())
	r.GET("/export", exportHandler(svc))
	r.POST("/import", importHandler(svc))
}

func exportHandler(svc datasheet.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var storeID uint
		if v := c.Query("store_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			storeID = uint(id)
		}
		data, err := svc.Export(c.Request.Context(), GetActor(c), storeID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=datasheet.xlsx")
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
	}
}

func importHandler(svc datasheet.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var storeID uint
		if v := c.Query("store_id"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 64)
			storeID = uint(id)
		}

		file, _, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file is required (multipart field: 'file')"})
			return
		}
		defer file.Close()

		result, rowErrs, err := svc.Import(c.Request.Context(), GetActor(c), file, storeID)
		if err != nil {
			respondError(c, err)
			return
		}
		if len(rowErrs) > 0 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"errors":  rowErrs,
				"message": "import rejected: fix all errors and retry",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":  "import successful",
			"upserted": result.Upserted,
		})
	}
}
