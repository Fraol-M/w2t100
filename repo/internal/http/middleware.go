package http

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"propertyops/backend/internal/auth"
	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLogger is the interface for audit logging used by middleware.
// Defined here to avoid circular imports with the audit package.
type AuditLogger interface {
	Log(actorID uint64, action, resourceType string, resourceID uint64, description string, ipAddress string, requestID string)
}

// LogService is the interface for structured request logging.
type LogService interface {
	LogRequest(method, path string, status int, duration time.Duration, requestID string, actorID uint64, ip string)
}

// Middleware holds shared dependencies for all HTTP middleware.
type Middleware struct {
	authService *auth.Service
	auditLogger AuditLogger
	db          *gorm.DB
	cfg         *config.Config
}

// NewMiddleware creates a new Middleware instance.
func NewMiddleware(authService *auth.Service, auditLogger AuditLogger, db *gorm.DB, cfg *config.Config) *Middleware {
	return &Middleware{
		authService: authService,
		auditLogger: auditLogger,
		db:          db,
		cfg:         cfg,
	}
}

// RequestID generates a UUID for each request, sets the X-Request-ID response
// header, and stores it in the Gin context.
func (m *Middleware) RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		c.Set(string(common.CtxKeyRequestID), id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// StructuredLogging logs each request with method, path, status, duration,
// request_id, and actor_id. Passwords and tokens are never logged.
func (m *Middleware) StructuredLogging(logService LogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path

		requestID, _ := c.Get(string(common.CtxKeyRequestID))
		reqIDStr, _ := requestID.(string)

		var actorID uint64
		if uid, exists := c.Get(string(common.CtxKeyUserID)); exists {
			actorID, _ = uid.(uint64)
		}

		ip := c.ClientIP()

		if logService != nil {
			logService.LogRequest(method, path, status, duration, reqIDStr, actorID, ip)
		} else {
			log.Printf("request method=%s path=%s status=%d duration=%s request_id=%s actor_id=%d ip=%s",
				method, path, status, duration, reqIDStr, actorID, ip)
		}
	}
}

// PanicRecovery recovers from panics, logs the error, and returns a 500 response.
func (m *Middleware) PanicRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				requestID, _ := c.Get(string(common.CtxKeyRequestID))
				reqIDStr, _ := requestID.(string)
				log.Printf("PANIC recovered: %v request_id=%s path=%s", r, reqIDStr, c.Request.URL.Path)
				common.RespondError(c, common.NewInternalError(""))
				c.Abort()
			}
		}()
		c.Next()
	}
}

// Authenticate reads the Authorization Bearer token from the request,
// validates the session, and stores user_id, roles, and session_id in context.
func (m *Middleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			common.RespondError(c, common.NewUnauthorizedError(""))
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			common.RespondError(c, common.NewUnauthorizedError("Invalid authorization header format"))
			c.Abort()
			return
		}

		rawToken := strings.TrimSpace(parts[1])
		if rawToken == "" {
			common.RespondError(c, common.NewUnauthorizedError(""))
			c.Abort()
			return
		}

		tokenHash := auth.HashToken(rawToken)

		session, appErr := m.authService.ValidateSession(tokenHash)
		if appErr != nil {
			common.RespondError(c, appErr)
			c.Abort()
			return
		}

		// Load user to get roles
		user, appErr := m.authService.GetUserByID(session.UserID)
		if appErr != nil {
			common.RespondError(c, appErr)
			c.Abort()
			return
		}

		c.Set(string(common.CtxKeyUserID), user.ID)
		c.Set(string(common.CtxKeyUsername), user.Username)
		c.Set(string(common.CtxKeyRoles), user.RoleNames())
		c.Set(string(common.CtxKeySessionID), tokenHash)

		c.Next()
	}
}

// RequireRole checks that the authenticated user has at least one of the specified roles.
// This satisfies the Middleware interface expected by other packages.
func (m *Middleware) RequireRole(roles ...string) gin.HandlerFunc {
	return m.RequireRoles(roles...)
}

// RequireRoles checks that the authenticated user has at least one of the specified roles.
func (m *Middleware) RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		userRolesVal, exists := c.Get(string(common.CtxKeyRoles))
		if !exists {
			common.RespondError(c, common.NewForbiddenError(""))
			c.Abort()
			return
		}

		userRoles, ok := userRolesVal.([]string)
		if !ok {
			common.RespondError(c, common.NewForbiddenError(""))
			c.Abort()
			return
		}

		for _, r := range userRoles {
			if _, found := allowed[r]; found {
				c.Next()
				return
			}
		}

		common.RespondError(c, common.NewForbiddenError("Insufficient permissions"))
		c.Abort()
	}
}

