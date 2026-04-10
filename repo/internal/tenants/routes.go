package tenants

import (
	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Middleware defines the interface for route middleware used by tenant routes.
type Middleware interface {
	RequireRole(roles ...string) gin.HandlerFunc
}

// RegisterRoutes registers all tenant-related routes on the given router group.
// Tenants can view/update their own profile; managers and admins can manage all.
func RegisterRoutes(rg *gin.RouterGroup, service *Service, mw Middleware) {
	h := NewHandler(service)

	// Create profile (admin or property manager)
	rg.POST("", mw.RequireRole(common.RoleSystemAdmin, common.RolePropertyManager), h.CreateProfile)

	// Get profile by ID (object-level auth in handler)
	rg.GET("/:id", h.GetProfile)

	// Get profile by user ID (object-level auth in handler)
	rg.GET("/by-user/:user_id", h.GetProfileByUser)

	// Update profile (object-level auth in handler)
	rg.PUT("/:id", h.UpdateProfile)

	// List tenants by property (admin or property manager)
	rg.GET("/by-property/:property_id",
		mw.RequireRole(common.RoleSystemAdmin, common.RolePropertyManager), h.ListByProperty)
}
