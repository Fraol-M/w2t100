package workorders

import "github.com/gin-gonic/gin"

// Middleware is the interface for the HTTP middleware used by route registration.
type Middleware interface {
	RequireRoles(roles ...string) gin.HandlerFunc
	RateLimit(action string) gin.HandlerFunc
}

// RegisterRoutes registers all work order routes on the given router group.
// uploader is optional: when non-nil, the Create endpoint additionally accepts
// multipart/form-data requests with inline attachment files.
func RegisterRoutes(rg *gin.RouterGroup, service *Service, mw Middleware, uploader AttachmentUploader) {
	h := NewHandler(service, uploader)

	rg.POST("", mw.RateLimit("work_order_create"), h.Create)
	rg.GET("", h.List)
	rg.GET("/:id", h.GetByID)
	rg.POST("/:id/dispatch", h.Dispatch)
	rg.POST("/:id/transition", h.Transition)
	rg.POST("/:id/reassign", h.Reassign)
	rg.POST("/:id/cost-items", h.AddCostItem)
	rg.GET("/:id/cost-items", h.ListCostItems)
	rg.POST("/:id/rate", h.Rate)
	rg.GET("/:id/events", h.ListEvents)
}
