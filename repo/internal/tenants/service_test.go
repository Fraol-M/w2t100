package tenants

import (
	"testing"

	"propertyops/backend/internal/common"

	"gorm.io/gorm"
)

// --- Mock AuditLogger ---

type mockAuditLogger struct {
	calls []auditCall
}

type auditCall struct {
	actorID      uint64
	action       string
	resourceType string
	resourceID   uint64
}

func (m *mockAuditLogger) Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string) {
	m.calls = append(m.calls, auditCall{
		actorID:      actorID,
		action:       action,
		resourceType: resourceType,
		resourceID:   resourceID,
	})
}

// --- Mock FieldEncryptor ---

type mockEncryptor struct{}

func (m *mockEncryptor) Encrypt(plaintext []byte) ([]byte, int, error) {
	enc := make([]byte, len(plaintext))
	for i, b := range plaintext {
		enc[len(plaintext)-1-i] = b
	}
	return enc, 1, nil
}

func (m *mockEncryptor) Decrypt(ciphertext []byte, keyVersion int) ([]byte, error) {
	dec := make([]byte, len(ciphertext))
	for i, b := range ciphertext {
		dec[len(ciphertext)-1-i] = b
	}
	return dec, nil
}

// --- Mock Repository ---

type mockRepository struct {
	profiles map[uint64]*TenantProfile
	nextID   uint64
	pmUnits  map[pmUnitKey]bool
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		profiles: make(map[uint64]*TenantProfile),
		nextID:   1,
		pmUnits:  make(map[pmUnitKey]bool),
	}
}

func (m *mockRepository) Create(profile *TenantProfile) error {
	profile.ID = m.nextID
	m.nextID++
	m.profiles[profile.ID] = profile
	return nil
}

func (m *mockRepository) FindByID(id uint64) (*TenantProfile, error) {
	p, ok := m.profiles[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return p, nil
}

func (m *mockRepository) FindByUserID(userID uint64) (*TenantProfile, error) {
	for _, p := range m.profiles {
		if p.UserID == userID {
			return p, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockRepository) Update(profile *TenantProfile) error {
	if _, ok := m.profiles[profile.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	m.profiles[profile.ID] = profile
	return nil
}

func (m *mockRepository) List(page, perPage int) ([]TenantProfile, int64, error) {
	var result []TenantProfile
	for _, p := range m.profiles {
		result = append(result, *p)
	}
	return result, int64(len(result)), nil
}

func (m *mockRepository) FindByProperty(propertyID uint64, page, perPage int) ([]TenantProfile, int64, error) {
	// Simplified: return all profiles (mock doesn't track units/properties)
	return m.List(page, perPage)
}

// pmUnits is a set of (unitID, pmID) pairs the mock considers managed.
// Tests populate this to control IsPMForUnit outcomes.
type pmUnitKey struct{ unitID, pmID uint64 }

func (m *mockRepository) IsPMForUnit(unitID, pmID uint64) bool {
	return m.pmUnits[pmUnitKey{unitID, pmID}]
}

func (m *mockRepository) IsPMForProperty(propertyID, pmID uint64) bool {
	return false // not exercised by existing unit tests
}

// --- Tests ---

func TestCreateProfile(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}

	svc := NewService(repo, audit, encryptor)

	req := CreateTenantProfileRequest{
		UserID:           10,
		EmergencyContact: "555-1234",
		LeaseStart:       "2025-01-01",
		LeaseEnd:         "2026-01-01",
		Notes:            "Test tenant",
	}

	resp, appErr := svc.CreateProfile(req, 1, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.UserID != 10 {
		t.Errorf("expected user_id 10, got %d", resp.UserID)
	}
	if resp.UUID == "" {
		t.Error("expected UUID to be generated")
	}
}

func TestCreateProfile_Duplicate(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}

	svc := NewService(repo, audit, encryptor)

	req := CreateTenantProfileRequest{
		UserID: 10,
	}

	_, appErr := svc.CreateProfile(req, 1, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("first create failed: %v", appErr)
	}

	_, appErr = svc.CreateProfile(req, 1, "127.0.0.1", "req-2")
	if appErr == nil {
		t.Fatal("expected conflict error for duplicate profile")
	}
	if appErr.Code != "CONFLICT" {
		t.Errorf("expected CONFLICT error code, got %s", appErr.Code)
	}
}

func TestObjectLevelAuth_TenantCanAccessOwnProfile(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}

	svc := NewService(repo, audit, encryptor)

	req := CreateTenantProfileRequest{
		UserID:           42,
		EmergencyContact: "555-9999",
	}

	created, _ := svc.CreateProfile(req, 1, "127.0.0.1", "req-1")

	// Tenant (user 42) accesses their own profile
	resp, appErr := svc.GetByID(created.ID, 42, []string{common.RoleTenant})
	if appErr != nil {
		t.Fatalf("expected access to own profile, got error: %v", appErr)
	}
	if resp.UserID != 42 {
		t.Errorf("expected user_id 42, got %d", resp.UserID)
	}
	// Emergency contact should be revealed for own profile
	if resp.EmergencyContact == "" {
		t.Error("expected emergency contact to be revealed for own profile")
	}
}

func TestObjectLevelAuth_TenantCannotAccessOtherProfile(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}

	svc := NewService(repo, audit, encryptor)

	req := CreateTenantProfileRequest{
		UserID: 42,
	}

	created, _ := svc.CreateProfile(req, 1, "127.0.0.1", "req-1")

	// A different tenant (user 99) tries to access the profile
	_, appErr := svc.GetByID(created.ID, 99, []string{common.RoleTenant})
	if appErr == nil {
		t.Fatal("expected forbidden error, got nil")
	}
	if appErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN error code, got %s", appErr.Code)
	}
}

