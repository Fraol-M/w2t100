package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"propertyops/backend/internal/app"
	"propertyops/backend/internal/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// testConfig returns a minimal Config suitable for integration tests.
// It does not require any real DB, encryption keys, or storage paths.
func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Port: 0, GinMode: "test"},
		DB:     config.DBConfig{},
		Auth: config.AuthConfig{
			BcryptCost:         4, // minimum cost for fast tests
			SessionIdleTimeout: 30 * time.Minute,
			SessionMaxLifetime: 168 * time.Hour,
		},
		Encryption: config.EncryptionConfig{
			KeyDir:      ".",
			ActiveKeyID: 1,
		},
		Storage: config.StorageConfig{
			Root:       ".",
			BackupRoot: ".",
			LogRoot:    ".",
		},
		Backup: config.BackupConfig{
			ScheduleCron:      "0 2 * * *",
			RetentionDays:     30,
			EncryptionEnabled: false,
		},
		RateLimit: config.RateLimitConfig{
			MaxSubmissionsPerHour: 1000, // generous limit so tests don't hit it
		},
		Payment: config.PaymentConfig{
			IntentExpiryMinutes:   30,
			DualApprovalThreshold: 500.00,
		},
		Anomaly: config.AnomalyConfig{
			AllowedCIDRs: []string{"127.0.0.0/8", "0.0.0.0/0"},
		},
	}
}

// newTestRouter builds a full Gin engine backed by the provided SQLite DB.
// All routes and middleware are registered exactly as in production via app.RegisterRoutes.
func newTestRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	engine := gin.New()
	app.RegisterRoutes(engine, db, cfg)
	return engine
}

// makeRequest performs an HTTP request against the provided router and returns the recorder.
// token is the raw bearer token; pass "" to omit the Authorization header.
// body may be nil for requests without a body.
func makeRequest(t *testing.T, router *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("makeRequest: marshal body: %v", err)
		}
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req, err := http.NewRequest(method, path, reqBody)
	if err != nil {
		t.Fatalf("makeRequest: new request: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// makeRawRequest performs an HTTP request with a pre-encoded JSON string body.
func makeRawRequest(t *testing.T, router *gin.Engine, method, path, token, rawBody string) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader *strings.Reader
	if rawBody != "" {
		bodyReader = strings.NewReader(rawBody)
	} else {
		bodyReader = strings.NewReader("")
	}

	req, err := http.NewRequest(method, path, bodyReader)
	if err != nil {
		t.Fatalf("makeRawRequest: %v", err)
	}
	if rawBody != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// loginUser logs in a user via the API and returns the raw bearer token.
// Fails the test if the login does not return 200 or does not include a token.
func loginUser(t *testing.T, router *gin.Engine, username, password string) string {
	t.Helper()

	w := makeRequest(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": username,
		"password": password,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("loginUser: expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("loginUser: parse response: %v", err)
	}
	if resp.Data.Token == "" {
		t.Fatalf("loginUser: got empty token; body: %s", w.Body.String())
	}
	return resp.Data.Token
}

// parseResponse unmarshals the JSON response body into target.
// It fails the test if unmarshalling fails.
func parseResponse(t *testing.T, w *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), target); err != nil {
		t.Fatalf("parseResponse: %v; body: %s", err, w.Body.String())
	}
}

// assertStatus fails the test if the recorder's status code does not match expected.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Errorf("expected HTTP %d, got %d; body: %s", expected, w.Code, w.Body.String())
	}
}
