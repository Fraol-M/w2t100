package notifications

import "github.com/gin-gonic/gin"

// Middleware is the interface for the HTTP middleware used by route registration.
type Middleware interface {
	RequireRoles(roles ...string) gin.HandlerFunc
}

// RegisterRoutes registers all notification and thread routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, service *Service, mw Middleware) {
	h := NewHandler(service)

	// Notification endpoints (all authenticated users).
	rg.GET("", h.ListNotifications)
	rg.GET("/unread-count", h.GetUnreadCount)
	rg.GET("/:id", h.GetNotification)
	rg.PATCH("/:id/read", h.MarkRead)

	// Send endpoint (admin/manager only).
	rg.POST("/send", h.SendNotification)

	// Thread endpoints.
	threads := rg.Group("/threads")
	{
		threads.POST("", h.CreateThread)
		threads.GET("", h.ListThreads)
		threads.GET("/:id", h.GetThread)
		threads.POST("/:id/participants", h.AddParticipant)
		threads.GET("/:id/participants", h.ListParticipants)
		threads.POST("/:id/messages", h.AddMessage)
		threads.GET("/:id/messages", h.ListMessages)
	}
}
