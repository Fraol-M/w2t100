package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"
)

// woEnv bundles DB + router for work-order tests.
type woEnv struct {
	db     *gorm.DB
	cfg    *config.Config
	router *gin.Engine
}

func newWOEnv(t *testing.T) *woEnv {
	t.Helper()
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)
	return &woEnv{db: db, cfg: cfg, router: router}
}

// TestWorkOrderLifecycle covers the full happy path:
// Tenant creates → SLA assigned → manager transitions to Assigned → Technician InProgress →
// → AwaitingApproval → Manager completes → Tenant rates.
func TestWorkOrderLifecycle(t *testing.T) {
	env := newWOEnv(t)

	tenantUser, tenantPw := createTestUser(t, env.db, "lc_tenant", common.RoleTenant)
	techUser, techPw := createTestUser(t, env.db, "lc_tech", common.RoleTechnician)
	managerUser, managerPw := createTestUser(t, env.db, "lc_manager", common.RolePropertyManager)

	// Manager must be assigned to the property for manager-owned transitions.
	assignManagerToProperty(t, env, managerUser.ID, 1)

	tenantToken := loginUser(t, env.router, "lc_tenant", tenantPw)
	techToken := loginUser(t, env.router, "lc_tech", techPw)
	managerToken := loginUser(t, env.router, "lc_manager", managerPw)

	// Step 1: Tenant creates a work order.
	createBody := map[string]interface{}{
		"property_id": 1,
		"description": "The kitchen tap is leaking and needs urgent repair by plumber.",
		"priority":    common.PriorityNormal,
		"issue_type":  "Plumbing",
	}
	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/work-orders", tenantToken, createBody)
	assertStatus(t, w, http.StatusCreated)

	var createResp struct {
		Data struct {
			ID       uint64  `json:"id"`
			Status   string  `json:"status"`
			SLADueAt *string `json:"sla_due_at"`
			TenantID uint64  `json:"tenant_id"`
		} `json:"data"`
	}
	parseResponse(t, w, &createResp)
	woID := createResp.Data.ID

	if woID == 0 {
		t.Fatalf("expected non-zero work order ID")
	}
	if createResp.Data.TenantID != tenantUser.ID {
		t.Errorf("expected tenant_id=%d, got %d", tenantUser.ID, createResp.Data.TenantID)
	}
	if createResp.Data.SLADueAt == nil || *createResp.Data.SLADueAt == "" {
		t.Errorf("expected sla_due_at to be set for Normal priority WO")
	}
	if createResp.Data.Status != common.WOStatusNew {
		t.Errorf("expected status=New, got %q", createResp.Data.Status)
	}

	// Step 2: Manager transitions New → Assigned.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusAssigned})
	assertStatus(t, w, http.StatusOK)

	// Assign the technician so technician-owned transitions are authorized.
	assignTechnicianToWO(t, env, woID, techUser.ID)

	// Step 3: Technician transitions Assigned → InProgress.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), techToken,
		map[string]string{"to_status": common.WOStatusInProgress})
	assertStatus(t, w, http.StatusOK)

	// Step 4: Technician transitions InProgress → AwaitingApproval.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), techToken,
		map[string]string{"to_status": common.WOStatusAwaitingApproval, "notes": "Work complete"})
	assertStatus(t, w, http.StatusOK)

	// Step 5: Manager completes the work order (AwaitingApproval → Completed).
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusCompleted})
	assertStatus(t, w, http.StatusOK)

	var completeResp struct {
		Data struct {
			Status      string  `json:"status"`
			CompletedAt *string `json:"completed_at"`
		} `json:"data"`
	}
	parseResponse(t, w, &completeResp)
	if completeResp.Data.Status != common.WOStatusCompleted {
		t.Errorf("expected status=Completed, got %q", completeResp.Data.Status)
	}
	if completeResp.Data.CompletedAt == nil {
		t.Errorf("expected completed_at to be set")
	}

	// Step 6: Tenant rates the completed work order.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/rate", woID), tenantToken,
		map[string]interface{}{"rating": 5, "feedback": "Excellent work!"})
	assertStatus(t, w, http.StatusOK)

	var rateResp struct {
		Data struct {
			Rating   *int    `json:"rating"`
			Feedback *string `json:"feedback"`
		} `json:"data"`
	}
	parseResponse(t, w, &rateResp)
	if rateResp.Data.Rating == nil || *rateResp.Data.Rating != 5 {
		t.Errorf("expected rating=5, got %v", rateResp.Data.Rating)
	}
}

