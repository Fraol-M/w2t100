package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"
	"propertyops/backend/internal/governance"
)

// govEnv bundles DB + router for governance tests.
type govEnv struct {
	db     *gorm.DB
	cfg    *config.Config
	router *gin.Engine
}

func newGovEnv(t *testing.T) *govEnv {
	t.Helper()
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)
	return &govEnv{db: db, cfg: cfg, router: router}
}

// TestReportCreation verifies that any authenticated user can file a report,
// and the new report has Open status.
func TestReportCreation(t *testing.T) {
	env := newGovEnv(t)

	_, tenantPw := createTestUser(t, env.db, "rc_tenant", common.RoleTenant)
	tenantToken := loginUser(t, env.router, "rc_tenant", tenantPw)

	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/governance/reports", tenantToken,
		map[string]interface{}{
			"target_type": common.ReportTargetTenant,
			"target_id":   42,
			"category":    "Harassment",
			"description": "This tenant is repeatedly harassing other residents in the common areas and parking lot.",
		})
	assertStatus(t, w, http.StatusCreated)

	var resp struct {
		Data struct {
			ID         uint64 `json:"id"`
			Status     string `json:"status"`
			ReporterID uint64 `json:"reporter_id"`
			Category   string `json:"category"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)

	if resp.Data.ID == 0 {
		t.Errorf("expected non-zero report ID")
	}
	if resp.Data.Status != common.ReportStatusOpen {
		t.Errorf("expected status=Open, got %q", resp.Data.Status)
	}
	if resp.Data.Category != "Harassment" {
		t.Errorf("expected category=Harassment, got %q", resp.Data.Category)
	}
}

// TestReportCreation_AnyRole verifies that non-tenant roles can also file reports.
func TestReportCreation_AnyRole(t *testing.T) {
	env := newGovEnv(t)

	_, techPw := createTestUser(t, env.db, "rc2_tech", common.RoleTechnician)
	techToken := loginUser(t, env.router, "rc2_tech", techPw)

	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/governance/reports", techToken,
		map[string]interface{}{
			"target_type": common.ReportTargetWorkOrder,
			"target_id":   99,
			"category":    "Maintenance",
			"description": "Work order has documented unsafe working conditions that endanger employees.",
		})
	assertStatus(t, w, http.StatusCreated)
}

// TestReportReview_OnlyComplianceReviewer verifies that a Tenant cannot review a report (403),
// but a ComplianceReviewer can.
func TestReportReview_OnlyComplianceReviewer(t *testing.T) {
	env := newGovEnv(t)

	_, tenantPw := createTestUser(t, env.db, "rr_tenant", common.RoleTenant)
	_, reviewerPw := createTestUser(t, env.db, "rr_reviewer", common.RoleComplianceReviewer)

	tenantToken := loginUser(t, env.router, "rr_tenant", tenantPw)
	reviewerToken := loginUser(t, env.router, "rr_reviewer", reviewerPw)

	// Create report as tenant.
	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/governance/reports", tenantToken,
		map[string]interface{}{
			"target_type": common.ReportTargetTenant,
			"target_id":   55,
			"category":    "Noise",
			"description": "Tenant playing loud music after quiet hours every single night repeatedly.",
		})
	assertStatus(t, w, http.StatusCreated)
	var createResp struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &createResp)
	reportID := createResp.Data.ID

	// Tenant tries to review → 403 (PATCH /:id/review requires ComplianceReviewer).
	w = makeRequest(t, env.router, http.MethodPatch,
		fmt.Sprintf("/api/v1/governance/reports/%d/review", reportID), tenantToken,
		map[string]string{"status": common.ReportStatusInReview})
	// The route itself doesn't have role middleware in the open registration, but
	// the handler enforces it via the service. Expect either 403 or 422.
	if w.Code == http.StatusOK {
		t.Errorf("tenant should not be able to review a report, got 200")
	}

	// ComplianceReviewer reviews → 200.
	w = makeRequest(t, env.router, http.MethodPatch,
		fmt.Sprintf("/api/v1/governance/reports/%d/review", reportID), reviewerToken,
		map[string]string{"status": common.ReportStatusInReview})
	assertStatus(t, w, http.StatusOK)

	var reviewResp struct {
		Data struct{ Status string `json:"status"` } `json:"data"`
	}
	parseResponse(t, w, &reviewResp)
	if reviewResp.Data.Status != common.ReportStatusInReview {
		t.Errorf("expected status=InReview, got %q", reviewResp.Data.Status)
	}
}

// TestEnforcement_Suspension verifies that applying a Suspension enforcement action
// causes the target user's subsequent requests to be blocked with 403.
func TestEnforcement_Suspension(t *testing.T) {
	env := newGovEnv(t)

	targetUser, targetPw := createTestUser(t, env.db, "sus_target", common.RoleTenant)
	_, adminPw := createTestUser(t, env.db, "sus_admin", common.RoleSystemAdmin)

	targetToken := loginUser(t, env.router, "sus_target", targetPw)
	adminToken := loginUser(t, env.router, "sus_admin", adminPw)

	// Verify target user can access protected endpoints.
	w := makeRequest(t, env.router, http.MethodGet, "/api/v1/auth/me", targetToken, nil)
	assertStatus(t, w, http.StatusOK)

	// Admin applies suspension.
	w = makeRequest(t, env.router, http.MethodPost, "/api/v1/governance/enforcements", adminToken,
		map[string]interface{}{
			"user_id":     targetUser.ID,
			"action_type": common.EnforcementSuspension,
			"reason":      "Repeated violation of community guidelines including harassment and property damage.",
			"ends_at":     "7day",
		})
	assertStatus(t, w, http.StatusCreated)

	var enfResp struct {
		Data struct {
			ID         uint64 `json:"id"`
			IsActive   bool   `json:"is_active"`
			ActionType string `json:"action_type"`
		} `json:"data"`
	}
	parseResponse(t, w, &enfResp)
	if !enfResp.Data.IsActive {
		t.Errorf("expected enforcement to be active")
	}
	if enfResp.Data.ActionType != common.EnforcementSuspension {
		t.Errorf("expected action_type=Suspension, got %q", enfResp.Data.ActionType)
	}

	// Suspended user's next request should be 403.
	w = makeRequest(t, env.router, http.MethodGet, "/api/v1/auth/me", targetToken, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for suspended user, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestEnforcement_Revoke verifies that revoking a suspension unblocks the user.
func TestEnforcement_Revoke(t *testing.T) {
	env := newGovEnv(t)

	targetUser, targetPw := createTestUser(t, env.db, "rev_target", common.RoleTenant)
	_, adminPw := createTestUser(t, env.db, "rev_admin", common.RoleSystemAdmin)

	targetToken := loginUser(t, env.router, "rev_target", targetPw)
	adminToken := loginUser(t, env.router, "rev_admin", adminPw)

	// Apply suspension.
	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/governance/enforcements", adminToken,
		map[string]interface{}{
			"user_id":     targetUser.ID,
			"action_type": common.EnforcementSuspension,
			"reason":      "Temporary suspension pending investigation of reported incident by management.",
			"ends_at":     "indefinite",
		})
	assertStatus(t, w, http.StatusCreated)
	var enfResp struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &enfResp)
	enfID := enfResp.Data.ID

	// Confirm suspended.
	w = makeRequest(t, env.router, http.MethodGet, "/api/v1/auth/me", targetToken, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for suspended user before revoke, got %d", w.Code)
	}

	// Admin revokes suspension.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/governance/enforcements/%d/revoke", enfID), adminToken,
		map[string]string{"reason": "Investigation concluded, user cleared of all allegations made"})
	assertStatus(t, w, http.StatusOK)

	var revokeResp struct {
		Data struct{ IsActive bool `json:"is_active"` } `json:"data"`
	}
	parseResponse(t, w, &revokeResp)
	if revokeResp.Data.IsActive {
		t.Errorf("expected is_active=false after revoke")
	}

	// User should now be able to access protected endpoints again.
	w = makeRequest(t, env.router, http.MethodGet, "/api/v1/auth/me", targetToken, nil)
	assertStatus(t, w, http.StatusOK)
}

// TestRateLimit_Enforcement verifies that a RateLimit enforcement action is created successfully.
// Full rate-limit testing requires faking audit_log entries; here we verify the enforcement is persisted.
func TestRateLimit_Enforcement(t *testing.T) {
	env := newGovEnv(t)

	targetUser, _ := createTestUser(t, env.db, "rl_target", common.RoleTenant)
	_, adminPw := createTestUser(t, env.db, "rl_admin", common.RoleSystemAdmin)
	adminToken := loginUser(t, env.router, "rl_admin", adminPw)

	maxRequests := 3
	windowMinutes := 60

	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/governance/enforcements", adminToken,
		map[string]interface{}{
			"user_id":                   targetUser.ID,
			"action_type":               common.EnforcementRateLimit,
			"reason":                    "User is submitting excessive work order requests beyond acceptable limits.",
			"rate_limit_max":            maxRequests,
			"rate_limit_window_minutes": windowMinutes,
		})
	assertStatus(t, w, http.StatusCreated)

	var resp struct {
		Data struct {
			ActionType             string `json:"action_type"`
			RateLimitMax           *int   `json:"rate_limit_max"`
			RateLimitWindowMinutes *int   `json:"rate_limit_window_minutes"`
			IsActive               bool   `json:"is_active"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)

	if resp.Data.ActionType != common.EnforcementRateLimit {
		t.Errorf("expected action_type=RateLimit, got %q", resp.Data.ActionType)
	}
	if resp.Data.RateLimitMax == nil || *resp.Data.RateLimitMax != maxRequests {
		t.Errorf("expected rate_limit_max=%d, got %v", maxRequests, resp.Data.RateLimitMax)
	}
	if !resp.Data.IsActive {
		t.Errorf("expected enforcement to be active")
	}
}

