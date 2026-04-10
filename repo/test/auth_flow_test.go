package integration_test

import (
	"net/http"
	"testing"
	"time"

	authpkg "propertyops/backend/internal/auth"
)

// TestLogin_Success verifies that a valid username/password returns HTTP 200 and a non-empty token.
func TestLogin_Success(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, password := createTestUser(t, db, "alice", "Tenant")

	w := makeRequest(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "alice",
		"password": password,
	})
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
			User  struct {
				Username string `json:"username"`
			} `json:"user"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)

	if !resp.Success {
		t.Errorf("expected success=true")
	}
	if resp.Data.Token == "" {
		t.Errorf("expected non-empty token")
	}
	if resp.Data.User.Username != "alice" {
		t.Errorf("expected username=alice, got %q", resp.Data.User.Username)
	}
}

// TestLogin_WrongPassword verifies that an incorrect password returns HTTP 401.
func TestLogin_WrongPassword(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	createTestUser(t, db, "bob", "Tenant")

	w := makeRequest(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "bob",
		"password": "wrongpassword",
	})
	assertStatus(t, w, http.StatusUnauthorized)
}

// TestLogin_UnknownUser verifies that an unknown username returns HTTP 401.
func TestLogin_UnknownUser(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	w := makeRequest(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "nobody",
		"password": "Password123!",
	})
	assertStatus(t, w, http.StatusUnauthorized)
}

// TestLogin_InactiveUser verifies that a deactivated user cannot log in.
func TestLogin_InactiveUser(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	user, password := createTestUser(t, db, "carol", "Tenant")

	// Deactivate the user directly in the DB.
	if err := db.Model(&authpkg.User{}).Where("id = ?", user.ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("failed to deactivate user: %v", err)
	}

	w := makeRequest(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "carol",
		"password": password,
	})
	assertStatus(t, w, http.StatusUnauthorized)
}

// TestLogout verifies that POST /api/v1/auth/logout with a valid token returns HTTP 204.
func TestLogout(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	createTestUser(t, db, "dave", "Tenant")
	token := loginUser(t, router, "dave", "Password123!")

	w := makeRequest(t, router, http.MethodPost, "/api/v1/auth/logout", token, nil)
	assertStatus(t, w, http.StatusNoContent)
}

// TestLogout_ThenMeReturns401 verifies that after logout, using the old token on /me returns 401.
func TestLogout_ThenMeReturns401(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	createTestUser(t, db, "eve", "Tenant")
	token := loginUser(t, router, "eve", "Password123!")

	// Logout first.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/auth/logout", token, nil)
	assertStatus(t, w, http.StatusNoContent)

	// Now /me should be 401.
	w2 := makeRequest(t, router, http.MethodGet, "/api/v1/auth/me", token, nil)
	assertStatus(t, w2, http.StatusUnauthorized)
}

// TestGetMe verifies that GET /api/v1/auth/me with a valid token returns HTTP 200 with user info.
func TestGetMe(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	createTestUser(t, db, "frank", "Tenant")
	token := loginUser(t, router, "frank", "Password123!")

	w := makeRequest(t, router, http.MethodGet, "/api/v1/auth/me", token, nil)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Username string   `json:"username"`
			Roles    []string `json:"roles"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)

	if !resp.Success {
		t.Errorf("expected success=true")
	}
	if resp.Data.Username != "frank" {
		t.Errorf("expected username=frank, got %q", resp.Data.Username)
	}
	if len(resp.Data.Roles) == 0 {
		t.Errorf("expected at least one role, got none")
	}
}

// TestGetMe_Unauthenticated verifies that GET /api/v1/auth/me without a token returns HTTP 401.
func TestGetMe_Unauthenticated(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	w := makeRequest(t, router, http.MethodGet, "/api/v1/auth/me", "", nil)
	assertStatus(t, w, http.StatusUnauthorized)
}

// TestSessionIdleExpiry verifies that a session with an already-expired idle timeout
// returns 401 on the next request.
func TestSessionIdleExpiry(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	createTestUser(t, db, "grace", "Tenant")
	token := loginUser(t, router, "grace", "Password123!")

	// Compute token hash the same way the middleware does.
	tokenHash := authpkg.HashToken(token)

	// Backdate the session's idle_expires_at so it looks expired.
	past := time.Now().UTC().Add(-2 * time.Hour)
	if err := db.Model(&authpkg.Session{}).
		Where("token_hash = ?", tokenHash).
		Update("idle_expires_at", past).Error; err != nil {
		t.Fatalf("failed to expire session: %v", err)
	}

	// The next request should now be rejected as expired.
	w := makeRequest(t, router, http.MethodGet, "/api/v1/auth/me", token, nil)
	assertStatus(t, w, http.StatusUnauthorized)
}

// TestSessionAbsoluteExpiry verifies that a session with an already-expired absolute timeout
// returns 401.
func TestSessionAbsoluteExpiry(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	createTestUser(t, db, "henry", "Tenant")
	token := loginUser(t, router, "henry", "Password123!")

	tokenHash := authpkg.HashToken(token)

	// Backdate both expiry timestamps.
	past := time.Now().UTC().Add(-24 * time.Hour)
	if err := db.Model(&authpkg.Session{}).
		Where("token_hash = ?", tokenHash).
		Updates(map[string]interface{}{
			"absolute_expires_at": past,
			"idle_expires_at":     past,
		}).Error; err != nil {
		t.Fatalf("failed to expire session: %v", err)
	}

	w := makeRequest(t, router, http.MethodGet, "/api/v1/auth/me", token, nil)
	assertStatus(t, w, http.StatusUnauthorized)
}

// TestLogin_MissingFields verifies that missing required fields return HTTP 400.
func TestLogin_MissingFields(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	// Missing password field.
	w := makeRawRequest(t, router, http.MethodPost, "/api/v1/auth/login", "", `{"username":"alice"}`)
	if w.Code == http.StatusOK {
		t.Errorf("expected non-200 for missing password, got 200")
	}
}
