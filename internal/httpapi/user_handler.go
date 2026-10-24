package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/auth"
)

func registerUserRoutes(rg *gin.RouterGroup, svc auth.Service) {
	users := rg.Group("/users")
	users.Use(RequireAdmin())
	{
		users.GET("", listUsersHandler(svc))
		users.POST("", createUserHandler(svc))
		users.PUT("/:id", updateUserHandler(svc))
		users.DELETE("/:id", deactivateUserHandler(svc))
	}
}

func listUsersHandler(svc auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		users, err := svc.ListUsers(c.Request.Context(), GetActor(c))
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, users)
	}
}

func createUserHandler(svc auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input auth.CreateUserInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		user, err := svc.CreateUser(c.Request.Context(), GetActor(c), input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, user)
	}
}

func updateUserHandler(svc auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var input auth.UpdateUserInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := svc.UpdateUser(c.Request.Context(), GetActor(c), id, input); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "updated"})
	}
}

func deactivateUserHandler(svc auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := svc.DeactivateUser(c.Request.Context(), GetActor(c), id); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deactivated"})
	}
}

func parseUintParam(c *gin.Context, name string) (uint, error) {
	v, err := strconv.ParseUint(c.Param(name), 10, 64)
	return uint(v), err
}