// CheckSuspension queries enforcement_actions for an active suspension on
// the authenticated user and returns 403 if one is found.
func (m *Middleware) CheckSuspension() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get(string(common.CtxKeyUserID))
		if !exists {
			c.Next()
			return
		}

		userID, ok := userIDVal.(uint64)
		if !ok {
			c.Next()
			return
		}

		var count int64
		err := m.db.Table("enforcement_actions").
			Where("user_id = ? AND action_type = ? AND is_active = ? AND (ends_at IS NULL OR ends_at > ?) AND revoked_at IS NULL",
				userID, common.EnforcementSuspension, true, time.Now().UTC()).
			Count(&count).Error
		if err != nil {
			log.Printf("WARN CheckSuspension: failed to query enforcement_actions: %v", err)
			c.Next()
			return
		}

		if count > 0 {
			common.RespondError(c, common.NewForbiddenError("Your account is currently suspended"))
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimit enforces a DB-backed rate limit. It counts how many records the
// authenticated user has created for the given action within the enforcement
// window (defaults to 60 minutes; overridden by rate_limit_window_minutes on
// an active EnforcementAction) and returns 429 if the max is reached.
func (m *Middleware) RateLimit(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get(string(common.CtxKeyUserID))
		if !exists {
			c.Next()
			return
		}

		userID, ok := userIDVal.(uint64)
		if !ok {
			c.Next()
			return
		}

		maxPerHour := m.cfg.RateLimit.MaxSubmissionsPerHour

		// Check for a custom rate-limit enforcement action; honour both the
		// max count and the operator-configured window (defaults to 60 minutes).
		var customLimit struct {
			RateLimitMax           int
			RateLimitWindowMinutes int
		}
		windowMinutes := 60 // default 1-hour window
		err := m.db.Table("enforcement_actions").
			Select("rate_limit_max, rate_limit_window_minutes").
			Where("user_id = ? AND action_type = ? AND is_active = ? AND (ends_at IS NULL OR ends_at > ?) AND revoked_at IS NULL",
				userID, common.EnforcementRateLimit, true, time.Now().UTC()).
			Order("rate_limit_max ASC").
			Limit(1).
			Scan(&customLimit).Error
		if err == nil {
			if customLimit.RateLimitMax > 0 {
				maxPerHour = customLimit.RateLimitMax
			}
			if customLimit.RateLimitWindowMinutes > 0 {
				windowMinutes = customLimit.RateLimitWindowMinutes
			}
		}

		windowStart := time.Now().UTC().Add(-time.Duration(windowMinutes) * time.Minute)

		var count int64
		err = m.db.Table("audit_logs").
			Where("actor_id = ? AND action = ? AND created_at > ?", userID, action, windowStart).
			Count(&count).Error
		if err != nil {
			log.Printf("WARN RateLimit: failed to count actions: %v", err)
			c.Next()
			return
		}

		if count >= int64(maxPerHour) {
			common.RespondError(c, common.NewRateLimitError())
			c.Abort()
			return
		}

		c.Next()
	}
}

// LocalNetworkOnly restricts access to requests originating from the specified
// CIDR ranges. Returns 403 for requests from disallowed IPs.
func (m *Middleware) LocalNetworkOnly(allowedCIDRs []string) gin.HandlerFunc {
	// Parse CIDRs at middleware creation time
	nets := make([]*net.IPNet, 0, len(allowedCIDRs))
	for _, cidr := range allowedCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			log.Printf("WARN LocalNetworkOnly: invalid CIDR %q: %v", cidr, err)
			continue
		}
		nets = append(nets, ipNet)
	}

	return func(c *gin.Context) {
		clientIP := net.ParseIP(c.ClientIP())
		if clientIP == nil {
			common.RespondError(c, common.NewForbiddenError("Unable to determine client IP"))
			c.Abort()
			return
		}

		for _, ipNet := range nets {
			if ipNet.Contains(clientIP) {
				c.Next()
				return
			}
		}

		common.RespondError(c, common.NewForbiddenError(fmt.Sprintf("Access restricted to local network")))
		c.Abort()
	}
}

// AnomalyAllowlist checks client IP against configured CIDR allowlist.
// Uses the anomaly.AllowedCIDRs from the application configuration.
func (m *Middleware) AnomalyAllowlist() gin.HandlerFunc {
	return m.LocalNetworkOnly(m.cfg.Anomaly.AllowedCIDRs)
}

// CORS returns a middleware that sets Cross-Origin Resource Sharing headers.
func (m *Middleware) CORS(allowedOrigins []string) gin.HandlerFunc {
	originsMap := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originsMap[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := originsMap[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
