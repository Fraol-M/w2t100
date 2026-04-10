package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"propertyops/backend/internal/common"
)

// ---------------------------------------------------------------------------
// Property CRUD
// ---------------------------------------------------------------------------

// TestProperty_AdminCreateAndGet verifies that an admin can create a property
// and immediately read it back.
func TestProperty_AdminCreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, adminPw := createTestUser(t, db, "pac_admin", common.RoleSystemAdmin)
	adminToken := loginUser(t, router, "pac_admin", adminPw)

	// Create property.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/properties", adminToken, map[string]interface{}{
		"name":          "Sunset Apartments",
		"address_line1": "123 Sunset Blvd",
		"city":          "Los Angeles",
		"state":         "CA",
		"zip_code":      "90028",
	})
	assertStatus(t, w, http.StatusCreated)

	var createResp struct {
		Data struct {
			ID       uint64 `json:"id"`
			Name     string `json:"name"`
			City     string `json:"city"`
			IsActive bool   `json:"is_active"`
		} `json:"data"`
	}
	parseResponse(t, w, &createResp)
	propID := createResp.Data.ID

	if propID == 0 {
		t.Fatalf("expected non-zero property ID")
	}
	if createResp.Data.Name != "Sunset Apartments" {
		t.Errorf("expected name=Sunset Apartments, got %q", createResp.Data.Name)
	}
	if createResp.Data.City != "Los Angeles" {
		t.Errorf("expected city=Los Angeles, got %q", createResp.Data.City)
	}
	if !createResp.Data.IsActive {
		t.Errorf("new property should be active by default")
	}

	// Admin gets the property.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/properties/%d", propID), adminToken, nil)
	assertStatus(t, w, http.StatusOK)

	var getResp struct {
		Data struct {
			ID   uint64 `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	parseResponse(t, w, &getResp)
	if getResp.Data.ID != propID {
		t.Errorf("GET property: expected id=%d, got %d", propID, getResp.Data.ID)
	}
}

// TestProperty_UpdateProperty verifies that an admin can update a property's fields.
func TestProperty_UpdateProperty(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, adminPw := createTestUser(t, db, "pup_admin", common.RoleSystemAdmin)
	adminToken := loginUser(t, router, "pup_admin", adminPw)

	// Create then update.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/properties", adminToken, map[string]interface{}{
		"name":          "Old Name Tower",
		"address_line1": "456 Old St",
		"city":          "Chicago",
		"state":         "IL",
		"zip_code":      "60601",
	})
	assertStatus(t, w, http.StatusCreated)
	var cr struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &cr)
	propID := cr.Data.ID

	newName := "New Name Tower"
	w = makeRequest(t, router, http.MethodPut,
		fmt.Sprintf("/api/v1/properties/%d", propID), adminToken, map[string]interface{}{
			"name": newName,
		})
	assertStatus(t, w, http.StatusOK)

	var updateResp struct {
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	parseResponse(t, w, &updateResp)
	if updateResp.Data.Name != newName {
		t.Errorf("expected name=%q after update, got %q", newName, updateResp.Data.Name)
	}
}

// TestProperty_ListProperties_PMScoped verifies that a PM only sees properties
// they manage, while an admin sees all.
func TestProperty_ListProperties_PMScoped(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, adminPw := createTestUser(t, db, "pls_admin", common.RoleSystemAdmin)
	pmUser, pmPw := createTestUser(t, db, "pls_pm", common.RolePropertyManager)

	adminToken := loginUser(t, router, "pls_admin", adminPw)
	pmToken := loginUser(t, router, "pls_pm", pmPw)

	// Admin creates two properties; PM is manager of the first, not the second.
	createProp := func(name string) uint64 {
		w := makeRequest(t, router, http.MethodPost, "/api/v1/properties", adminToken, map[string]interface{}{
			"name":          name,
			"address_line1": "1 Main St",
			"city":          "Austin",
			"state":         "TX",
			"zip_code":      "78701",
		})
		var resp struct {
			Data struct{ ID uint64 `json:"id"` } `json:"data"`
		}
		parseResponse(t, w, &resp)
		return resp.Data.ID
	}

	prop1ID := createProp("PM Managed Property")
	_ = createProp("Unmanaged Property")

	// Assign PM to property 1 via staff assignment.
	w := makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/properties/%d/staff", prop1ID), adminToken,
		map[string]interface{}{
			"user_id": pmUser.ID,
			"role":    common.RolePropertyManager,
		})
	assertStatus(t, w, http.StatusCreated)

	// PM lists properties — must only see the one they manage.
	w = makeRequest(t, router, http.MethodGet, "/api/v1/properties", pmToken, nil)
	assertStatus(t, w, http.StatusOK)

	var pmList struct {
		Data []struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &pmList)

	for _, p := range pmList.Data {
		if p.ID != prop1ID {
			t.Errorf("PM should only see managed property (id=%d), but got id=%d", prop1ID, p.ID)
		}
	}

	// Admin lists all — must see both.
	w = makeRequest(t, router, http.MethodGet, "/api/v1/properties", adminToken, nil)
	assertStatus(t, w, http.StatusOK)

	var adminList struct {
		Data []struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &adminList)
	if len(adminList.Data) < 2 {
		t.Errorf("admin should see at least 2 properties, got %d", len(adminList.Data))
	}
}

// TestProperty_TenantForbidden verifies that Tenant and Technician cannot
// create or list properties.
func TestProperty_TenantForbidden(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, tenantPw := createTestUser(t, db, "ptf_tenant", common.RoleTenant)
	_, techPw := createTestUser(t, db, "ptf_tech", common.RoleTechnician)

	tenantToken := loginUser(t, router, "ptf_tenant", tenantPw)
	techToken := loginUser(t, router, "ptf_tech", techPw)

	for _, token := range []string{tenantToken, techToken} {
		w := makeRequest(t, router, http.MethodGet, "/api/v1/properties", token, nil)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for non-PM/admin listing properties, got %d", w.Code)
		}
		w = makeRequest(t, router, http.MethodPost, "/api/v1/properties", token, map[string]interface{}{
			"name": "Hacked Property", "address_line1": "1 St", "city": "C", "state": "S", "zip_code": "00000",
		})
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for non-PM/admin creating property, got %d", w.Code)
		}
	}
}

// TestProperty_PMCannotAccessUnmanagedProperty verifies that a PM is denied
// access to properties they don't manage (GET, PUT).
func TestProperty_PMCannotAccessUnmanagedProperty(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, adminPw := createTestUser(t, db, "pmua_admin", common.RoleSystemAdmin)
	_, pmPw := createTestUser(t, db, "pmua_pm", common.RolePropertyManager)

	adminToken := loginUser(t, router, "pmua_admin", adminPw)
	pmToken := loginUser(t, router, "pmua_pm", pmPw)

	// Admin creates a property — PM has no assignment.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/properties", adminToken, map[string]interface{}{
		"name":          "Admin-Only Building",
		"address_line1": "99 Admin Rd",
		"city":          "Dallas",
		"state":         "TX",
		"zip_code":      "75201",
	})
	assertStatus(t, w, http.StatusCreated)
	var cr struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &cr)
	propID := cr.Data.ID

	// PM tries to GET → must be denied.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/properties/%d", propID), pmToken, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("PM should not GET unmanaged property, got %d; body: %s", w.Code, w.Body.String())
	}

	// PM tries to PUT → must be denied.
	w = makeRequest(t, router, http.MethodPut,
		fmt.Sprintf("/api/v1/properties/%d", propID), pmToken, map[string]interface{}{
			"name": "Hacked Name",
		})
	if w.Code != http.StatusForbidden {
		t.Errorf("PM should not PUT unmanaged property, got %d; body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Unit management
// ---------------------------------------------------------------------------

// TestUnit_CreateListGetUpdate exercises the full unit lifecycle under a property.
func TestUnit_CreateListGetUpdate(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, adminPw := createTestUser(t, db, "unit_admin", common.RoleSystemAdmin)
	adminToken := loginUser(t, router, "unit_admin", adminPw)

	// Create a property.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/properties", adminToken, map[string]interface{}{
		"name":          "Unit Test Building",
		"address_line1": "10 Unit Ave",
		"city":          "Seattle",
		"state":         "WA",
		"zip_code":      "98101",
	})
	assertStatus(t, w, http.StatusCreated)
	var propResp struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &propResp)
	propID := propResp.Data.ID

	// Create a unit.
	w = makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/properties/%d/units", propID), adminToken, map[string]interface{}{
			"unit_number": "101",
			"floor":       1,
			"bedrooms":    2,
			"bathrooms":   1,
			"square_feet": 850,
			"status":      "Vacant",
		})
	assertStatus(t, w, http.StatusCreated)

	var createResp struct {
		Data struct {
			ID         uint64 `json:"id"`
			PropertyID uint64 `json:"property_id"`
			UnitNumber string `json:"unit_number"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	parseResponse(t, w, &createResp)
	unitID := createResp.Data.ID

	if unitID == 0 {
		t.Fatalf("expected non-zero unit ID")
	}
	if createResp.Data.PropertyID != propID {
		t.Errorf("expected property_id=%d, got %d", propID, createResp.Data.PropertyID)
	}
	if createResp.Data.UnitNumber != "101" {
		t.Errorf("expected unit_number=101, got %q", createResp.Data.UnitNumber)
	}

	// List units.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/properties/%d/units", propID), adminToken, nil)
	assertStatus(t, w, http.StatusOK)

	var listResp struct {
		Data []struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &listResp)
	if len(listResp.Data) == 0 {
		t.Errorf("expected at least one unit in list")
	}

	// Get single unit.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/properties/%d/units/%d", propID, unitID), adminToken, nil)
	assertStatus(t, w, http.StatusOK)

	// Update unit status.
	newStatus := "Occupied"
	w = makeRequest(t, router, http.MethodPut,
		fmt.Sprintf("/api/v1/properties/%d/units/%d", propID, unitID), adminToken,
		map[string]interface{}{"status": newStatus})
	assertStatus(t, w, http.StatusOK)

	var updateResp struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	parseResponse(t, w, &updateResp)
	if updateResp.Data.Status != newStatus {
		t.Errorf("expected status=%q after update, got %q", newStatus, updateResp.Data.Status)
	}
}

// ---------------------------------------------------------------------------
// Staff assignments
// ---------------------------------------------------------------------------

// TestStaff_AssignListRemove exercises the staff assignment API for a property.
func TestStaff_AssignListRemove(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, adminPw := createTestUser(t, db, "staff_admin", common.RoleSystemAdmin)
	pmUser, pmPw := createTestUser(t, db, "staff_pm", common.RolePropertyManager)
	techUser, _ := createTestUser(t, db, "staff_tech", common.RoleTechnician)

	adminToken := loginUser(t, router, "staff_admin", adminPw)
	pmToken := loginUser(t, router, "staff_pm", pmPw)

	// Admin creates property.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/properties", adminToken, map[string]interface{}{
		"name":          "Staff Test Property",
		"address_line1": "50 Staff Ln",
		"city":          "Boston",
		"state":         "MA",
		"zip_code":      "02101",
	})
	assertStatus(t, w, http.StatusCreated)
	var propResp struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &propResp)
	propID := propResp.Data.ID

	// Assign PM to property.
	w = makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/properties/%d/staff", propID), adminToken,
		map[string]interface{}{
			"user_id": pmUser.ID,
			"role":    common.RolePropertyManager,
		})
	assertStatus(t, w, http.StatusCreated)

	var assignResp struct {
		Data struct {
			PropertyID uint64 `json:"property_id"`
			UserID     uint64 `json:"user_id"`
			Role       string `json:"role"`
		} `json:"data"`
	}
	parseResponse(t, w, &assignResp)
	if assignResp.Data.PropertyID != propID {
		t.Errorf("expected property_id=%d, got %d", propID, assignResp.Data.PropertyID)
	}
	if assignResp.Data.UserID != pmUser.ID {
		t.Errorf("expected user_id=%d, got %d", pmUser.ID, assignResp.Data.UserID)
	}

	// PM (now managing this property) can assign a technician.
	w = makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/properties/%d/staff", propID), pmToken,
		map[string]interface{}{
			"user_id": techUser.ID,
			"role":    common.RoleTechnician,
		})
	assertStatus(t, w, http.StatusCreated)

	// List staff — must include both.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/properties/%d/staff", propID), adminToken, nil)
	assertStatus(t, w, http.StatusOK)

	var staffList struct {
		Data []struct {
			UserID uint64 `json:"user_id"`
			Role   string `json:"role"`
		} `json:"data"`
	}
	parseResponse(t, w, &staffList)

	foundPM := false
	foundTech := false
	for _, s := range staffList.Data {
		if s.UserID == pmUser.ID && s.Role == common.RolePropertyManager {
			foundPM = true
		}
		if s.UserID == techUser.ID && s.Role == common.RoleTechnician {
			foundTech = true
		}
	}
	if !foundPM {
		t.Errorf("expected PM (user_id=%d) in staff list", pmUser.ID)
	}
	if !foundTech {
		t.Errorf("expected Technician (user_id=%d) in staff list", techUser.ID)
	}

	// Remove the technician from the property.
	w = makeRequest(t, router, http.MethodDelete,
		fmt.Sprintf("/api/v1/properties/%d/staff/%d/%s", propID, techUser.ID, common.RoleTechnician),
		adminToken, nil)
	assertStatus(t, w, http.StatusOK)

	// Verify technician no longer in staff list.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/properties/%d/staff", propID), adminToken, nil)
	assertStatus(t, w, http.StatusOK)
	parseResponse(t, w, &staffList)

	for _, s := range staffList.Data {
		if s.UserID == techUser.ID {
			t.Errorf("technician (user_id=%d) should have been removed from staff list", techUser.ID)
		}
	}
}

