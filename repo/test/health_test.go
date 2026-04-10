package integration_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestHealthLive verifies GET /health/live returns 200 with status="ok".
// This endpoint has no auth and no dependency checks — it just proves the
// process is up.
func TestHealthLive(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	w := makeRequest(t, router, http.MethodGet, "/health/live", "", nil)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse /health/live response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status=ok, got %q", resp.Status)
	}
}

// TestHealthReady_AllChecksPass verifies GET /health/ready returns 200 when
// the DB is reachable and the configured directories exist.
// testConfig() sets Storage.Root, Storage.BackupRoot, and Encryption.KeyDir
// all to "." — which always exists in the test working directory.
func TestHealthReady_AllChecksPass(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	w := makeRequest(t, router, http.MethodGet, "/health/ready", "", nil)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Status string                     `json:"status"`
		Checks map[string]json.RawMessage `json:"checks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse /health/ready response: %v; body: %s", err, w.Body.String())
	}
	if resp.Status != "ok" {
		t.Errorf("expected overall status=ok, got %q; body: %s", resp.Status, w.Body.String())
	}

	// Verify all expected sub-checks are present.
	for _, key := range []string{"db", "storage", "keys", "backup_dir"} {
		if _, ok := resp.Checks[key]; !ok {
			t.Errorf("expected check %q to be present in /health/ready response", key)
		}
	}

	// DB check must be "ok" (we have a live DB in the test).
	var dbCheck struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Checks["db"], &dbCheck); err != nil {
		t.Fatalf("parse db check: %v", err)
	}
	if dbCheck.Status != "ok" {
		t.Errorf("expected db check status=ok, got %q", dbCheck.Status)
	}
}

// TestHealthLive_NoAuthRequired verifies that /health/live is publicly accessible
// without any bearer token.
func TestHealthLive_NoAuthRequired(t *testing.T) {
	db := setupTestDB(t)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	// Deliberately pass no token — must still get 200.
	w := makeRequest(t, router, http.MethodGet, "/health/live", "", nil)
	if w.Code != http.StatusOK {
		t.Errorf("/health/live must be public (no auth), got %d", w.Code)
	}
}

// TestHealthReady_NoAuthRequired verifies that /health/ready is publicly accessible.
func TestHealthReady_NoAuthRequired(t *testing.T) {
	db := setupTestDB(t)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	w := makeRequest(t, router, http.MethodGet, "/health/ready", "", nil)
	// 200 (all pass) or 503 (some dep unavailable) — but never 401.
	if w.Code == http.StatusUnauthorized {
		t.Errorf("/health/ready must be public (no auth), got 401")
	}
}
