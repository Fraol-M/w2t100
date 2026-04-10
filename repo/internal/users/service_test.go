package users

import (
	"errors"
	"testing"

	"propertyops/backend/internal/config"

	"golang.org/x/crypto/bcrypt"
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
	description  string
}

func (m *mockAuditLogger) Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string) {
	m.calls = append(m.calls, auditCall{
		actorID:      actorID,
		action:       action,
		resourceType: resourceType,
		resourceID:   resourceID,
		description:  description,
	})
}

// --- Mock FieldEncryptor ---

type mockEncryptor struct{}

func (m *mockEncryptor) Encrypt(plaintext []byte) ([]byte, int, error) {
	// Return plaintext reversed as "encrypted" with key version 1
	enc := make([]byte, len(plaintext))
	for i, b := range plaintext {
		enc[len(plaintext)-1-i] = b
	}
	return enc, 1, nil
}

func (m *mockEncryptor) Decrypt(ciphertext []byte, keyVersion int) ([]byte, error) {
	// Reverse to "decrypt"
	dec := make([]byte, len(ciphertext))
	for i, b := range ciphertext {
		dec[len(ciphertext)-1-i] = b
	}
	return dec, nil
}

// --- Mock Repository ---

type mockRepository struct {
	users     map[uint64]*User
	roles     map[string]*Role
	userRoles map[uint64][]uint64 // userID -> roleIDs
	nextID    uint64
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		users: make(map[uint64]*User),
		roles: map[string]*Role{
			"Tenant":          {ID: 1, Name: "Tenant", Description: "Tenant role"},
			"Technician":      {ID: 2, Name: "Technician", Description: "Technician role"},
			"PropertyManager": {ID: 3, Name: "PropertyManager", Description: "Property Manager role"},
			"SystemAdmin":     {ID: 4, Name: "SystemAdmin", Description: "System Admin role"},
		},
		userRoles: make(map[uint64][]uint64),
		nextID:    1,
	}
}

func (m *mockRepository) Create(user *User) error {
	user.ID = m.nextID
	m.nextID++
	m.users[user.ID] = user
	return nil
}

func (m *mockRepository) FindByID(id uint64) (*User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	// Attach roles
	u.Roles = m.getRolesForUser(id)
	return u, nil
}