// TestStaff_UnassignedPMCannotManageProperty verifies that assigning a PM to a
// property via the API actually grants them access to manage it, and that a PM
// without an assignment is still denied.
func TestStaff_PMAccessGatedByAssignment(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, adminPw := createTestUser(t, db, "pma_admin", common.RoleSystemAdmin)
	pm1User, pm1Pw := createTestUser(t, db, "pma_pm1", common.RolePropertyManager)
	_, pm2Pw := createTestUser(t, db, "pma_pm2", common.RolePropertyManager)

	adminToken := loginUser(t, router, "pma_admin", adminPw)
	pm1Token := loginUser(t, router, "pma_pm1", pm1Pw)
	pm2Token := loginUser(t, router, "pma_pm2", pm2Pw)

	// Create property.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/properties", adminToken, map[string]interface{}{
		"name":          "Access Gated Building",
		"address_line1": "1 Gate Rd",
		"city":          "Miami",
		"state":         "FL",
		"zip_code":      "33101",
	})
	assertStatus(t, w, http.StatusCreated)
	var propResp struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &propResp)
	propID := propResp.Data.ID

	// Before assignment: PM1 is denied.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/properties/%d", propID), pm1Token, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("PM1 should be denied before assignment, got %d", w.Code)
	}

	// Admin assigns PM1 to property.
	makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/properties/%d/staff", propID), adminToken,
		map[string]interface{}{"user_id": pm1User.ID, "role": common.RolePropertyManager})

	// After assignment: PM1 can access.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/properties/%d", propID), pm1Token, nil)
	assertStatus(t, w, http.StatusOK)

	// PM2 (never assigned) is still denied.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/properties/%d", propID), pm2Token, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("PM2 (unassigned) should still be denied, got %d; body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Technician skill tags
