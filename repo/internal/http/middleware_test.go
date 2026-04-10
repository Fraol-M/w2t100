package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"propertyops/backend/internal/auth"
	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"
	httpMw "propertyops/backend/internal/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func testConfig() *config.Config {
	return &config.Config{
		Auth: config.AuthConfig{
			BcryptCost:         12,
			SessionIdleTimeout: 30 * time.Minute,
			SessionMaxLifetime: 7 * 24 * time.Hour,
		},
		RateLimit: config.RateLimitConfig{
			MaxSubmissionsPerHour: 10,
		},
		Anomaly: config.AnomalyConfig{
			AllowedCIDRs: []string{"127.0.0.1/32", "10.0.0.0/8"},
		},
	}
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Skipf("SQLite unavailable (CGO disabled or no C compiler): %v", err)
	}
	if err := db.AutoMigrate(&auth.User{}, &auth.Role{}, &auth.Session{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	db.Exec("CREATE TABLE IF NOT EXISTS user_roles (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, role_id INTEGER NOT NULL)")
	// Create audit_logs table for rate limit tests
	db.Exec("CREATE TABLE IF NOT EXISTS audit_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, actor_id INTEGER NOT NULL, action TEXT NOT NULL, created_at DATETIME NOT NULL)")
	// Create enforcement_actions table for suspension/rate limit tests
	db.Exec("CREATE TABLE IF NOT EXISTS enforcement_actions (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, action_type TEXT NOT NULL, is_active BOOLEAN NOT NULL, ends_at DATETIME, revoked_at DATETIME, rate_limit_max INTEGER)")
	return db
}

func seedUserWithRole(t *testing.T, db *gorm.DB, username, password, roleName string) *auth.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	role := auth.Role{Name: roleName}
	db.Create(&role)

	user := auth.User{
		UUID:         "test-uuid-" + username,
		Username:     username,
		Email:        username + "@test.com",
		PasswordHash: string(hash),
		IsActive:     true,
	}
	db.Create(&user)
	db.Exec("INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)", user.ID, role.ID)
	return &user
}

type noopAuditLogger struct{}

func (n *noopAuditLogger) Log(actorID uint64, action, resourceType string, resourceID uint64, description string, ipAddress string, requestID string) {
}

func newMiddleware(t *testing.T, db *gorm.DB) (*httpMw.Middleware, *auth.Service) {
	t.Helper()
	cfg := testConfig()
	repo := auth.NewRepository(db)
	svc := auth.NewService(repo, cfg.Auth, &noopAuditLogger{})
	mw := httpMw.NewMiddleware(svc, &noopAuditLogger{}, db, cfg)
	return mw, svc
}

func TestRequestIDGeneration(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.Use(mw.RequestID())
	r.GET("/test", func(c *gin.Context) {
		reqID, exists := c.Get(string(common.CtxKeyRequestID))
		if !exists {
			t.Error("request_id not set in context")
		}
		c.String(http.StatusOK, reqID.(string))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Check X-Request-ID header was set
	xReqID := w.Header().Get("X-Request-ID")
	if xReqID == "" {
		t.Error("X-Request-ID header not set")
	}

	// Body should match header
	if w.Body.String() != xReqID {
		t.Errorf("body=%q != X-Request-ID=%q", w.Body.String(), xReqID)
	}
}

func TestRequestIDPreserved(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.Use(mw.RequestID())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	customID := "my-custom-request-id-123"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", customID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") != customID {
		t.Errorf("expected X-Request-ID=%q, got %q", customID, w.Header().Get("X-Request-ID"))
	}
}

func TestRequireRolesAllowed(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		// Simulate authenticated context
		c.Set(string(common.CtxKeyRoles), []string{"SystemAdmin", "PropertyManager"})
		c.Next()
	}, mw.RequireRoles("SystemAdmin"), func(c *gin.Context) {
		c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRequireRolesDenied(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		c.Set(string(common.CtxKeyRoles), []string{"Tenant"})
		c.Next()
	}, mw.RequireRoles("SystemAdmin"), func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	var resp common.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false")
	}
}