func (m *mockRepository) FindByUsername(username string) (*User, error) {
	for _, u := range m.users {
		if u.Username == username {
			u.Roles = m.getRolesForUser(u.ID)
			return u, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockRepository) FindByEmail(email string) (*User, error) {
	for _, u := range m.users {
		if u.Email == email {
			u.Roles = m.getRolesForUser(u.ID)
			return u, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockRepository) Update(user *User) error {
	if _, ok := m.users[user.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	m.users[user.ID] = user
	return nil
}

func (m *mockRepository) List(page, perPage int, role, search string, active *bool) ([]User, int64, error) {
	var result []User
	for _, u := range m.users {
		result = append(result, *u)
	}
	return result, int64(len(result)), nil
}

func (m *mockRepository) AssignRole(userID, roleID uint64) error {
	for _, rid := range m.userRoles[userID] {
		if rid == roleID {
			return errors.New("role already assigned")
		}
	}
	m.userRoles[userID] = append(m.userRoles[userID], roleID)
	return nil
}

func (m *mockRepository) RemoveRole(userID, roleID uint64) error {
	roles := m.userRoles[userID]
	for i, rid := range roles {
		if rid == roleID {
			m.userRoles[userID] = append(roles[:i], roles[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockRepository) FindRoleByName(name string) (*Role, error) {
	r, ok := m.roles[name]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return r, nil
}

func (m *mockRepository) CountByRole(roleName string) (int64, error) {
	role, ok := m.roles[roleName]
	if !ok {
		return 0, nil
	}
	var count int64
	for _, roleIDs := range m.userRoles {
		for _, rid := range roleIDs {
			if rid == role.ID {
				count++
			}
		}
	}
	return count, nil
}

func (m *mockRepository) getRolesForUser(userID uint64) []Role {
	var roles []Role
	for _, rid := range m.userRoles[userID] {
		for _, r := range m.roles {
			if r.ID == rid {
				roles = append(roles, *r)
			}
		}
	}
	return roles
}

// --- Tests ---

func TestCreateUser_PasswordHashCost(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}
	authCfg := config.AuthConfig{BcryptCost: 12}

	svc := NewService(repo, audit, encryptor, authCfg)

	req := CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "securepassword123",
	}

	resp, appErr := svc.Create(req, 1, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	// Verify the password was hashed with the correct cost
	user := repo.users[resp.ID]
	cost, err := bcrypt.Cost([]byte(user.PasswordHash))
	if err != nil {
		t.Fatalf("failed to get bcrypt cost: %v", err)
	}
	if cost != 12 {
		t.Errorf("expected bcrypt cost 12, got %d", cost)
	}

	// Verify the password can be verified
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("securepassword123"))
	if err != nil {
		t.Error("password verification failed")
	}
}

func TestCreateUser_ValidationErrors(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}
	authCfg := config.AuthConfig{BcryptCost: 12}

	svc := NewService(repo, audit, encryptor, authCfg)

	tests := []struct {
		name    string
		req     CreateUserRequest
		wantErr bool
	}{
		{
			name:    "empty username",
			req:     CreateUserRequest{Username: "", Email: "test@example.com", Password: "password123"},
			wantErr: true,
		},
		{
			name:    "empty email",
			req:     CreateUserRequest{Username: "user", Email: "", Password: "password123"},
			wantErr: true,
		},
		{
			name:    "short password",
			req:     CreateUserRequest{Username: "user", Email: "test@example.com", Password: "short"},
			wantErr: true,
		},
		{
			name:    "valid request",
			req:     CreateUserRequest{Username: "user", Email: "test@example.com", Password: "password123"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, appErr := svc.Create(tt.req, 1, "127.0.0.1", "req-1")
			if tt.wantErr && appErr == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && appErr != nil {
				t.Errorf("unexpected error: %v", appErr)
			}
		})
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}
	authCfg := config.AuthConfig{BcryptCost: 12}

	svc := NewService(repo, audit, encryptor, authCfg)

	req := CreateUserRequest{
		Username: "duplicate",
		Email:    "first@example.com",
		Password: "password123",
	}

	_, appErr := svc.Create(req, 1, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("first create failed: %v", appErr)
	}

	// Try creating another user with the same username
	req.Email = "second@example.com"
	_, appErr = svc.Create(req, 1, "127.0.0.1", "req-2")
	if appErr == nil {
		t.Fatal("expected conflict error for duplicate username")
	}
	if appErr.Code != "CONFLICT" {
		t.Errorf("expected CONFLICT error code, got %s", appErr.Code)
	}
}

func TestAssignRole(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}
	authCfg := config.AuthConfig{BcryptCost: 12}

	svc := NewService(repo, audit, encryptor, authCfg)

	// Create a user first
	req := CreateUserRequest{
		Username: "roleuser",
		Email:    "role@example.com",
		Password: "password123",
	}
	resp, _ := svc.Create(req, 1, "127.0.0.1", "req-1")

	// Assign Tenant role
	appErr := svc.AssignRole(resp.ID, "Tenant", 1, "127.0.0.1", "req-2")
	if appErr != nil {
		t.Fatalf("assign role failed: %v", appErr)
	}

	// Verify role was assigned
	user, _ := svc.GetByID(resp.ID, 1)
	found := false
	for _, r := range user.Roles {
		if r == "Tenant" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Tenant role to be assigned")
	}

	// Assign duplicate role should fail
	appErr = svc.AssignRole(resp.ID, "Tenant", 1, "127.0.0.1", "req-3")
	if appErr == nil {
		t.Error("expected conflict error for duplicate role assignment")
	}
}

func TestAssignRole_InvalidRole(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}
	authCfg := config.AuthConfig{BcryptCost: 12}

	svc := NewService(repo, audit, encryptor, authCfg)

	// Create a user
	req := CreateUserRequest{
		Username: "testuser2",
		Email:    "test2@example.com",
		Password: "password123",
	}
	resp, _ := svc.Create(req, 1, "127.0.0.1", "req-1")

	// Try to assign non-existent role
	appErr := svc.AssignRole(resp.ID, "NonExistent", 1, "127.0.0.1", "req-2")
	if appErr == nil {
		t.Error("expected not found error for invalid role")
	}
	if appErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND error code, got %s", appErr.Code)
	}
}

func TestRemoveRole(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}
	authCfg := config.AuthConfig{BcryptCost: 12}

	svc := NewService(repo, audit, encryptor, authCfg)

	// Create a user and assign a role
	req := CreateUserRequest{
		Username: "removeuser",
		Email:    "remove@example.com",
		Password: "password123",
	}
	resp, _ := svc.Create(req, 1, "127.0.0.1", "req-1")
	_ = svc.AssignRole(resp.ID, "Tenant", 1, "127.0.0.1", "req-2")

	// Remove the role
	appErr := svc.RemoveRole(resp.ID, "Tenant", 1, "127.0.0.1", "req-3")
	if appErr != nil {
		t.Fatalf("remove role failed: %v", appErr)
	}

	// Verify role was removed
	user, _ := svc.GetByID(resp.ID, 1)
	for _, r := range user.Roles {
		if r == "Tenant" {
			t.Error("expected Tenant role to be removed")
		}
	}
}

func TestToggleActive(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}
	authCfg := config.AuthConfig{BcryptCost: 12}

	svc := NewService(repo, audit, encryptor, authCfg)

	req := CreateUserRequest{
		Username: "activeuser",
		Email:    "active@example.com",
		Password: "password123",
	}
	resp, _ := svc.Create(req, 1, "127.0.0.1", "req-1")

	// Deactivate
	updated, appErr := svc.ToggleActive(resp.ID, false, 1, "127.0.0.1", "req-2")
	if appErr != nil {
		t.Fatalf("toggle active failed: %v", appErr)
	}
	if updated.IsActive {
		t.Error("expected user to be deactivated")
	}

	// Reactivate
	updated, appErr = svc.ToggleActive(resp.ID, true, 1, "127.0.0.1", "req-3")
	if appErr != nil {
		t.Fatalf("toggle active failed: %v", appErr)
	}
	if !updated.IsActive {
		t.Error("expected user to be activated")
	}
}

func TestPhoneEncryptionAndMasking(t *testing.T) {
	repo := newMockRepository()
	audit := &mockAuditLogger{}
	encryptor := &mockEncryptor{}
	authCfg := config.AuthConfig{BcryptCost: 12}

	svc := NewService(repo, audit, encryptor, authCfg)

	req := CreateUserRequest{
		Username: "phoneuser",
		Email:    "phone@example.com",
		Password: "password123",
		Phone:    "5551234567",
	}

	resp, appErr := svc.Create(req, 1, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("create failed: %v", appErr)
	}

	// Phone should be masked in the response (not the actor's own profile during create)
	if resp.Phone == "5551234567" {
		t.Error("expected phone to be masked in response")
	}
	if resp.Phone == "" {
		t.Error("expected masked phone, got empty string")
	}

	// When the user views their own profile, phone should be revealed
	ownResp, appErr := svc.GetByID(resp.ID, resp.ID)
	if appErr != nil {
		t.Fatalf("get failed: %v", appErr)
	}
	if ownResp.Phone != "5551234567" {
		t.Errorf("expected revealed phone '5551234567', got '%s'", ownResp.Phone)
	}

	// When another user views the profile, phone should be masked
	otherResp, appErr := svc.GetByID(resp.ID, 999)
	if appErr != nil {
		t.Fatalf("get failed: %v", appErr)
	}
	if otherResp.Phone == "5551234567" {
		t.Error("expected phone to be masked when viewed by another user")
	}
}
