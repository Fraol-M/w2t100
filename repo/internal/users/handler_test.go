package users

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *Service) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}
	authCfg := config.AuthConfig{BcryptCost: 12}
	svc := NewService(repo, audit, encryptor, authCfg)

	return router, svc
}

// setAuthContext is a middleware helper that sets auth context for tests.
func setAuthContext(userID uint64, roles []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(string(common.CtxKeyUserID), userID)
		c.Set(string(common.CtxKeyRoles), roles)
		c.Set(string(common.CtxKeyRequestID), "test-req-id")
		c.Next()
	}
}

func TestHandler_CreateUser(t *testing.T) {
	router, svc := setupTestRouter()
	h := NewHandler(svc)

	router.POST("/users", setAuthContext(1, []string{"SystemAdmin"}), h.CreateUser)

	body := CreateUserRequest{
		Username:  "newuser",
		Email:     "new@example.com",
		Password:  "securepassword123",
		FirstName: "John",
		LastName:  "Doe",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp common.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success response")
	}
}

func TestHandler_CreateUser_InvalidBody(t *testing.T) {
	router, svc := setupTestRouter()
	h := NewHandler(svc)

	router.POST("/users", setAuthContext(1, []string{"SystemAdmin"}), h.CreateUser)

	// Send an incomplete body (missing required fields)
	body := map[string]string{"username": "test"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_GetUser(t *testing.T) {
	router, svc := setupTestRouter()
	h := NewHandler(svc)

	// First create a user via service
	createReq := CreateUserRequest{
		Username: "getuser",
		Email:    "get@example.com",
		Password: "securepassword123",
	}
	created, appErr := svc.Create(createReq, 1, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("create failed: %v", appErr)
	}

	router.GET("/users/:id", setAuthContext(created.ID, []string{"Tenant"}), h.GetUser)

	req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp common.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success response")
	}
}

func TestHandler_GetUser_NotFound(t *testing.T) {
	router, svc := setupTestRouter()
	h := NewHandler(svc)

	router.GET("/users/:id", setAuthContext(1, []string{"SystemAdmin"}), h.GetUser)

	req := httptest.NewRequest(http.MethodGet, "/users/999", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_UpdateUser_Unauthorized(t *testing.T) {
	router, svc := setupTestRouter()
	h := NewHandler(svc)

	// Create a user
	createReq := CreateUserRequest{
		Username: "target",
		Email:    "target@example.com",
		Password: "securepassword123",
	}
	created, _ := svc.Create(createReq, 1, "127.0.0.1", "req-1")

	// Another non-admin user tries to update
	router.PUT("/users/:id", setAuthContext(999, []string{"Tenant"}), h.UpdateUser)

	firstName := "Hacker"
	body := UpdateUserRequest{FirstName: &firstName}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/users/"+idStr(created.ID), bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateUser_OwnProfile(t *testing.T) {
	router, svc := setupTestRouter()
	h := NewHandler(svc)

	// Create a user
	createReq := CreateUserRequest{
		Username: "selfupdate",
		Email:    "self@example.com",
		Password: "securepassword123",
	}
	created, _ := svc.Create(createReq, 1, "127.0.0.1", "req-1")

	// User updates their own profile
	router.PUT("/users/:id", setAuthContext(created.ID, []string{"Tenant"}), h.UpdateUser)

	firstName := "Updated"
	body := UpdateUserRequest{FirstName: &firstName}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/users/"+idStr(created.ID), bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListUsers(t *testing.T) {
	router, svc := setupTestRouter()
	h := NewHandler(svc)

	// Create some users
	for i := 0; i < 3; i++ {
		req := CreateUserRequest{
			Username: "listuser" + idStr(uint64(i)),
			Email:    "list" + idStr(uint64(i)) + "@example.com",
			Password: "password123",
		}
		_, _ = svc.Create(req, 1, "127.0.0.1", "req")
	}

	router.GET("/users", setAuthContext(1, []string{"SystemAdmin"}), h.ListUsers)

	req := httptest.NewRequest(http.MethodGet, "/users?page=1&per_page=10", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp common.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success response")
	}
	if resp.Meta == nil {
		t.Error("expected meta in response")
	}
}

func idStr(id uint64) string {
	return fmt.Sprintf("%d", id)
}