func TestRequireRolesNoRoles(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.GET("/admin", mw.RequireRoles("SystemAdmin"), func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRequireRoleAllowed(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.GET("/pm", func(c *gin.Context) {
		c.Set(string(common.CtxKeyRoles), []string{"PropertyManager"})
		c.Next()
	}, mw.RequireRole("PropertyManager", "SystemAdmin"), func(c *gin.Context) {
		c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/pm", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRequireRoleDenied(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.GET("/pm", func(c *gin.Context) {
		c.Set(string(common.CtxKeyRoles), []string{"Tenant"})
		c.Next()
	}, mw.RequireRole("SystemAdmin"), func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodGet, "/pm", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestAuthenticateNoHeader(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.GET("/protected", mw.Authenticate(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthenticateInvalidFormat(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.GET("/protected", mw.Authenticate(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthenticateExpiredSession(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	// Create an expired session directly in the DB
	now := time.Now().UTC()
	session := auth.Session{
		TokenHash:         auth.HashToken("expired-token"),
		UserID:            1,
		IPAddress:         "127.0.0.1",
		UserAgent:         "test",
		CreatedAt:         now.Add(-8 * 24 * time.Hour),
		LastActiveAt:      now.Add(-2 * time.Hour),
		IdleExpiresAt:     now.Add(-1 * time.Hour), // idle expired
		AbsoluteExpiresAt: now.Add(-24 * time.Hour), // absolute expired
	}
	db.Create(&session)

	r := gin.New()
	r.GET("/protected", mw.Authenticate(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthenticateValidSession(t *testing.T) {
	db := setupTestDB(t)
	mw, svc := newMiddleware(t, db)

	user := seedUserWithRole(t, db, "authuser", "Password123!", "Tenant")

	// Login to create a valid session
	loginResp, appErr := svc.Login("authuser", "Password123!", "127.0.0.1", "test-agent", "req-1")
	if appErr != nil {
		t.Fatalf("login failed: %v", appErr)
	}

	r := gin.New()
	r.GET("/protected", mw.Authenticate(), func(c *gin.Context) {
		uid, _ := c.Get(string(common.CtxKeyUserID))
		if uid.(uint64) != user.ID {
			t.Errorf("expected user_id=%d, got %v", user.ID, uid)
		}
		roles, _ := c.Get(string(common.CtxKeyRoles))
		roleList := roles.([]string)
		if len(roleList) == 0 || roleList[0] != "Tenant" {
			t.Errorf("expected roles=[Tenant], got %v", roleList)
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPanicRecovery(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.Use(mw.RequestID(), mw.PanicRecovery())
	r.GET("/panic", func(c *gin.Context) {
		panic("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	var resp common.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false")
	}
}

func TestLocalNetworkOnlyAllowed(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.GET("/internal", mw.LocalNetworkOnly([]string{"127.0.0.1/32"}), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/internal", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLocalNetworkOnlyDenied(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	r := gin.New()
	r.GET("/internal", mw.LocalNetworkOnly([]string{"10.0.0.0/8"}), func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodGet, "/internal", nil)
	req.RemoteAddr = "203.0.113.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRateLimitEnforcement(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	// Insert audit logs to simulate reaching the limit (10 per hour)
	now := time.Now().UTC()
	for i := 0; i < 10; i++ {
		db.Exec("INSERT INTO audit_logs (actor_id, action, created_at) VALUES (?, ?, ?)",
			1, common.AuditActionCreate, now.Add(-time.Duration(i)*time.Minute))
	}

	r := gin.New()
	r.POST("/submit", func(c *gin.Context) {
		c.Set(string(common.CtxKeyUserID), uint64(1))
		c.Next()
	}, mw.RateLimit(common.AuditActionCreate), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRateLimitAllowed(t *testing.T) {
	db := setupTestDB(t)
	mw, _ := newMiddleware(t, db)

	// Only insert 5 audit logs (under the 10 per hour limit)
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		db.Exec("INSERT INTO audit_logs (actor_id, action, created_at) VALUES (?, ?, ?)",
			1, common.AuditActionCreate, now.Add(-time.Duration(i)*time.Minute))
	}

	r := gin.New()
	r.POST("/submit", func(c *gin.Context) {
		c.Set(string(common.CtxKeyUserID), uint64(1))
		c.Next()
	}, mw.RateLimit(common.AuditActionCreate), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

