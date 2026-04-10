package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"propertyops/backend/internal/common"
)

// ---------------------------------------------------------------------------
// User management tests
// ---------------------------------------------------------------------------

// TestUser_GetOwnProfile verifies that a user can GET their own profile.
func TestUser_GetOwnProfile(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	user, pw := createTestUser(t, db, "gown_user", common.RoleTenant)
	token := loginUser(t, router, "gown_user", pw)

	w := makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/users/%d", user.ID), token, nil)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Data struct {
			ID       uint64 `json:"id"`
			Username string `json:"username"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)
	if resp.Data.ID != user.ID {
		t.Errorf("expected user id=%d, got %d", user.ID, resp.Data.ID)
	}
	if resp.Data.Username != "gown_user" {
		t.Errorf("expected username=gown_user, got %q", resp.Data.Username)
	}
}

// TestUser_GetOtherProfile_Forbidden verifies that a non-admin user cannot
// retrieve another user's profile.
func TestUser_GetOtherProfile_Forbidden(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	target, _ := createTestUser(t, db, "gother_target", common.RoleTenant)
	_, attackerPw := createTestUser(t, db, "gother_attacker", common.RoleTenant)
	attackerToken := loginUser(t, router, "gother_attacker", attackerPw)

	w := makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/users/%d", target.ID), attackerToken, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-user GET, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestUser_AdminCanGetAnyProfile verifies that a SystemAdmin can GET any user's profile.
func TestUser_AdminCanGetAnyProfile(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	target, _ := createTestUser(t, db, "gadmin_target", common.RoleTenant)
	_, adminPw := createTestUser(t, db, "gadmin_admin", common.RoleSystemAdmin)
	adminToken := loginUser(t, router, "gadmin_admin", adminPw)

	w := makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/users/%d", target.ID), adminToken, nil)
	assertStatus(t, w, http.StatusOK)
}

// TestUser_UpdateOwnProfile verifies that a user can update their own profile fields.
func TestUser_UpdateOwnProfile(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	user, pw := createTestUser(t, db, "upd_user", common.RoleTenant)
	token := loginUser(t, router, "upd_user", pw)

	w := makeRequest(t, router, http.MethodPut,
		fmt.Sprintf("/api/v1/users/%d", user.ID), token, map[string]interface{}{
			"first_name": "Updated",
			"last_name":  "Name",
			"email":      "upd_user_new@test.local",
		})
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Data struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)
	if resp.Data.FirstName != "Updated" {
		t.Errorf("expected first_name=Updated, got %q", resp.Data.FirstName)
	}
	if resp.Data.LastName != "Name" {
		t.Errorf("expected last_name=Name, got %q", resp.Data.LastName)
	}
}

// TestUser_UpdateOtherProfile_Forbidden verifies that a Tenant cannot update another
// user's profile via PUT /users/:id.
func TestUser_UpdateOtherProfile_Forbidden(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	target, _ := createTestUser(t, db, "updoth_target", common.RoleTenant)
	_, attackerPw := createTestUser(t, db, "updoth_attacker", common.RoleTenant)
	attackerToken := loginUser(t, router, "updoth_attacker", attackerPw)

	w := makeRequest(t, router, http.MethodPut,
		fmt.Sprintf("/api/v1/users/%d", target.ID), attackerToken, map[string]interface{}{
			"first_name": "Hacked",
		})
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-user PUT, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestUser_ToggleActive verifies that a SystemAdmin can deactivate and reactivate a user.
func TestUser_ToggleActive(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	user, userPw := createTestUser(t, db, "toggle_user", common.RoleTenant)
	_, adminPw := createTestUser(t, db, "toggle_admin", common.RoleSystemAdmin)
	userToken := loginUser(t, router, "toggle_user", userPw)
	adminToken := loginUser(t, router, "toggle_admin", adminPw)

	// User is active — can access /me.
	w := makeRequest(t, router, http.MethodGet, "/api/v1/auth/me", userToken, nil)
	assertStatus(t, w, http.StatusOK)

	// Admin deactivates the user.
	w = makeRequest(t, router, http.MethodPatch,
		fmt.Sprintf("/api/v1/users/%d/active", user.ID), adminToken,
		map[string]interface{}{"is_active": false})
	assertStatus(t, w, http.StatusOK)

	// Deactivated user cannot login.
	w = makeRequest(t, router, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": "toggle_user", "password": userPw})
	if w.Code == http.StatusOK {
		t.Errorf("deactivated user should not be able to login, got 200")
	}

	// Admin reactivates.
	w = makeRequest(t, router, http.MethodPatch,
		fmt.Sprintf("/api/v1/users/%d/active", user.ID), adminToken,
		map[string]interface{}{"is_active": true})
	assertStatus(t, w, http.StatusOK)

	// User can now log in again.
	w = makeRequest(t, router, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": "toggle_user", "password": userPw})
	assertStatus(t, w, http.StatusOK)
}

// TestUser_ToggleActive_NonAdminForbidden verifies that a non-admin cannot
// toggle another user's active state.
func TestUser_ToggleActive_NonAdminForbidden(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	target, _ := createTestUser(t, db, "tgfb_target", common.RoleTenant)
	_, pmPw := createTestUser(t, db, "tgfb_pm", common.RolePropertyManager)
	pmToken := loginUser(t, router, "tgfb_pm", pmPw)

	w := makeRequest(t, router, http.MethodPatch,
		fmt.Sprintf("/api/v1/users/%d/active", target.ID), pmToken,
		map[string]interface{}{"is_active": false})
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin ToggleActive, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestUser_AssignAndRemoveRole verifies that an admin can assign and remove roles.
func TestUser_AssignAndRemoveRole(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	user, _ := createTestUser(t, db, "role_user", common.RoleTenant)
	_, adminPw := createTestUser(t, db, "role_admin", common.RoleSystemAdmin)
	adminToken := loginUser(t, router, "role_admin", adminPw)

	// Assign PropertyManager role.
	w := makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/users/%d/roles", user.ID), adminToken,
		map[string]string{"role_name": common.RolePropertyManager})
	assertStatus(t, w, http.StatusOK)

	// Remove PropertyManager role.
	w = makeRequest(t, router, http.MethodDelete,
		fmt.Sprintf("/api/v1/users/%d/roles/%s", user.ID, common.RolePropertyManager), adminToken, nil)
	assertStatus(t, w, http.StatusOK)
}

// TestUser_AssignRole_NonAdminForbidden verifies that only SystemAdmin can assign roles.
func TestUser_AssignRole_NonAdminForbidden(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	target, _ := createTestUser(t, db, "arfb_target", common.RoleTenant)
	_, pmPw := createTestUser(t, db, "arfb_pm", common.RolePropertyManager)
	pmToken := loginUser(t, router, "arfb_pm", pmPw)

	w := makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/users/%d/roles", target.ID), pmToken,
		map[string]string{"role_name": common.RoleSystemAdmin})
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin AssignRole, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestUser_ListUsers_AdminOnly verifies that GET /users is restricted to SystemAdmin.
func TestUser_ListUsers_AdminOnly(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, adminPw := createTestUser(t, db, "list_admin", common.RoleSystemAdmin)
	_, tenantPw := createTestUser(t, db, "list_tenant", common.RoleTenant)

	adminToken := loginUser(t, router, "list_admin", adminPw)
	tenantToken := loginUser(t, router, "list_tenant", tenantPw)

	// Admin can list.
	w := makeRequest(t, router, http.MethodGet, "/api/v1/users", adminToken, nil)
	assertStatus(t, w, http.StatusOK)

	// Tenant is denied.
	w = makeRequest(t, router, http.MethodGet, "/api/v1/users", tenantToken, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for Tenant ListUsers, got %d; body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Tenant profile tests
// ---------------------------------------------------------------------------

// TestTenantProfile_CreateAndGet verifies that a PM can create a tenant profile
// and the tenant themselves can retrieve it.
func TestTenantProfile_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	tenantUser, tenantPw := createTestUser(t, db, "tp_tenant", common.RoleTenant)
	_, pmPw := createTestUser(t, db, "tp_pm", common.RolePropertyManager)

	pmToken := loginUser(t, router, "tp_pm", pmPw)
	tenantToken := loginUser(t, router, "tp_tenant", tenantPw)

	// PM creates a tenant profile for the tenant.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/tenants", pmToken, map[string]interface{}{
		"user_id":           tenantUser.ID,
		"emergency_contact": "Jane Doe - 555-1234",
		"lease_start":       "2024-01-01",
		"lease_end":         "2024-12-31",
	})
	assertStatus(t, w, http.StatusCreated)

	var createResp struct {
		Data struct {
			ID     uint64 `json:"id"`
			UserID uint64 `json:"user_id"`
		} `json:"data"`
	}
	parseResponse(t, w, &createResp)
	profileID := createResp.Data.ID

	if profileID == 0 {
		t.Fatalf("expected non-zero tenant profile ID")
	}
	if createResp.Data.UserID != tenantUser.ID {
		t.Errorf("expected user_id=%d, got %d", tenantUser.ID, createResp.Data.UserID)
	}

	// Tenant can retrieve their own profile.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/tenants/%d", profileID), tenantToken, nil)
	assertStatus(t, w, http.StatusOK)

	var getResp struct {
		Data struct {
			ID     uint64 `json:"id"`
			UserID uint64 `json:"user_id"`
		} `json:"data"`
	}
	parseResponse(t, w, &getResp)
	if getResp.Data.UserID != tenantUser.ID {
		t.Errorf("GET tenant profile: expected user_id=%d, got %d", tenantUser.ID, getResp.Data.UserID)
	}
}

// TestTenantProfile_GetByUser verifies that GET /tenants/by-user/:user_id works.
func TestTenantProfile_GetByUser(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	tenantUser, tenantPw := createTestUser(t, db, "tpbu_tenant", common.RoleTenant)
	_, pmPw := createTestUser(t, db, "tpbu_pm", common.RolePropertyManager)

	pmToken := loginUser(t, router, "tpbu_pm", pmPw)
	tenantToken := loginUser(t, router, "tpbu_tenant", tenantPw)

	// PM creates the profile.
	makeRequest(t, router, http.MethodPost, "/api/v1/tenants", pmToken, map[string]interface{}{
		"user_id": tenantUser.ID,
	})

	// Tenant retrieves by user_id.
	w := makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/tenants/by-user/%d", tenantUser.ID), tenantToken, nil)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Data struct {
			UserID uint64 `json:"user_id"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)
	if resp.Data.UserID != tenantUser.ID {
		t.Errorf("expected user_id=%d, got %d", tenantUser.ID, resp.Data.UserID)
	}
}

