package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"propertyops/backend/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupTestDB opens an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("SQLite unavailable (CGO disabled or no C compiler): %v", err)
	}
	return db
}

// buildTestConfig returns a Config with valid temporary directories.
func buildTestConfig(t *testing.T) *config.Config {
	t.Helper()
	storageDir := t.TempDir()
	backupDir := t.TempDir()
	keyDir := t.TempDir()

	return &config.Config{
		Storage: config.StorageConfig{
			Root:       storageDir,
			BackupRoot: backupDir,
			LogRoot:    t.TempDir(),
		},
		Encryption: config.EncryptionConfig{
			KeyDir:       keyDir,
			ActiveKeyID:  1,
			RotationDays: 180,
		},
	}
}

// --- Liveness ---

func TestLiveness_AlwaysOK(t *testing.T) {
	r := gin.New()
	db := setupTestDB(t)
	cfg := buildTestConfig(t)
	RegisterRoutes(r, db, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health/live", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp LivenessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status=ok, got %q", resp.Status)
	}
}

// --- Readiness: all checks pass ---

func TestReadiness_AllChecksPass(t *testing.T) {
	r := gin.New()
	db := setupTestDB(t)
	cfg := buildTestConfig(t)
	RegisterRoutes(r, db, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp ReadinessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status=ok, got %q", resp.Status)
	}
	for name, check := range resp.Checks {
		if check.Status != "ok" {
			t.Errorf("check %q: expected ok, got %q (%s)", name, check.Status, check.Message)
		}
	}
}

// --- Readiness: storage dir missing ---

func TestReadiness_StorageDirMissing_503(t *testing.T) {
	r := gin.New()
	db := setupTestDB(t)
	cfg := buildTestConfig(t)

	// Point storage root at a non-existent path.
	cfg.Storage.Root = "/nonexistent/path/that/does/not/exist"

	RegisterRoutes(r, db, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var resp ReadinessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "degraded" {
		t.Errorf("expected status=degraded, got %q", resp.Status)
	}
	if resp.Checks["storage"].Status != "fail" {
		t.Errorf("expected storage check to fail, got %q", resp.Checks["storage"].Status)
	}
}

// --- Readiness: key dir missing ---

func TestReadiness_KeyDirMissing_503(t *testing.T) {
	r := gin.New()
	db := setupTestDB(t)
	cfg := buildTestConfig(t)

	cfg.Encryption.KeyDir = "/nonexistent/keydir"

	RegisterRoutes(r, db, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var resp ReadinessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Checks["keys"].Status != "fail" {
		t.Errorf("expected keys check to fail, got %q", resp.Checks["keys"].Status)
	}
}

// --- Readiness: backup dir missing ---

func TestReadiness_BackupDirMissing_503(t *testing.T) {
	r := gin.New()
	db := setupTestDB(t)
	cfg := buildTestConfig(t)

	// Remove backup dir after creating config so it exists in config but not on disk.
	if err := os.Remove(cfg.Storage.BackupRoot); err != nil {
		// TempDir creates a dir, not a file; remove the dir.
		os.RemoveAll(cfg.Storage.BackupRoot)
	}
	cfg.Storage.BackupRoot = "/nonexistent/backups"

	RegisterRoutes(r, db, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var resp ReadinessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Checks["backup_dir"].Status != "fail" {
		t.Errorf("expected backup_dir check to fail, got %q", resp.Checks["backup_dir"].Status)
	}
}

// --- Check helpers ---

func TestCheckDir_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	check := checkDir("test", dir)
	if check.Status != "ok" {
		t.Errorf("expected ok for existing dir, got %q: %s", check.Status, check.Message)
	}
}

func TestCheckDir_MissingDir(t *testing.T) {
	check := checkDir("test", "/nonexistent/dir")
	if check.Status != "fail" {
		t.Errorf("expected fail for missing dir, got %q", check.Status)
	}
}

func TestCheckDir_EmptyPath(t *testing.T) {
	check := checkDir("test", "")
	if check.Status != "fail" {
		t.Errorf("expected fail for empty path, got %q", check.Status)
	}
}

// --- Readiness response structure ---

func TestReadiness_ResponseHasAllChecks(t *testing.T) {
	r := gin.New()
	db := setupTestDB(t)
	cfg := buildTestConfig(t)
	RegisterRoutes(r, db, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
	r.ServeHTTP(w, req)

	var resp ReadinessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	for _, name := range []string{"db", "storage", "keys", "backup_dir"} {
		if _, ok := resp.Checks[name]; !ok {
			t.Errorf("expected check %q to be present in readiness response", name)
		}
	}
}
