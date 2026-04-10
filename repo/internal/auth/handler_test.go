package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"propertyops/backend/internal/auth"
	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory SQLite database with the necessary tables.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Skipf("SQLite unavailable (CGO disabled or no C compiler): %v", err)
	}

	// Create tables
	if err := db.AutoMigrate(&auth.User{}, &auth.Role{}, &auth.Session{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create user_roles join table manually for SQLite
	db.Exec("CREATE TABLE IF NOT EXISTS user_roles (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, role_id INTEGER NOT NULL)")

	return db
}

// seedUser inserts a test user with a bcrypt password hash.
func seedUser(t *testing.T, db *gorm.DB, username, password string) *auth.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	role := auth.Role{Name: "Tenant"}
	db.Create(&role)

	user := auth.User{
		UUID:         "test-uuid-1234",
		Username:     username,
		Email:        username + "@test.com",
		PasswordHash: string(hash),
		IsActive:     true,
	}
	db.Create(&user)

	// Assign role
	db.Exec("INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)", user.ID, role.ID)

	return &user
}

// seedInactiveUser inserts an inactive test user.
func seedInactiveUser(t *testing.T, db *gorm.DB, username, password string) *auth.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	user := auth.User{
		UUID:         "test-uuid-inactive",
		Username:     username,
		Email:        username + "@test.com",
		PasswordHash: string(hash),
		IsActive:     false,
	}
	db.Create(&user)

	return &user
}

func testAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		BcryptCost:         12,
		SessionIdleTimeout: 30 * time.Minute,
		SessionMaxLifetime: 7 * 24 * time.Hour,
	}
}

// noopAuditLogger satisfies AuditLogger but does nothing.
type noopAuditLogger struct{}

func (n *noopAuditLogger) Log(actorID uint64, action, resourceType string, resourceID uint64, description string, ipAddress string, requestID string) {
}

func setupRouter(t *testing.T) (*gin.Engine, *auth.Service, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(repo, testAuthConfig(), &noopAuditLogger{})

	r := gin.New()
	authGroup := r.Group("/auth")
	auth.RegisterRoutes(authGroup, svc)

	return r, svc, db
}

func TestLoginSuccess(t *testing.T) {
	r, _, db := setupRouter(t)
	seedUser(t, db, "testuser", "CorrectPassword123!")

	body, _ := json.Marshal(auth.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp common.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// Extract token from data
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	token, ok := dataMap["token"].(string)
	if !ok || token == "" {
		t.Fatal("expected non-empty token in response")
	}
	if len(token) != 64 {
		t.Errorf("token length = %d, want 64", len(token))
	}
}

func TestLoginWrongPassword(t *testing.T) {
	r, _, db := setupRouter(t)
	seedUser(t, db, "testuser", "CorrectPassword123!")

	body, _ := json.Marshal(auth.LoginRequest{
		Username: "testuser",
		Password: "WrongPassword",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}

	var resp common.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false")
	}
}

func TestLoginNonExistentUser(t *testing.T) {
	r, _, _ := setupRouter(t)

	body, _ := json.Marshal(auth.LoginRequest{
		Username: "nobody",
		Password: "anything",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLoginInactiveUser(t *testing.T) {
	r, _, db := setupRouter(t)
	seedInactiveUser(t, db, "inactive", "Password123!")

	body, _ := json.Marshal(auth.LoginRequest{
		Username: "inactive",
		Password: "Password123!",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}

	var resp common.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false")
	}
}

func TestLogout(t *testing.T) {
	_, svc, db := setupRouter(t)
	seedUser(t, db, "testuser", "CorrectPassword123!")

	// Login first to get a session
	loginResp, appErr := svc.Login("testuser", "CorrectPassword123!", "127.0.0.1", "test-agent", "req-1")
	if appErr != nil {
		t.Fatalf("login failed: %v", appErr)
	}

	tokenHash := auth.HashToken(loginResp.Token)

	// Set up an authenticated request to /auth/logout
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	w := httptest.NewRecorder()

	// We need to set context values that the auth middleware would set.
	// Create a new router with the context values injected.
	gin.SetMode(gin.TestMode)
	rLogout := gin.New()
	rLogout.POST("/auth/logout", func(c *gin.Context) {
		c.Set(string(common.CtxKeySessionID), tokenHash)
		c.Set(string(common.CtxKeyUserID), uint64(1))
		c.Set(string(common.CtxKeyRequestID), "test-req-id")
		auth.NewHandler(svc).Logout(c)
	})

	rLogout.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify session is revoked — validate should fail
	_, validateErr := svc.ValidateSession(tokenHash)
	if validateErr == nil {
		t.Error("expected session validation to fail after logout")
	}
}

func TestMeEndpoint(t *testing.T) {
	_, svc, db := setupRouter(t)
	user := seedUser(t, db, "testuser", "CorrectPassword123!")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/auth/me", func(c *gin.Context) {
		c.Set(string(common.CtxKeyUserID), user.ID)
		auth.NewHandler(svc).Me(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp common.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if dataMap["username"] != "testuser" {
		t.Errorf("expected username=testuser, got %v", dataMap["username"])
	}
}

func TestMeUnauthorized(t *testing.T) {
	_, svc, _ := setupRouter(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/auth/me", func(c *gin.Context) {
		// No user_id set in context — simulates unauthenticated request
		auth.NewHandler(svc).Me(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLoginInvalidBody(t *testing.T) {
	r, _, _ := setupRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