// TestTenantProfile_UpdateProfile verifies that the owning tenant can update their profile.
func TestTenantProfile_UpdateProfile(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	tenantUser, tenantPw := createTestUser(t, db, "tpupd_tenant", common.RoleTenant)
	_, pmPw := createTestUser(t, db, "tpupd_pm", common.RolePropertyManager)

	pmToken := loginUser(t, router, "tpupd_pm", pmPw)
	tenantToken := loginUser(t, router, "tpupd_tenant", tenantPw)

	// Create profile.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/tenants", pmToken, map[string]interface{}{
		"user_id": tenantUser.ID,
	})
	assertStatus(t, w, http.StatusCreated)
	var createResp struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &createResp)
	profileID := createResp.Data.ID

	newContact := "Bob Smith - 555-9876"

	// Tenant updates their own profile.
	w = makeRequest(t, router, http.MethodPut,
		fmt.Sprintf("/api/v1/tenants/%d", profileID), tenantToken, map[string]interface{}{
			"emergency_contact": newContact,
		})
	assertStatus(t, w, http.StatusOK)
}

// TestTenantProfile_Create_TenantForbidden verifies that a Tenant cannot create
// tenant profiles (only PM and SystemAdmin can).
func TestTenantProfile_Create_TenantForbidden(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	tenantUser, tenantPw := createTestUser(t, db, "tpcf_tenant", common.RoleTenant)
	tenantToken := loginUser(t, router, "tpcf_tenant", tenantPw)

	w := makeRequest(t, router, http.MethodPost, "/api/v1/tenants", tenantToken, map[string]interface{}{
		"user_id": tenantUser.ID,
	})
	if w.Code != http.StatusForbidden {
		t.Errorf("Tenant should not be able to create tenant profiles, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestTenantProfile_CrossTenantBlocked verifies that a tenant cannot GET
// another tenant's profile.
func TestTenantProfile_CrossTenantBlocked(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	tenant1, _ := createTestUser(t, db, "tpct_t1", common.RoleTenant)
	_, tenant2Pw := createTestUser(t, db, "tpct_t2", common.RoleTenant)
	_, pmPw := createTestUser(t, db, "tpct_pm", common.RolePropertyManager)

	pmToken := loginUser(t, router, "tpct_pm", pmPw)
	tenant2Token := loginUser(t, router, "tpct_t2", tenant2Pw)

	// PM creates a profile for tenant1.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/tenants", pmToken, map[string]interface{}{
		"user_id": tenant1.ID,
	})
	assertStatus(t, w, http.StatusCreated)
	var resp struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &resp)
	profileID := resp.Data.ID

	// Tenant2 tries to GET tenant1's profile — should be denied.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/tenants/%d", profileID), tenant2Token, nil)
	if w.Code == http.StatusOK {
		t.Errorf("tenant2 should not access tenant1's profile, got 200; body: %s", w.Body.String())
	}
}
