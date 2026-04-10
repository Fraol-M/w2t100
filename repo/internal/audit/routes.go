package audit

import (
	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Middleware is the subset of the HTTP middleware needed by this package.
type Middleware interface {
	RequireRole(roles ...string) gin.HandlerFunc
}

// RegisterRoutes wires audit log endpoints onto the provided router group.
// All routes require the SystemAdmin role.
func RegisterRoutes(rg *gin.RouterGroup, svc *Service, mw Middleware) {
	h := NewHandler(svc)

	rg.Use(mw.RequireRole(common.RoleSystemAdmin))
	rg.GET("", h.List)
	rg.GET("/:id", h.Get)
}
