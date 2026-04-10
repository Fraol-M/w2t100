package properties

import (
	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Middleware defines the interface for route middleware used by property routes.
type Middleware interface {
	RequireRole(roles ...string) gin.HandlerFunc
}

// RegisterRoutes registers all property-related routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, service *Service, mw Middleware) {
	h := NewHandler(service)

	pmOrAdmin := mw.RequireRole(common.RoleSystemAdmin, common.RolePropertyManager)

	// Property CRUD
	rg.POST("", pmOrAdmin, h.CreateProperty)
	rg.GET("", pmOrAdmin, h.ListProperties)
	rg.GET("/:id", pmOrAdmin, h.GetProperty)
	rg.PUT("/:id", pmOrAdmin, h.UpdateProperty)

	// Unit CRUD (nested under property)
	rg.POST("/:id/units", pmOrAdmin, h.CreateUnit)
	rg.GET("/:id/units", pmOrAdmin, h.ListUnits)
	rg.GET("/:id/units/:unit_id", pmOrAdmin, h.GetUnit)
	rg.PUT("/:id/units/:unit_id", pmOrAdmin, h.UpdateUnit)

	// Staff assignments
	rg.POST("/:id/staff", pmOrAdmin, h.AssignStaff)
	rg.GET("/:id/staff", pmOrAdmin, h.ListStaff)
	rg.DELETE("/:id/staff/:user_id/:role", pmOrAdmin, h.RemoveStaff)

	// Technician skill tags
	techGroup := rg.Group("/technicians")
	techGroup.Use(mw.RequireRole(common.RoleSystemAdmin, common.RolePropertyManager))
	{
		techGroup.POST("/:user_id/skills", h.AddSkillTag)
		techGroup.GET("/:user_id/skills", h.ListSkillTags)
		techGroup.DELETE("/:user_id/skills/:tag", h.RemoveSkillTag)
	}
}
