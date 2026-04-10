package admin

import (
	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Middleware is the subset of HTTP middleware needed by this package.
type Middleware interface {
	RequireRole(roles ...string) gin.HandlerFunc
	AnomalyAllowlist() gin.HandlerFunc
}

// RegisterRoutes registers all admin endpoints on the provided router group.
// The rg group is expected to be mounted at /admin (already within the protected group).
//
// Routes:
//
//	GET    /admin/settings         — list system settings
//	PUT    /admin/settings/:key    — update a setting
//	GET    /admin/keys             — list encryption key versions
//	POST   /admin/keys/rotate      — rotate encryption key
//	POST   /admin/pii-export       — export PII data (purpose required)
func RegisterRoutes(
	rg *gin.RouterGroup,
	db *gorm.DB,
	cfg *config.Config,
	security SecurityService,
	audit AuditLogger,
	users UserService,
	mw Middleware,
) {
	h := NewHandler(db, cfg, security, audit, users)

	sysAdmin := mw.RequireRole(common.RoleSystemAdmin)

	// Settings management
	settings := rg.Group("/settings")
	settings.Use(sysAdmin)
	{
		settings.GET("", h.ListSettings)
		settings.PUT("/:key", h.UpdateSetting)
	}

	// Encryption key management
	keys := rg.Group("/keys")
	keys.Use(sysAdmin)
	{
		keys.GET("", h.ListKeys)
		keys.POST("/rotate", h.RotateKey)
	}

	// PII export (SystemAdmin only, requires purpose)
	rg.POST("/pii-export", sysAdmin, h.ExportPII)

	// Data retention enforcement (SystemAdmin only; permanently deletes expired DB records)
	rg.DELETE("/data-retention", sysAdmin, h.EnforceDataRetention)
}

// RegisterAnomalyRoute mounts the anomaly-ingestion endpoint on the provided group.
// This must be called OUTSIDE the authenticated middleware so that local network
// services can post anomalies without a bearer token.
// Access is limited by the AnomalyAllowlist middleware (CIDR check).
func RegisterAnomalyRoute(
	rg *gin.RouterGroup,
	db *gorm.DB,
	cfg *config.Config,
	audit AuditLogger,
	mw Middleware,
) {
	h := NewHandler(db, cfg, nil, audit, nil)
	rg.POST("/anomaly", mw.AnomalyAllowlist(), h.IngestAnomaly)
}