func TestObjectLevelAuth_AdminCanAccessAnyProfile(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}

	svc := NewService(repo, audit, encryptor)

	req := CreateTenantProfileRequest{
		UserID:           42,
		EmergencyContact: "555-9999",
	}

	created, _ := svc.CreateProfile(req, 1, "127.0.0.1", "req-1")

	// Admin accesses any profile
	resp, appErr := svc.GetByID(created.ID, 1, []string{common.RoleSystemAdmin})
	if appErr != nil {
		t.Fatalf("expected admin to access profile, got error: %v", appErr)
	}
	if resp.UserID != 42 {
		t.Errorf("expected user_id 42, got %d", resp.UserID)
	}
}

func TestObjectLevelAuth_PMDeniedWhenProfileHasNoUnit(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}

	svc := NewService(repo, audit, encryptor)

	// Profile with no UnitID — PM has no unit to check against.
	req := CreateTenantProfileRequest{
		UserID: 42,
	}

	created, _ := svc.CreateProfile(req, 1, "127.0.0.1", "req-1")

	_, appErr := svc.GetByID(created.ID, 5, []string{common.RolePropertyManager})
	if appErr == nil {
		t.Fatal("expected FORBIDDEN for PM when profile has no unit, got nil")
	}
	if appErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN, got %s", appErr.Code)
	}
}

func TestObjectLevelAuth_PMDeniedWhenNotManagingUnit(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}

	svc := NewService(repo, audit, encryptor)

	unitID := uint64(10)
	req := CreateTenantProfileRequest{
		UserID: 42,
		UnitID: &unitID,
	}

	created, _ := svc.CreateProfile(req, 1, "127.0.0.1", "req-1")

	// PM 5 is not in the pmUnits map for unit 10 — access must be denied.
	_, appErr := svc.GetByID(created.ID, 5, []string{common.RolePropertyManager})
	if appErr == nil {
		t.Fatal("expected FORBIDDEN for PM not managing the unit, got nil")
	}
	if appErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN, got %s", appErr.Code)
	}
}

func TestObjectLevelAuth_PMAllowedWhenManagingUnit(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}

	svc := NewService(repo, audit, encryptor)

	unitID := uint64(10)
	req := CreateTenantProfileRequest{
		UserID: 42,
		UnitID: &unitID,
	}

	created, _ := svc.CreateProfile(req, 1, "127.0.0.1", "req-1")

	// Grant PM 5 access to unit 10.
	repo.pmUnits[pmUnitKey{unitID: 10, pmID: 5}] = true

	_, appErr := svc.GetByID(created.ID, 5, []string{common.RolePropertyManager})
	if appErr != nil {
		t.Fatalf("expected PM managing the unit to access profile, got error: %v", appErr)
	}
}

func TestUpdateProfile_ObjectLevelAuth(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}

	svc := NewService(repo, audit, encryptor)

	req := CreateTenantProfileRequest{
		UserID: 42,
		Notes:  "Original notes",
	}

	created, _ := svc.CreateProfile(req, 1, "127.0.0.1", "req-1")

	// Another tenant cannot update
	newNotes := "Hacked"
	_, appErr := svc.UpdateProfile(created.ID, UpdateTenantProfileRequest{Notes: &newNotes}, 99, []string{common.RoleTenant}, "127.0.0.1", "req-2")
	if appErr == nil {
		t.Fatal("expected forbidden error")
	}
	if appErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN, got %s", appErr.Code)
	}

	// Owner can update
	updatedNotes := "Updated notes"
	resp, appErr := svc.UpdateProfile(created.ID, UpdateTenantProfileRequest{Notes: &updatedNotes}, 42, []string{common.RoleTenant}, "127.0.0.1", "req-3")
	if appErr != nil {
		t.Fatalf("expected owner to update profile, got error: %v", appErr)
	}
	if resp.Notes == nil || *resp.Notes != "Updated notes" {
		t.Error("expected notes to be updated")
	}
}

func TestCreateProfile_InvalidDates(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}

	svc := NewService(repo, audit, encryptor)

	req := CreateTenantProfileRequest{
		UserID:     50,
		LeaseStart: "not-a-date",
	}

	_, appErr := svc.CreateProfile(req, 1, "127.0.0.1", "req-1")
	if appErr == nil {
		t.Fatal("expected validation error for invalid date")
	}
	if appErr.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", appErr.Code)
	}
}