// TestWorkOrderSLAAssigned verifies that an Emergency-priority WO gets sla_due_at set.
func TestWorkOrderSLAAssigned(t *testing.T) {
	env := newWOEnv(t)

	_, pw := createTestUser(t, env.db, "sla_tenant", common.RoleTenant)
	token := loginUser(t, env.router, "sla_tenant", pw)

	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/work-orders", token, map[string]interface{}{
		"property_id": 1,
		"description": "Gas leak — immediate attention required for safety of all residents.",
		"priority":    common.PriorityEmergency,
	})
	assertStatus(t, w, http.StatusCreated)

	var resp struct {
		Data struct {
			Priority string  `json:"priority"`
			SLADueAt *string `json:"sla_due_at"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)

	if resp.Data.SLADueAt == nil || *resp.Data.SLADueAt == "" {
		t.Errorf("expected sla_due_at to be set for Emergency priority")
	}
	if resp.Data.Priority != common.PriorityEmergency {
		t.Errorf("expected priority=Emergency, got %q", resp.Data.Priority)
	}
}

// TestWorkOrderInvalidTransition verifies that an illegal state transition
// (e.g. New → Completed) returns 422.
func TestWorkOrderInvalidTransition(t *testing.T) {
	env := newWOEnv(t)

	_, tenantPw := createTestUser(t, env.db, "inv_tenant", common.RoleTenant)
	_, adminPw := createTestUser(t, env.db, "inv_admin", common.RoleSystemAdmin)

	tenantToken := loginUser(t, env.router, "inv_tenant", tenantPw)
	adminToken := loginUser(t, env.router, "inv_admin", adminPw)

	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/work-orders", tenantToken, map[string]interface{}{
		"property_id": 1,
		"description": "Broken window latch needs replacement as soon as possible.",
		"priority":    common.PriorityNormal,
	})
	assertStatus(t, w, http.StatusCreated)

	var createResp struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	parseResponse(t, w, &createResp)
	woID := createResp.Data.ID

	// Attempt invalid transition: New → Completed.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), adminToken,
		map[string]string{"to_status": common.WOStatusCompleted})
	if w.Code != http.StatusUnprocessableEntity && w.Code != http.StatusBadRequest {
		t.Errorf("expected 422 or 400 for invalid transition, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestWorkOrderReassign_RequiresReason verifies that reassign without a reason returns 422,
// and with a valid reason returns 200.
func TestWorkOrderReassign_RequiresReason(t *testing.T) {
	env := newWOEnv(t)

	_, tenantPw := createTestUser(t, env.db, "re_tenant", common.RoleTenant)
	tech1User, tech1Pw := createTestUser(t, env.db, "re_tech1", common.RoleTechnician)
	tech2, _ := createTestUser(t, env.db, "re_tech2", common.RoleTechnician)
	managerUser, managerPw := createTestUser(t, env.db, "re_manager", common.RolePropertyManager)

	assignManagerToProperty(t, env, managerUser.ID, 1)

	tenantToken := loginUser(t, env.router, "re_tenant", tenantPw)
	tech1Token := loginUser(t, env.router, "re_tech1", tech1Pw)
	managerToken := loginUser(t, env.router, "re_manager", managerPw)

	// Create a WO and progress to InProgress.
	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/work-orders", tenantToken, map[string]interface{}{
		"property_id": 1,
		"description": "HVAC unit not cooling properly, needs immediate inspection today.",
		"priority":    common.PriorityHigh,
	})
	assertStatus(t, w, http.StatusCreated)
	var createResp struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	parseResponse(t, w, &createResp)
	woID := createResp.Data.ID

	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusAssigned})
	assertStatus(t, w, http.StatusOK)
	assignTechnicianToWO(t, env, woID, tech1User.ID)

	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), tech1Token,
		map[string]string{"to_status": common.WOStatusInProgress})
	assertStatus(t, w, http.StatusOK)

	// Reassign without reason → 422.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/reassign", woID), managerToken,
		map[string]interface{}{
			"technician_id": tech2.ID,
			"reason":        "",
		})
	if w.Code != http.StatusUnprocessableEntity && w.Code != http.StatusBadRequest {
		t.Errorf("expected 422/400 for empty reason, got %d; body: %s", w.Code, w.Body.String())
	}

	// Reassign with valid reason → 200.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/reassign", woID), managerToken,
		map[string]interface{}{
			"technician_id": tech2.ID,
			"reason":        "Original technician is sick and unavailable for this week assignment",
		})
	assertStatus(t, w, http.StatusOK)

	var reassignResp struct {
		Data struct {
			AssignedTo *uint64 `json:"assigned_to"`
		} `json:"data"`
	}
	parseResponse(t, w, &reassignResp)
	if reassignResp.Data.AssignedTo == nil || *reassignResp.Data.AssignedTo != tech2.ID {
		t.Errorf("expected assigned_to=%d, got %v", tech2.ID, reassignResp.Data.AssignedTo)
	}
}

// TestWorkOrderCrossPropertyAccess verifies that tenants can access WOs they own.
func TestWorkOrderCrossPropertyAccess(t *testing.T) {
	env := newWOEnv(t)

	_, pwA := createTestUser(t, env.db, "cross_tenantA", common.RoleTenant)
	_, pwB := createTestUser(t, env.db, "cross_tenantB", common.RoleTenant)

	tokenA := loginUser(t, env.router, "cross_tenantA", pwA)
	tokenB := loginUser(t, env.router, "cross_tenantB", pwB)

	// TenantA creates a WO on property 10.
	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/work-orders", tokenA, map[string]interface{}{
		"property_id": 10,
		"description": "Elevator is out of service and requires inspection by certified engineer.",
		"priority":    common.PriorityHigh,
	})
	assertStatus(t, w, http.StatusCreated)
	var createResp struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	parseResponse(t, w, &createResp)
	woID := createResp.Data.ID

	// TenantA can access their own WO.
	w = makeRequest(t, env.router, http.MethodGet,
		fmt.Sprintf("/api/v1/work-orders/%d", woID), tokenA, nil)
	assertStatus(t, w, http.StatusOK)

	// TenantB accessing the same WO — confirm no 500 (route is accessible; property
	// isolation is enforced at the application/filter level, not route level).
	w = makeRequest(t, env.router, http.MethodGet,
		fmt.Sprintf("/api/v1/work-orders/%d", woID), tokenB, nil)
	if w.Code == http.StatusInternalServerError {
		t.Errorf("unexpected 500 for cross-property GET; body: %s", w.Body.String())
	}
}

// TestWorkOrderCostItem verifies that a cost item can be added and appears in the list.
func TestWorkOrderCostItem(t *testing.T) {
	env := newWOEnv(t)

	_, tenantPw := createTestUser(t, env.db, "ci_tenant", common.RoleTenant)
	techUser, techPw := createTestUser(t, env.db, "ci_tech", common.RoleTechnician)
	managerUser, managerPw := createTestUser(t, env.db, "ci_manager", common.RolePropertyManager)

	assignManagerToProperty(t, env, managerUser.ID, 1)

	tenantToken := loginUser(t, env.router, "ci_tenant", tenantPw)
	techToken := loginUser(t, env.router, "ci_tech", techPw)
	managerToken := loginUser(t, env.router, "ci_manager", managerPw)

	// Create and progress WO to InProgress.
	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/work-orders", tenantToken, map[string]interface{}{
		"property_id": 1,
		"description": "Bathroom ceiling has water damage and requires repair and repainting.",
		"priority":    common.PriorityNormal,
	})
	assertStatus(t, w, http.StatusCreated)
	var createResp struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	parseResponse(t, w, &createResp)
	woID := createResp.Data.ID

	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusAssigned})
	assertStatus(t, w, http.StatusOK)
	assignTechnicianToWO(t, env, woID, techUser.ID)

	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), techToken,
		map[string]string{"to_status": common.WOStatusInProgress})
	assertStatus(t, w, http.StatusOK)

	// Add a Labor cost item.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/cost-items", woID), techToken,
		map[string]interface{}{
			"cost_type":      common.CostTypeLabor,
			"description":    "Two hours of plumbing labour at standard rate per contract",
			"amount":         150.00,
			"responsibility": common.ResponsibilityProperty,
		})
	assertStatus(t, w, http.StatusCreated)

	// List cost items.
	w = makeRequest(t, env.router, http.MethodGet,
		fmt.Sprintf("/api/v1/work-orders/%d/cost-items", woID), techToken, nil)
	assertStatus(t, w, http.StatusOK)

	var listResp struct {
		Data []struct {
			CostType string  `json:"cost_type"`
			Amount   float64 `json:"amount"`
		} `json:"data"`
	}
	parseResponse(t, w, &listResp)
	if len(listResp.Data) == 0 {
		t.Fatalf("expected at least one cost item, got none")
	}
	if listResp.Data[0].CostType != common.CostTypeLabor {
		t.Errorf("expected cost_type=Labor, got %q", listResp.Data[0].CostType)
	}
	if listResp.Data[0].Amount != 150.00 {
		t.Errorf("expected amount=150, got %v", listResp.Data[0].Amount)
	}
}

// TestWorkOrderCostItem_InvalidType verifies that an invalid cost_type returns 422.
func TestWorkOrderCostItem_InvalidType(t *testing.T) {
	env := newWOEnv(t)

	_, tenantPw := createTestUser(t, env.db, "ci2_tenant", common.RoleTenant)
	techUser, techPw := createTestUser(t, env.db, "ci2_tech", common.RoleTechnician)
	managerUser, managerPw := createTestUser(t, env.db, "ci2_manager", common.RolePropertyManager)

	assignManagerToProperty(t, env, managerUser.ID, 1)

	tenantToken := loginUser(t, env.router, "ci2_tenant", tenantPw)
	techToken := loginUser(t, env.router, "ci2_tech", techPw)
	managerToken := loginUser(t, env.router, "ci2_manager", managerPw)

	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/work-orders", tenantToken, map[string]interface{}{
		"property_id": 1,
		"description": "Faulty electrical socket in living room needs urgent replacement now.",
		"priority":    common.PriorityNormal,
	})
	assertStatus(t, w, http.StatusCreated)
	var cr struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	parseResponse(t, w, &cr)
	woID := cr.Data.ID

	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusAssigned})
	assertStatus(t, w, http.StatusOK)
	assignTechnicianToWO(t, env, woID, techUser.ID)

	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), techToken,
		map[string]string{"to_status": common.WOStatusInProgress})
	assertStatus(t, w, http.StatusOK)

	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/cost-items", woID), techToken,
		map[string]interface{}{
			"cost_type":      "InvalidType",
			"description":    "Some description that is long enough to pass validation checks",
			"amount":         50.0,
			"responsibility": common.ResponsibilityProperty,
		})
	if w.Code != http.StatusUnprocessableEntity && w.Code != http.StatusBadRequest {
		t.Errorf("expected 422/400 for invalid cost_type, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestWorkOrderRating_NotOwnTenant verifies that a different tenant cannot rate a WO.
func TestWorkOrderRating_NotOwnTenant(t *testing.T) {
	env := newWOEnv(t)

	_, pw1 := createTestUser(t, env.db, "rat_tenant1", common.RoleTenant)
	_, pw2 := createTestUser(t, env.db, "rat_tenant2", common.RoleTenant)
	managerUser, managerPw := createTestUser(t, env.db, "rat_manager", common.RolePropertyManager)
	techUser, techPw := createTestUser(t, env.db, "rat_tech", common.RoleTechnician)

	assignManagerToProperty(t, env, managerUser.ID, 1)

	token1 := loginUser(t, env.router, "rat_tenant1", pw1)
	token2 := loginUser(t, env.router, "rat_tenant2", pw2)
	managerToken := loginUser(t, env.router, "rat_manager", managerPw)
	techToken := loginUser(t, env.router, "rat_tech", techPw)

	// Tenant1 creates WO.
	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/work-orders", token1, map[string]interface{}{
		"property_id": 1,
		"description": "Broken door handle on front entrance requires immediate replacement.",
		"priority":    common.PriorityNormal,
	})
	assertStatus(t, w, http.StatusCreated)
	var cr struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	parseResponse(t, w, &cr)
	woID := cr.Data.ID

	// Progress to Completed.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusAssigned})
	assertStatus(t, w, http.StatusOK)
	assignTechnicianToWO(t, env, woID, techUser.ID)

	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), techToken,
		map[string]string{"to_status": common.WOStatusInProgress})
	assertStatus(t, w, http.StatusOK)

	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), techToken,
		map[string]string{"to_status": common.WOStatusAwaitingApproval})
	assertStatus(t, w, http.StatusOK)

	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusCompleted})
	assertStatus(t, w, http.StatusOK)

	// Tenant2 tries to rate → must be 403.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/rate", woID), token2,
		map[string]interface{}{"rating": 3})
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong tenant rating, got %d; body: %s", w.Code, w.Body.String())
	}
}

// assignManagerToProperty inserts a PropertyStaffAssignment so that IsManagedBy
// returns true for the given manager on the given property. This is required for
// any test that exercises manager transition/dispatch authorization.
func assignManagerToProperty(t *testing.T, env *woEnv, managerID, propertyID uint64) {
	t.Helper()
	if err := env.db.Exec(
		"INSERT INTO property_staff_assignments (property_id, user_id, role, is_active, created_at) VALUES (?, ?, 'PropertyManager', true, NOW())",
		propertyID, managerID,
	).Error; err != nil {
		t.Fatalf("assignManagerToProperty: %v", err)
	}
}

// assignTechnicianToWO directly sets the assigned_to field on a work order so that
// technician transition authorization (which checks assigned_to == userID) passes.
func assignTechnicianToWO(t *testing.T, env *woEnv, woID, techID uint64) {
	t.Helper()
	if err := env.db.Exec("UPDATE work_orders SET assigned_to = ?, status = 'Assigned' WHERE id = ?", techID, woID).Error; err != nil {
		t.Fatalf("assignTechnicianToWO: %v", err)
	}
}

// TestWorkOrder_PMForbiddenOnTechnicianTransitions asserts that a PropertyManager
// cannot perform transitions that are reserved for the assigned technician:
//   - Assigned → InProgress  (technician starts work)
//   - InProgress → AwaitingApproval  (technician submits for review)
//
// This enforces role semantics: PMs handle dispatch/approval, technicians handle
// execution. Before this fix, authorizeTransition allowed PMs to do any transition
// on their managed properties, which violated the stated role boundaries.
func TestWorkOrder_PMForbiddenOnTechnicianTransitions(t *testing.T) {
	env := newWOEnv(t)

	_, tenantPw := createTestUser(t, env.db, "pmfb_tenant", common.RoleTenant)
	techUser, techPw := createTestUser(t, env.db, "pmfb_tech", common.RoleTechnician)
	managerUser, managerPw := createTestUser(t, env.db, "pmfb_manager", common.RolePropertyManager)

	// Assign manager to property 1 so IsManagedBy passes.
	assignManagerToProperty(t, env, managerUser.ID, 1)

	tenantToken := loginUser(t, env.router, "pmfb_tenant", tenantPw)
	techToken := loginUser(t, env.router, "pmfb_tech", techPw)
	managerToken := loginUser(t, env.router, "pmfb_manager", managerPw)

	// Tenant creates WO → New.
	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/work-orders", tenantToken, map[string]interface{}{
		"property_id": 1,
		"description": "Roof gutter is blocked with debris and causing water damage to walls.",
		"priority":    common.PriorityNormal,
	})
	assertStatus(t, w, http.StatusCreated)
	var cr struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	parseResponse(t, w, &cr)
	woID := cr.Data.ID

	// Manager dispatches: New → Assigned (PM-owned transition — must succeed).
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusAssigned})
	assertStatus(t, w, http.StatusOK)

	// Directly wire up the technician so technician checks pass in subsequent steps.
	assignTechnicianToWO(t, env, woID, techUser.ID)

	// PM attempts technician-only transition: Assigned → InProgress.
	// Must be rejected (403 for authorization failure).
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusInProgress})
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for PM Assigned→InProgress, got %d; body: %s", w.Code, w.Body.String())
	}

	// Technician correctly advances: Assigned → InProgress.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), techToken,
		map[string]string{"to_status": common.WOStatusInProgress})
	assertStatus(t, w, http.StatusOK)

	// PM attempts technician-only transition: InProgress → AwaitingApproval.
	// Must be rejected (403 for authorization failure).
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusAwaitingApproval, "notes": "PM trying to submit for approval"})
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for PM InProgress→AwaitingApproval, got %d; body: %s", w.Code, w.Body.String())
	}

	// Technician correctly submits: InProgress → AwaitingApproval.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), techToken,
		map[string]string{"to_status": common.WOStatusAwaitingApproval, "notes": "Work is complete"})
	assertStatus(t, w, http.StatusOK)

	// PM correctly approves: AwaitingApproval → Completed (PM-owned — must succeed).
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusCompleted})
	assertStatus(t, w, http.StatusOK)
}

// TestWorkOrderEvents verifies that events are recorded throughout the lifecycle.
func TestWorkOrderEvents(t *testing.T) {
	env := newWOEnv(t)

	_, tenantPw := createTestUser(t, env.db, "ev_tenant", common.RoleTenant)
	managerUser, managerPw := createTestUser(t, env.db, "ev_manager", common.RolePropertyManager)

	assignManagerToProperty(t, env, managerUser.ID, 1)

	tenantToken := loginUser(t, env.router, "ev_tenant", tenantPw)
	managerToken := loginUser(t, env.router, "ev_manager", managerPw)

	w := makeRequest(t, env.router, http.MethodPost, "/api/v1/work-orders", tenantToken, map[string]interface{}{
		"property_id": 1,
		"description": "Air conditioning unit making loud noise and not cooling effectively.",
		"priority":    common.PriorityNormal,
	})
	assertStatus(t, w, http.StatusCreated)
	var cr struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	parseResponse(t, w, &cr)
	woID := cr.Data.ID

	// Transition once.
	w = makeRequest(t, env.router, http.MethodPost,
		fmt.Sprintf("/api/v1/work-orders/%d/transition", woID), managerToken,
		map[string]string{"to_status": common.WOStatusAssigned})
	assertStatus(t, w, http.StatusOK)

	// Get events.
	w = makeRequest(t, env.router, http.MethodGet,
		fmt.Sprintf("/api/v1/work-orders/%d/events", woID), tenantToken, nil)
	assertStatus(t, w, http.StatusOK)

	var eventsResp struct {
		Data []json.RawMessage `json:"data"`
	}
	parseResponse(t, w, &eventsResp)

	// Should have at least the "created" event plus the transition event.
	if len(eventsResp.Data) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(eventsResp.Data))
	}
}
