package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes registers the public auth routes (login only) on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, service *Service) {
	h := NewHandler(service)

	// Public — no auth required
	rg.POST("/login", h.Login)
}

// RegisterProtectedRoutes registers the auth routes that require an authenticated session.
// The caller must ensure that Authenticate() middleware is already applied to rg.
func RegisterProtectedRoutes(rg *gin.RouterGroup, service *Service) {
	h := NewHandler(service)

	// Requires valid session in context (set by Authenticate middleware).
	rg.POST("/logout", h.Logout)
	rg.GET("/me", h.Me)
}
