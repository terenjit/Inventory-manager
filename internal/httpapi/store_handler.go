package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"inventory-manager/internal/apperr"
	"inventory-manager/internal/store"
)

func listStoresHandler(db *sqlx.DB) gin.HandlerFunc {
	repo := store.NewRepository(db)
	return func(c *gin.Context) {
		stores, err := repo.List(c.Request.Context())
		if err != nil {
			respondError(c, apperr.Internal(err))
			return
		}
		c.JSON(http.StatusOK, stores)
	}
}
