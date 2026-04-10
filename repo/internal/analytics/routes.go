package analytics

import (
	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Middleware is the subset of HTTP middleware needed by this package.
type Middleware interface {
	RequireRole(roles ...string) gin.HandlerFunc
}

// PropertyChecker abstracts the managed-property lookup used for PM scoping.
type PropertyChecker interface {
	GetManagedPropertyIDs(userID uint64) ([]uint64, error)
}

// RegisterRoutes registers all analytics endpoints on the provided router group.
// The rg group is expected to be mounted at /analytics.
//
// All routes require PropertyManager or SystemAdmin role.
func RegisterRoutes(rg *gin.RouterGroup, svc *Service, mw Middleware, propChecker PropertyChecker) {
	h := NewHandler(svc, propChecker)

	pm := mw.RequireRole(common.RolePropertyManager, common.RoleSystemAdmin)

	// Metric endpoints
	rg.GET("/popularity", pm, h.GetPopularity)
	rg.GET("/funnel", pm, h.GetFunnel)
	rg.GET("/retention", pm, h.GetRetention)
	rg.GET("/tags", pm, h.GetTags)
	rg.GET("/quality", pm, h.GetQuality)

	// Saved report endpoints
	reports := rg.Group("/reports")
	reports.Use(pm)
	{
		reports.POST("/saved", h.CreateSavedReport)
		reports.GET("/saved", h.ListSavedReports)
		reports.GET("/saved/:id", h.GetSavedReport)
		reports.DELETE("/saved/:id", h.DeleteSavedReport)
		reports.POST("/generate/:id", h.GenerateReport)
		reports.GET("/generated/:id", h.GetGeneratedReport)
	}

	// Ad-hoc export
	rg.POST("/export", pm, h.Export)
}
