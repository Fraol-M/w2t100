package governance

import (
	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Middleware is the interface for the HTTP middleware used by route registration.
type Middleware interface {
	RequireRoles(roles ...string) gin.HandlerFunc
	RateLimit(action string) gin.HandlerFunc
}

// RegisterRoutes registers all governance routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, service *Service, evidence EvidenceUploader, mw Middleware) {
	h := NewHandler(service, evidence)

	reviewerOrAdmin := mw.RequireRoles(common.RoleComplianceReviewer, common.RoleSystemAdmin)
	adminOnly := mw.RequireRoles(common.RoleSystemAdmin)

	// Reports — any authenticated user can file a report; only reviewers/admins can list/read/review.
	reports := rg.Group("/reports")
	{
		reports.POST("", mw.RateLimit("report_create"), h.CreateReport)
		reports.GET("", reviewerOrAdmin, h.ListReports)
		reports.GET("/:id", reviewerOrAdmin, h.GetReport)
		reports.PATCH("/:id/review", reviewerOrAdmin, h.ReviewReport)
		// Evidence attachments — the original reporter may upload; reviewers/admins can always access.
		// Object-level authorization is enforced inside the handler.
		reports.POST("/:id/evidence", h.UploadEvidence)
		reports.GET("/:id/evidence", h.ListEvidence)
	}

	// Enforcements — ComplianceReviewer/SystemAdmin only.
	enforcements := rg.Group("/enforcements")
	enforcements.Use(reviewerOrAdmin)
	{
		enforcements.POST("", h.CreateEnforcement)
		enforcements.GET("", h.ListEnforcements)
		enforcements.POST("/:id/revoke", h.RevokeEnforcement)
	}

	// Keywords — SystemAdmin only.
	keywords := rg.Group("/keywords")
	keywords.Use(adminOnly)
	{
		keywords.POST("", h.CreateKeyword)
		keywords.GET("", h.ListKeywords)
		keywords.DELETE("/:id", h.DeleteKeyword)
	}

	// Risk rules — SystemAdmin only.
	riskRules := rg.Group("/risk-rules")
	riskRules.Use(adminOnly)
	{
		riskRules.POST("", h.CreateRiskRule)
		riskRules.GET("", h.ListRiskRules)
		riskRules.GET("/:id", h.GetRiskRule)
		riskRules.PUT("/:id", h.UpdateRiskRule)
		riskRules.DELETE("/:id", h.DeleteRiskRule)
	}
}
