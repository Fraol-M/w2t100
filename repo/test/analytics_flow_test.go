package integration_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"gorm.io/gorm"

	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"github.com/gin-gonic/gin"
)

// analyticsEnv bundles the test dependencies for analytics tests.
type analyticsEnv struct {
	db     *gorm.DB
	cfg    *config.Config
	router *gin.Engine
}

func newAnalyticsEnv(t *testing.T) *analyticsEnv {
	t.Helper()
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)
	return &analyticsEnv{db: db, cfg: cfg, router: router}
}

// TestAnalytics_RoleGating verifies that analytics endpoints are accessible to
// PropertyManager and SystemAdmin, and return 403 for Tenant and Technician.
func TestAnalytics_RoleGating(t *testing.T) {
	env := newAnalyticsEnv(t)

	_, pmPw := createTestUser(t, env.db, "an_pm", common.RolePropertyManager)
	_, techPw := createTestUser(t, env.db, "an_tech", common.RoleTechnician)
	_, tenantPw := createTestUser(t, env.db, "an_tenant", common.RoleTenant)

	pmToken := loginUser(t, env.router, "an_pm", pmPw)
	techToken := loginUser(t, env.router, "an_tech", techPw)
	tenantToken := loginUser(t, env.router, "an_tenant", tenantPw)

	endpoints := []string{
		"/api/v1/analytics/popularity",
		"/api/v1/analytics/funnel",
		"/api/v1/analytics/retention",
		"/api/v1/analytics/tags",
		"/api/v1/analytics/quality",
	}

	for _, ep := range endpoints {
		t.Run("PM_can_access_"+ep, func(t *testing.T) {
			w := makeRequest(t, env.router, http.MethodGet, ep, pmToken, nil)
			assertStatus(t, w, http.StatusOK)
		})
		t.Run("Technician_forbidden_"+ep, func(t *testing.T) {
			w := makeRequest(t, env.router, http.MethodGet, ep, techToken, nil)
			assertStatus(t, w, http.StatusForbidden)
		})
		t.Run("Tenant_forbidden_"+ep, func(t *testing.T) {
			w := makeRequest(t, env.router, http.MethodGet, ep, tenantToken, nil)
			assertStatus(t, w, http.StatusForbidden)
		})
		t.Run("Unauthenticated_401_"+ep, func(t *testing.T) {
			w := makeRequest(t, env.router, http.MethodGet, ep, "", nil)
			assertStatus(t, w, http.StatusUnauthorized)
		})
	}
}

// TestAnalytics_PopularityShape verifies the popularity endpoint returns a JSON
// array (even if empty) when called with valid PM credentials.
func TestAnalytics_PopularityShape(t *testing.T) {
	env := newAnalyticsEnv(t)

	_, pmPw := createTestUser(t, env.db, "pop_pm", common.RolePropertyManager)
	pmToken := loginUser(t, env.router, "pop_pm", pmPw)

	w := makeRequest(t, env.router, http.MethodGet, "/api/v1/analytics/popularity", pmToken, nil)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	parseResponse(t, w, &resp)
	// Data must be a JSON array.
	if len(resp.Data) == 0 || resp.Data[0] != '[' {
		t.Errorf("expected data to be a JSON array, got: %s", resp.Data)
	}
}

// TestAnalytics_FunnelShape verifies the funnel endpoint returns an object with
// the expected keys.
func TestAnalytics_FunnelShape(t *testing.T) {
	env := newAnalyticsEnv(t)

	_, pmPw := createTestUser(t, env.db, "funnel_pm", common.RolePropertyManager)
	pmToken := loginUser(t, env.router, "funnel_pm", pmPw)

	w := makeRequest(t, env.router, http.MethodGet, "/api/v1/analytics/funnel", pmToken, nil)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Data struct {
			New        int64 `json:"new"`
			Assigned   int64 `json:"assigned"`
			InProgress int64 `json:"in_progress"`
			Completed  int64 `json:"completed"`
			Total      int64 `json:"total"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)
	// All counts must be >= 0 (DB is empty, so all zeros — shape check only).
	if resp.Data.Total < 0 {
		t.Errorf("unexpected negative total in funnel response")
	}
}

// TestAnalytics_RetentionShape verifies the retention endpoint returns an object
// with 30d and 90d window fields.
func TestAnalytics_RetentionShape(t *testing.T) {
	env := newAnalyticsEnv(t)

	_, pmPw := createTestUser(t, env.db, "ret_pm", common.RolePropertyManager)
	pmToken := loginUser(t, env.router, "ret_pm", pmPw)

	w := makeRequest(t, env.router, http.MethodGet, "/api/v1/analytics/retention", pmToken, nil)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Data struct {
			UniqueUnits30d int64   `json:"unique_units_30d"`
			RepeatUnits30d int64   `json:"repeat_units_30d"`
			RepeatRate30d  float64 `json:"repeat_rate_30d"`
			UniqueUnits90d int64   `json:"unique_units_90d"`
			RepeatRate90d  float64 `json:"repeat_rate_90d"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)
	if resp.Data.RepeatRate30d < 0 || resp.Data.RepeatRate30d > 1 {
		t.Errorf("RepeatRate30d out of range [0,1]: %f", resp.Data.RepeatRate30d)
	}
}

