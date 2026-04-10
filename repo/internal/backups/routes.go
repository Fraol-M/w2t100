package backups

import (
	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Middleware is the subset of HTTP middleware needed by this package.
type Middleware interface {
	RequireRole(roles ...string) gin.HandlerFunc
}

// RegisterRoutes registers all backup endpoints on the provided router group.
// The rg group is expected to be mounted at /admin/backups.
// All routes require the SystemAdmin role.
func RegisterRoutes(rg *gin.RouterGroup, svc *Service, mw Middleware) {
	h := NewHandler(svc)

	rg.Use(mw.RequireRole(common.RoleSystemAdmin))

	rg.POST("", h.CreateBackup)
	rg.GET("", h.ListBackups)
	rg.POST("/validate", h.ValidateBackup)
	rg.DELETE("/retention", h.ApplyRetention)
}
