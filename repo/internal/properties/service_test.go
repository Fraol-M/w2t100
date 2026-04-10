package properties

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

// --- Mock Repository ---

type mockRepository struct {
	properties map[uint64]*Property
	units      map[uint64]*Unit
	staff      []PropertyStaffAssignment
	skillTags  []TechnicianSkillTag
	cursors    map[string]*DispatchCursor // key: "propertyID:skillTag"
	nextPropID uint64
	nextUnitID uint64
	nextStaffID uint64
	nextTagID   uint64
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		properties: make(map[uint64]*Property),
		units:      make(map[uint64]*Unit),
		cursors:    make(map[string]*DispatchCursor),
		nextPropID: 1,
		nextUnitID: 1,
		nextStaffID: 1,
		nextTagID:   1,
	}
}

func (m *mockRepository) CreateProperty(property *Property) error {
	property.ID = m.nextPropID
	m.nextPropID++
	m.properties[property.ID] = property
	return nil
}

func (m *mockRepository) FindPropertyByID(id uint64) (*Property, error) {
	p, ok := m.properties[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return p, nil
}

func (m *mockRepository) UpdateProperty(property *Property) error {
	if _, ok := m.properties[property.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	m.properties[property.ID] = property
	return nil
}

func (m *mockRepository) ListProperties(page, perPage int, managerID *uint64, active *bool) ([]Property, int64, error) {
	var result []Property
	for _, p := range m.properties {
		if managerID != nil && (p.ManagerID == nil || *p.ManagerID != *managerID) {
			continue
		}
		if active != nil && p.IsActive != *active {
			continue
		}
		result = append(result, *p)
	}
	return result, int64(len(result)), nil
}

func (m *mockRepository) CreateUnit(unit *Unit) error {
	unit.ID = m.nextUnitID
	m.nextUnitID++
	m.units[unit.ID] = unit
	return nil
}

func (m *mockRepository) FindUnitByID(id uint64) (*Unit, error) {
	u, ok := m.units[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (m *mockRepository) UpdateUnit(unit *Unit) error {
	if _, ok := m.units[unit.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	m.units[unit.ID] = unit
	return nil
}

func (m *mockRepository) ListUnitsByProperty(propertyID uint64, page, perPage int) ([]Unit, int64, error) {
	var result []Unit
	for _, u := range m.units {
		if u.PropertyID == propertyID {
			result = append(result, *u)
		}
	}
	return result, int64(len(result)), nil
}

func (m *mockRepository) AssignStaff(assignment *PropertyStaffAssignment) error {
	assignment.ID = m.nextStaffID
	m.nextStaffID++
	m.staff = append(m.staff, *assignment)
	return nil
}

func (m *mockRepository) RemoveStaff(propertyID, userID uint64, role string) error {
	for i, a := range m.staff {
		if a.PropertyID == propertyID && a.UserID == userID && a.Role == role {
			m.staff = append(m.staff[:i], m.staff[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockRepository) ListStaffByProperty(propertyID uint64) ([]PropertyStaffAssignment, error) {
	var result []PropertyStaffAssignment
	for _, a := range m.staff {
		if a.PropertyID == propertyID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockRepository) IsStaffMember(propertyID, userID uint64) (bool, error) {
	for _, a := range m.staff {
		if a.PropertyID == propertyID && a.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockRepository) AddSkillTag(tag *TechnicianSkillTag) error {
	tag.ID = m.nextTagID
	m.nextTagID++
	m.skillTags = append(m.skillTags, *tag)
	return nil
}

func (m *mockRepository) RemoveSkillTag(userID uint64, tagName string) error {
	for i, t := range m.skillTags {
		if t.UserID == userID && t.Tag == tagName {
			m.skillTags = append(m.skillTags[:i], m.skillTags[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockRepository) ListSkillTagsByUser(userID uint64) ([]TechnicianSkillTag, error) {
	var result []TechnicianSkillTag
	for _, t := range m.skillTags {
		if t.UserID == userID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) FindTechniciansByPropertyAndSkill(propertyID uint64, skillTag string) ([]PropertyStaffAssignment, error) {
	var result []PropertyStaffAssignment
	for _, a := range m.staff {
		if a.PropertyID == propertyID && a.Role == "Technician" {
			for _, t := range m.skillTags {
				if t.UserID == a.UserID && t.Tag == skillTag {
					result = append(result, a)
					break
				}
			}
		}
	}
	return result, nil
}

func (m *mockRepository) GetDispatchCursor(propertyID uint64, skillTag string) (*DispatchCursor, error) {
	key := cursorKey(propertyID, skillTag)
	c, ok := m.cursors[key]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return c, nil
}

func (m *mockRepository) UpsertDispatchCursor(cursor *DispatchCursor) error {
	key := cursorKey(cursor.PropertyID, cursor.SkillTag)
	m.cursors[key] = cursor
	return nil
}

func cursorKey(propertyID uint64, skillTag string) string {
	return string(rune(propertyID)) + ":" + skillTag
}

// --- Tests ---

func TestCreateProperty(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	svc := NewService(repo, audit)

	req := CreatePropertyRequest{
		Name:         "Test Property",
		AddressLine1: "123 Main St",
		City:         "Springfield",
		State:        "IL",
		ZipCode:      "62701",
	}

	resp, appErr := svc.CreateProperty(req, 1, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Name != "Test Property" {
		t.Errorf("expected name 'Test Property', got '%s'", resp.Name)
	}
	if resp.UUID == "" {
		t.Error("expected UUID to be generated")
	}
	if resp.Timezone != "America/New_York" {
		t.Errorf("expected default timezone, got '%s'", resp.Timezone)
	}
}

func TestCreateProperty_ValidationErrors(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	svc := NewService(repo, audit)

	tests := []struct {
		name    string
		req     CreatePropertyRequest
		wantErr bool
	}{
		{
			name:    "empty name",
			req:     CreatePropertyRequest{Name: "", AddressLine1: "123 St", City: "City", State: "ST", ZipCode: "12345"},
			wantErr: true,
		},
		{
			name:    "empty address",
			req:     CreatePropertyRequest{Name: "Prop", AddressLine1: "", City: "City", State: "ST", ZipCode: "12345"},
			wantErr: true,
		},
		{
			name:    "valid",
			req:     CreatePropertyRequest{Name: "Prop", AddressLine1: "123 St", City: "City", State: "ST", ZipCode: "12345"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, appErr := svc.CreateProperty(tt.req, 1, "127.0.0.1", "req-1")
			if tt.wantErr && appErr == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && appErr != nil {
				t.Errorf("unexpected error: %v", appErr)
			}
		})
	}
}

func TestUpdateProperty_ManagerAuth(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	svc := NewService(repo, audit)

	managerID := uint64(5)
	req := CreatePropertyRequest{
		Name:         "Manager Property",
		AddressLine1: "456 Oak Ave",
		City:         "Chicago",
		State:        "IL",
		ZipCode:      "60601",
		ManagerID:    &managerID,
	}

	created, _ := svc.CreateProperty(req, 1, "127.0.0.1", "req-1")

	// Manager can update their own property
	newName := "Updated Property"
	_, appErr := svc.UpdateProperty(created.ID, UpdatePropertyRequest{Name: &newName}, 5, []string{common.RolePropertyManager}, "127.0.0.1", "req-2")
	if appErr != nil {
		t.Fatalf("manager should be able to update their own property: %v", appErr)
	}

	// Different manager cannot update
	_, appErr = svc.UpdateProperty(created.ID, UpdatePropertyRequest{Name: &newName}, 99, []string{common.RolePropertyManager}, "127.0.0.1", "req-3")
	if appErr == nil {
		t.Fatal("expected forbidden error for different manager")
	}
	if appErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN, got %s", appErr.Code)
	}

	// Admin can update any property
	_, appErr = svc.UpdateProperty(created.ID, UpdatePropertyRequest{Name: &newName}, 1, []string{common.RoleSystemAdmin}, "127.0.0.1", "req-4")
	if appErr != nil {
		t.Fatalf("admin should be able to update any property: %v", appErr)
	}
}

func TestCreateUnit(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	svc := NewService(repo, audit)

	// Create property first
	propReq := CreatePropertyRequest{
		Name:         "Unit Property",
		AddressLine1: "789 Elm St",
		City:         "Denver",
		State:        "CO",
		ZipCode:      "80201",
	}
	prop, _ := svc.CreateProperty(propReq, 1, "127.0.0.1", "req-1")

	// Create unit as admin
	unitReq := CreateUnitRequest{
		UnitNumber: "101A",
	}
	unit, appErr := svc.CreateUnit(prop.ID, unitReq, 1, []string{common.RoleSystemAdmin}, "127.0.0.1", "req-2")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if unit.UnitNumber != "101A" {
		t.Errorf("expected unit number '101A', got '%s'", unit.UnitNumber)
	}
	if unit.Status != common.UnitStatusVacant {
		t.Errorf("expected default status 'Vacant', got '%s'", unit.Status)
	}
}

func TestAssignStaff(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	svc := NewService(repo, audit)

	propReq := CreatePropertyRequest{
		Name:         "Staff Property",
		AddressLine1: "321 Pine St",
		City:         "Portland",
		State:        "OR",
		ZipCode:      "97201",
	}
	prop, _ := svc.CreateProperty(propReq, 1, "127.0.0.1", "req-1")

	staffReq := AssignStaffRequest{
		UserID: 10,
		Role:   "Technician",
	}
	assignment, appErr := svc.AssignStaff(prop.ID, staffReq, 1, []string{common.RoleSystemAdmin}, "127.0.0.1", "req-2")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if assignment.UserID != 10 {
		t.Errorf("expected user_id 10, got %d", assignment.UserID)
	}

	// List staff
	staffList, appErr := svc.ListStaffByProperty(prop.ID)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if len(staffList) != 1 {
		t.Errorf("expected 1 staff, got %d", len(staffList))
	}
}

func TestAddSkillTag(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	svc := NewService(repo, audit)

	tagReq := AddSkillTagRequest{Tag: "Plumbing"}
	tag, appErr := svc.AddSkillTag(10, tagReq, 1, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if tag.Tag != "Plumbing" {
		t.Errorf("expected tag 'Plumbing', got '%s'", tag.Tag)
	}

	// List tags
	tags, appErr := svc.ListSkillTagsByUser(10)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tags))
	}
}

func TestRemoveStaff(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	svc := NewService(repo, audit)

	propReq := CreatePropertyRequest{
		Name:         "Remove Staff Property",
		AddressLine1: "555 Maple St",
		City:         "Austin",
		State:        "TX",
		ZipCode:      "73301",
	}
	prop, _ := svc.CreateProperty(propReq, 1, "127.0.0.1", "req-1")

	staffReq := AssignStaffRequest{UserID: 20, Role: "Technician"}
	_, _ = svc.AssignStaff(prop.ID, staffReq, 1, []string{common.RoleSystemAdmin}, "127.0.0.1", "req-2")

	appErr := svc.RemoveStaff(prop.ID, 20, "Technician", 1, []string{common.RoleSystemAdmin}, "127.0.0.1", "req-3")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	staffList, _ := svc.ListStaffByProperty(prop.ID)
	if len(staffList) != 0 {
		t.Errorf("expected 0 staff after removal, got %d", len(staffList))
	}
}

func TestListProperties(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	svc := NewService(repo, audit)

	for i := 0; i < 3; i++ {
		req := CreatePropertyRequest{
			Name:         "Prop",
			AddressLine1: "Addr",
			City:         "City",
			State:        "ST",
			ZipCode:      "12345",
		}
		_, _ = svc.CreateProperty(req, 1, "127.0.0.1", "req")
	}

	properties, total, appErr := svc.ListProperties(1, 10, nil, nil)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(properties))
	}
}
