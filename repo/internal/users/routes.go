package users

import (
	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Middleware defines the interface for route middleware used by user routes.
type Middleware interface {
	RequireRole(roles ...string) gin.HandlerFunc
}

// RegisterRoutes registers all user-related routes on the given router group.
// SystemAdmin can manage all users; users can view/update their own profile.
func RegisterRoutes(rg *gin.RouterGroup, service *Service, mw Middleware) {
	h := NewHandler(service)

	usersGroup := rg.Group("/users")
	{
		// Admin-only: create, list, toggle active, assign/remove roles
		admin := usersGroup.Group("")
		admin.Use(mw.RequireRole(common.RoleSystemAdmin))
		{
			admin.POST("", h.CreateUser)
			admin.GET("", h.ListUsers)
			admin.PATCH("/:id/active", h.ToggleActive)
			admin.POST("/:id/roles", h.AssignRole)
			admin.DELETE("/:id/roles/:role", h.RemoveRole)
		}

		// Any authenticated user can view and update profiles
		// (object-level auth in handler ensures users can only update their own)
		usersGroup.GET("/:id", h.GetUser)
		usersGroup.PUT("/:id", h.UpdateUser)
	}
}
