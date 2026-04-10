package payments

import (
	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Middleware is the interface for the HTTP middleware used by route registration.
type Middleware interface {
	RequireRoles(roles ...string) gin.HandlerFunc
}

// RegisterRoutes registers all payment routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, service *Service, mw Middleware) {
	h := NewHandler(service)

	// Only PropertyManager and SystemAdmin may manage payment data.
	pmOrAdmin := mw.RequireRoles(common.RolePropertyManager, common.RoleSystemAdmin)

	// Payment intents — create.
	rg.POST("/intents", pmOrAdmin, h.CreateIntent)

	// List and get payments.
	rg.GET("", pmOrAdmin, h.ListPayments)
	rg.GET("/:id", pmOrAdmin, h.GetPayment)

	// Payment actions.
	rg.POST("/:id/mark-paid", pmOrAdmin, h.MarkPaid)
	rg.POST("/:id/approve", pmOrAdmin, h.ApprovePayment)
	rg.POST("/:id/reverse", pmOrAdmin, h.CreateReversal)
	rg.POST("/:id/makeup", pmOrAdmin, h.CreateMakeup)

	// Settlements.
	rg.POST("/settlements", pmOrAdmin, h.CreateSettlement)

	// Reconciliation — SystemAdmin only.
	// Reconciliation aggregates cross-portfolio financial data for a given date and must
	// not be scoped or accessible to PropertyManagers, who have single-portfolio visibility.
	adminOnly := mw.RequireRoles(common.RoleSystemAdmin)
	recon := rg.Group("/reconciliation")
	recon.Use(adminOnly)
	{
		recon.POST("/run", h.RunReconciliation)
		recon.GET("", h.ListReconciliations)
		recon.GET("/:id", h.GetReconciliation)
	}
}