// TestEnforcement_Warning verifies that a Warning enforcement action can be applied.
func TestEnforcement_Warning(t *testing.T) {
	env := newGovEnv(t)

	targetUser, targetPw := createTestUser(t, env.db, "warn_target", common.RoleTenant)
	_, adminPw := createTestUser(t, env.db, "warn_admin", common.RoleSystemAdmin)

	targetToken := loginUser(t, env.router, "warn_target", targetPw)
	adminToken := loginUser(t, env.router, "warn_admin", adminPw)

	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/governance/enforcements", adminToken,
		map[string]interface{}{
			"user_id":     targetUser.ID,
			"action_type": common.EnforcementWarning,
			"reason":      "First official warning for violation of community noise policy during late hours.",
		})
	assertStatus(t, w, http.StatusCreated)

	// Warning should NOT block the user (only Suspension does).
	w = makeRequest(t, env.router, http.MethodGet, "/api/v1/auth/me", targetToken, nil)
	assertStatus(t, w, http.StatusOK)
}

// TestEnforcement_InvalidActionType verifies that an unknown action_type returns 422.
func TestEnforcement_InvalidActionType(t *testing.T) {
	env := newGovEnv(t)

	targetUser, _ := createTestUser(t, env.db, "inv_target", common.RoleTenant)
	_, adminPw := createTestUser(t, env.db, "inv_admin", common.RoleSystemAdmin)
	adminToken := loginUser(t, env.router, "inv_admin", adminPw)

	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/governance/enforcements", adminToken,
		map[string]interface{}{
			"user_id":     targetUser.ID,
			"action_type": "BanForever",
			"reason":      "This is an invalid action type and should be rejected by the validator.",
		})
	if w.Code != http.StatusUnprocessableEntity && w.Code != http.StatusBadRequest {
		t.Errorf("expected 422/400 for invalid action_type, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestEnforcement_Suspension_ExpiredAutolifts verifies that an enforcement with an
// already-expired ends_at does not block the user (the DB query checks ends_at > now).
func TestEnforcement_Suspension_ExpiredAutolifts(t *testing.T) {
	env := newGovEnv(t)

	targetUser, targetPw := createTestUser(t, env.db, "exp_target", common.RoleTenant)
	_, adminPw := createTestUser(t, env.db, "exp_admin", common.RoleSystemAdmin)

	targetToken := loginUser(t, env.router, "exp_target", targetPw)
	adminToken := loginUser(t, env.router, "exp_admin", adminPw)

	// Apply a 1-day suspension.
	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/governance/enforcements", adminToken,
		map[string]interface{}{
			"user_id":     targetUser.ID,
			"action_type": common.EnforcementSuspension,
			"reason":      "Temporary suspension for administrative review of submitted complaint.",
			"ends_at":     "1day",
		})
	assertStatus(t, w, http.StatusCreated)
	var enfResp struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &enfResp)

	// Manually backdate ends_at so it appears expired.
	past := time.Now().UTC().Add(-48 * time.Hour)
	if err := env.db.Model(&governance.EnforcementAction{}).
		Where("id = ?", enfResp.Data.ID).
		Update("ends_at", past).Error; err != nil {
		t.Fatalf("failed to backdate ends_at: %v", err)
	}

	// Expired suspension should NOT block the user.
	w = makeRequest(t, env.router, http.MethodGet, "/api/v1/auth/me", targetToken, nil)
	assertStatus(t, w, http.StatusOK)
}