// TestAnalytics_QualityShape verifies the quality endpoint returns an object with
// rating aggregate fields.
func TestAnalytics_QualityShape(t *testing.T) {
	env := newAnalyticsEnv(t)

	_, pmPw := createTestUser(t, env.db, "qual_pm", common.RolePropertyManager)
	pmToken := loginUser(t, env.router, "qual_pm", pmPw)

	w := makeRequest(t, env.router, http.MethodGet, "/api/v1/analytics/quality", pmToken, nil)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Data struct {
			TotalRated    int64   `json:"total_rated"`
			AverageRating float64 `json:"average_rating"`
			NegativeRate  float64 `json:"negative_rate"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)
	if resp.Data.TotalRated < 0 {
		t.Errorf("TotalRated must be >= 0, got %d", resp.Data.TotalRated)
	}
}

// TestAnalytics_SavedReport_CRUD exercises the saved-report API endpoints:
// create, list, get, and delete — all scoped to the owning PM.
func TestAnalytics_SavedReport_CRUD(t *testing.T) {
	env := newAnalyticsEnv(t)

	_, pm1Pw := createTestUser(t, env.db, "sr_pm1", common.RolePropertyManager)
	_, pm2Pw := createTestUser(t, env.db, "sr_pm2", common.RolePropertyManager)
	pm1Token := loginUser(t, env.router, "sr_pm1", pm1Pw)
	pm2Token := loginUser(t, env.router, "sr_pm2", pm2Pw)

	// PM1 creates a saved report.
	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/analytics/reports/saved", pm1Token,
		map[string]interface{}{
			"name":          "PM1 Monthly Report",
			"report_type":   "work_orders",
			"output_format": "CSV",
			"schedule":      "daily",
		})
	assertStatus(t, w, http.StatusCreated)

	var createResp struct {
		Data struct {
			ID   uint64 `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	parseResponse(t, w, &createResp)
	if createResp.Data.ID == 0 {
		t.Fatal("expected non-zero saved report ID")
	}
	reportID := createResp.Data.ID

	// PM1 can get the report.
	w = makeRequest(t, env.router, http.MethodGet,
		"/api/v1/analytics/reports/saved/"+itoa(reportID), pm1Token, nil)
	assertStatus(t, w, http.StatusOK)

	// PM2 cannot get PM1's report (ownership check).
	w = makeRequest(t, env.router, http.MethodGet,
		"/api/v1/analytics/reports/saved/"+itoa(reportID), pm2Token, nil)
	if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
		t.Errorf("expected 403 or 404 for non-owner get, got %d", w.Code)
	}

	// PM1 lists saved reports — should include the one just created.
	w = makeRequest(t, env.router, http.MethodGet, "/api/v1/analytics/reports/saved", pm1Token, nil)
	assertStatus(t, w, http.StatusOK)
	var listResp struct {
		Data []struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	parseResponse(t, w, &listResp)
	if len(listResp.Data) == 0 {
		t.Error("expected at least one report in PM1's list")
	}

	// PM2 cannot delete PM1's report.
	w = makeRequest(t, env.router, http.MethodDelete,
		"/api/v1/analytics/reports/saved/"+itoa(reportID), pm2Token, nil)
	if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
		t.Errorf("expected 403 or 404 for non-owner delete, got %d", w.Code)
	}

	// PM1 can delete their own report.
	w = makeRequest(t, env.router, http.MethodDelete,
		"/api/v1/analytics/reports/saved/"+itoa(reportID), pm1Token, nil)
	assertStatus(t, w, http.StatusOK)

	// After deletion, get returns not-found.
	w = makeRequest(t, env.router, http.MethodGet,
		"/api/v1/analytics/reports/saved/"+itoa(reportID), pm1Token, nil)
	assertStatus(t, w, http.StatusNotFound)
}

// itoa converts a uint64 to its string representation (used for URL path building).
func itoa(n uint64) string {
	return strconv.FormatUint(n, 10)
}
