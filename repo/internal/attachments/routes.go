package attachments

import "github.com/gin-gonic/gin"

// Middleware is the interface for the HTTP middleware used by route registration.
type Middleware interface {
	RequireRoles(roles ...string) gin.HandlerFunc
}

// RegisterRoutes registers all attachment routes on the provided router group.
// The rg group is expected to be mounted at /work-orders.
//
// Endpoints:
//
//	POST   /:id/attachments          — upload an attachment to a work order
//	GET    /:id/attachments          — list attachments for a work order
//	GET    /attachments/:id          — download a specific attachment
//	DELETE /attachments/:id          — delete a specific attachment
func RegisterRoutes(rg *gin.RouterGroup, service *Service, mw Middleware) {
	h := NewHandler(service)

	// Work-order-scoped routes.
	rg.POST("/:id/attachments", h.Upload)
	rg.GET("/:id/attachments", h.ListByWorkOrder)

	// Attachment-scoped routes (nested under the same group for simplicity).
	attachGroup := rg.Group("/attachments")
	attachGroup.GET("/:id", h.Download)
	attachGroup.DELETE("/:id", h.Delete)
}
