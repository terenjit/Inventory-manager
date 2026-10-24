package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/apperr"
	"inventory-manager/internal/auth"
)

const actorKey = "actor"

// AuthRequired validates the JWT cookie and checks is_active on every request.
func AuthRequired(svc auth.Service, repo auth.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("auth_token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		actor, err := svc.ValidateToken(cookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		u, err := repo.GetByID(c.Request.Context(), actor.UserID)
		if err != nil || !u.IsActive {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "account is deactivated"})
			return
		}
		c.Set(actorKey, *actor)
		c.Next()
	}
}

// RequireAdmin aborts with 403 if the actor is not an admin.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !GetActor(c).IsAdmin() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin only"})
			return
		}
		c.Next()
	}
}

// GetActor retrieves the Actor set by AuthRequired from the Gin context.
func GetActor(c *gin.Context) auth.Actor {
	v, _ := c.Get(actorKey)
	actor, _ := v.(auth.Actor)
	return actor
}

// ResolveStore returns the correct store_id for the request.
// Cashiers always use their own store; admins use the requested store_id.
func ResolveStore(c *gin.Context, requestedStoreID uint) (uint, error) {
	actor := GetActor(c)
	if actor.IsAdmin() {
		return requestedStoreID, nil
	}
	if actor.StoreID == nil {
		return 0, apperr.Internal(errors.New("cashier has no store assigned"))
	}
	return *actor.StoreID, nil
}