// ---------------------------------------------------------------------------

// TestSkillTags_AddListRemove exercises the full skill tag lifecycle for a technician.
func TestSkillTags_AddListRemove(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, adminPw := createTestUser(t, db, "skill_admin", common.RoleSystemAdmin)
	techUser, _ := createTestUser(t, db, "skill_tech", common.RoleTechnician)

	adminToken := loginUser(t, router, "skill_admin", adminPw)

	// Add a skill tag.
	w := makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/properties/technicians/%d/skills", techUser.ID), adminToken,
		map[string]string{"tag": "Plumbing"})
	assertStatus(t, w, http.StatusCreated)

	var addResp struct {
		Data struct {
			UserID uint64 `json:"user_id"`
			Tag    string `json:"tag"`
		} `json:"data"`
	}
	parseResponse(t, w, &addResp)
	if addResp.Data.Tag != "Plumbing" {
		t.Errorf("expected tag=Plumbing, got %q", addResp.Data.Tag)
	}
	if addResp.Data.UserID != techUser.ID {
		t.Errorf("expected user_id=%d, got %d", techUser.ID, addResp.Data.UserID)
	}

	// Add a second skill.
	makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/properties/technicians/%d/skills", techUser.ID), adminToken,
		map[string]string{"tag": "HVAC"})

	// List skills — must contain both.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/properties/technicians/%d/skills", techUser.ID), adminToken, nil)
	assertStatus(t, w, http.StatusOK)

	var listResp struct {
		Data []struct {
			Tag string `json:"tag"`
		} `json:"data"`
	}
	parseResponse(t, w, &listResp)

	foundPlumbing := false
	foundHVAC := false
	for _, s := range listResp.Data {
		if s.Tag == "Plumbing" {
			foundPlumbing = true
		}
		if s.Tag == "HVAC" {
			foundHVAC = true
		}
	}
	if !foundPlumbing {
		t.Errorf("expected Plumbing skill in list")
	}
	if !foundHVAC {
		t.Errorf("expected HVAC skill in list")
	}

	// Remove the Plumbing skill.
	w = makeRequest(t, router, http.MethodDelete,
		fmt.Sprintf("/api/v1/properties/technicians/%d/skills/Plumbing", techUser.ID), adminToken, nil)
	assertStatus(t, w, http.StatusOK)

	// Verify Plumbing is gone, HVAC remains.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/properties/technicians/%d/skills", techUser.ID), adminToken, nil)
	assertStatus(t, w, http.StatusOK)
	parseResponse(t, w, &listResp)

	for _, s := range listResp.Data {
		if s.Tag == "Plumbing" {
			t.Errorf("Plumbing skill should have been removed")
		}
	}

	hasHVAC := false
	for _, s := range listResp.Data {
		if s.Tag == "HVAC" {
			hasHVAC = true
		}
	}
	if !hasHVAC {
		t.Errorf("HVAC skill should still be present after removing Plumbing")
	}
}