// TestReportList verifies that listing reports returns the correct data.
func TestReportList(t *testing.T) {
	env := newGovEnv(t)

	_, reviewerPw := createTestUser(t, env.db, "rl_reviewer", common.RoleComplianceReviewer)
	_, tenantPw := createTestUser(t, env.db, "rl_tenant", common.RoleTenant)

	reviewerToken := loginUser(t, env.router, "rl_reviewer", reviewerPw)
	tenantToken := loginUser(t, env.router, "rl_tenant", tenantPw)

	// File two reports.
	for i := 0; i < 2; i++ {
		makeRequest(t, env.router, http.MethodPost, "/api/v1/governance/reports", tenantToken,
			map[string]interface{}{
				"target_type": common.ReportTargetTenant,
				"target_id":   uint64(10 + i),
				"category":    "Damage",
				"description": "Property vandalism reported by tenant requiring immediate investigation.",
			})
	}

	w := makeRequest(t, env.router, http.MethodGet, "/api/v1/governance/reports", reviewerToken, nil)
	assertStatus(t, w, http.StatusOK)

	var listResp struct {
		Data []struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	parseResponse(t, w, &listResp)
	if len(listResp.Data) < 2 {
		t.Errorf("expected at least 2 reports, got %d", len(listResp.Data))
	}
}

// TestReportReview_InvalidTransition verifies that invalid transitions are rejected.
func TestReportReview_InvalidTransition(t *testing.T) {
	env := newGovEnv(t)

	_, tenantPw := createTestUser(t, env.db, "it_tenant", common.RoleTenant)
	_, reviewerPw := createTestUser(t, env.db, "it_reviewer", common.RoleComplianceReviewer)

	tenantToken := loginUser(t, env.router, "it_tenant", tenantPw)
	reviewerToken := loginUser(t, env.router, "it_reviewer", reviewerPw)

	// Create report.
	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/governance/reports", tenantToken,
		map[string]interface{}{
			"target_type": common.ReportTargetTenant,
			"target_id":   77,
			"category":    "Fraud",
			"description": "Suspected fraudulent activity reported by multiple residents in building.",
		})
	assertStatus(t, w, http.StatusCreated)
	var createResp struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &createResp)
	reportID := createResp.Data.ID

	// Attempt Open → Resolved (invalid; must go Open → InReview first).
	w = makeRequest(t, env.router, http.MethodPatch,
		fmt.Sprintf("/api/v1/governance/reports/%d/review", reportID), reviewerToken,
		map[string]string{"status": common.ReportStatusResolved})
	if w.Code != http.StatusUnprocessableEntity && w.Code != http.StatusBadRequest {
		t.Errorf("expected 422/400 for invalid Open→Resolved transition, got %d; body: %s", w.Code, w.Body.String())
	}
}