// TestSkillTags_TenantForbidden verifies that Tenant and Technician cannot
// manage skill tags (PM/Admin only).
func TestSkillTags_TenantForbidden(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	techUser, techPw := createTestUser(t, db, "sktf_tech", common.RoleTechnician)
	_, tenantPw := createTestUser(t, db, "sktf_tenant", common.RoleTenant)

	techToken := loginUser(t, router, "sktf_tech", techPw)
	tenantToken := loginUser(t, router, "sktf_tenant", tenantPw)

	for _, token := range []string{techToken, tenantToken} {
		// Adding a skill tag must be forbidden.
		w := makeRequest(t, router, http.MethodPost,
			fmt.Sprintf("/api/v1/properties/technicians/%d/skills", techUser.ID), token,
			map[string]string{"tag": "Electrical"})
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for non-PM adding skill tag, got %d; body: %s", w.Code, w.Body.String())
		}

		// Listing skill tags must also be forbidden.
		w = makeRequest(t, router, http.MethodGet,
			fmt.Sprintf("/api/v1/properties/technicians/%d/skills", techUser.ID), token, nil)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for non-PM listing skill tags, got %d; body: %s", w.Code, w.Body.String())
		}
	}
}

// TestProperties_Unauthenticated verifies that all property endpoints return 401
// when called without a bearer token.
func TestProperties_Unauthenticated(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/properties"},
		{http.MethodGet, "/api/v1/properties"},
		{http.MethodGet, "/api/v1/properties/1"},
		{http.MethodPut, "/api/v1/properties/1"},
		{http.MethodPost, "/api/v1/properties/1/units"},
		{http.MethodGet, "/api/v1/properties/1/units"},
		{http.MethodGet, "/api/v1/properties/1/units/1"},
		{http.MethodPut, "/api/v1/properties/1/units/1"},
		{http.MethodPost, "/api/v1/properties/1/staff"},
		{http.MethodGet, "/api/v1/properties/1/staff"},
		{http.MethodDelete, "/api/v1/properties/1/staff/1/Technician"},
		{http.MethodPost, "/api/v1/properties/technicians/1/skills"},
		{http.MethodGet, "/api/v1/properties/technicians/1/skills"},
		{http.MethodDelete, "/api/v1/properties/technicians/1/skills/Plumbing"},
	}

	for _, ep := range endpoints {
		ep := ep
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := makeRequest(t, router, ep.method, ep.path, "", nil)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 without token, got %d; body: %s", w.Code, w.Body.String())
			}
		})
	}
}
