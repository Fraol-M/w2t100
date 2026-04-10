package tenants

import (
	"errors"
	"fmt"
	"time"

	"propertyops/backend/internal/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLogger defines the interface for logging audit events.
type AuditLogger interface {
	Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string)
}

// FieldEncryptor defines the interface for encrypting and decrypting sensitive fields.
type FieldEncryptor interface {
	Encrypt(plaintext []byte) ([]byte, int, error)
	Decrypt(ciphertext []byte, keyVersion int) ([]byte, error)
}

// Service provides business logic for tenant profile management.
type Service struct {
	repo      Repository
	audit     AuditLogger
	encryptor FieldEncryptor
}

// NewService creates a new TenantService.
func NewService(repo Repository, audit AuditLogger, encryptor FieldEncryptor) *Service {
	return &Service{
		repo:      repo,
		audit:     audit,
		encryptor: encryptor,
	}
}

// CreateProfile creates a new tenant profile.
func (s *Service) CreateProfile(req CreateTenantProfileRequest, actorID uint64, ip, requestID string) (*TenantProfileResponse, *common.AppError) {
	// Check if profile already exists for this user
	if existing, _ := s.repo.FindByUserID(req.UserID); existing != nil {
		return nil, common.NewConflictError("tenant profile already exists for this user")
	}

	profile := &TenantProfile{
		UUID:   uuid.New().String(),
		UserID: req.UserID,
		UnitID: req.UnitID,
	}

	// Encrypt emergency contact if provided
	if req.EmergencyContact != "" {
		encrypted, keyVer, err := s.encryptor.Encrypt([]byte(req.EmergencyContact))
		if err != nil {
			return nil, common.NewInternalError("failed to encrypt emergency contact")
		}
		profile.EmergencyContactEncrypted = encrypted
		profile.EmergencyContactKeyVersion = &keyVer
	}

	// Parse dates
	if req.LeaseStart != "" {
		t, err := time.Parse("2006-01-02", req.LeaseStart)
		if err != nil {
			return nil, common.NewValidationError("Validation failed",
				common.FieldError{Field: "lease_start", Message: "must be in YYYY-MM-DD format"})
		}
		profile.LeaseStart = &t
	}
	if req.LeaseEnd != "" {
		t, err := time.Parse("2006-01-02", req.LeaseEnd)
		if err != nil {
			return nil, common.NewValidationError("Validation failed",
				common.FieldError{Field: "lease_end", Message: "must be in YYYY-MM-DD format"})
		}
		profile.LeaseEnd = &t
	}
	if req.MoveInDate != "" {
		t, err := time.Parse("2006-01-02", req.MoveInDate)
		if err != nil {
			return nil, common.NewValidationError("Validation failed",
				common.FieldError{Field: "move_in_date", Message: "must be in YYYY-MM-DD format"})
		}
		profile.MoveInDate = &t
	}
	if req.Notes != "" {
		profile.Notes = &req.Notes
	}

	if err := s.repo.Create(profile); err != nil {
		return nil, common.NewInternalError("failed to create tenant profile")
	}

	s.audit.Log(actorID, common.AuditActionCreate, "TenantProfile", profile.ID,
		fmt.Sprintf("Created tenant profile for user %d", req.UserID), ip, requestID)

	contact := s.decryptContact(profile)
	resp := ToTenantProfileResponse(profile, contact, actorID == profile.UserID)
	return &resp, nil
}

// UpdateProfile updates an existing tenant profile with object-level authorization.
func (s *Service) UpdateProfile(profileID uint64, req UpdateTenantProfileRequest, actorID uint64, roles []string, ip, requestID string) (*TenantProfileResponse, *common.AppError) {
	profile, err := s.repo.FindByID(profileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("TenantProfile")
		}
		return nil, common.NewInternalError("failed to find tenant profile")
	}

	// Object-level auth: tenants can only update their own profile
	if !s.canAccessProfile(actorID, roles, profile) {
		return nil, common.NewForbiddenError("you can only update your own profile")
	}

	if req.UnitID != nil {
		profile.UnitID = req.UnitID
	}
	if req.EmergencyContact != nil {
		contact := *req.EmergencyContact
		if contact == "" {
			profile.EmergencyContactEncrypted = nil
			profile.EmergencyContactKeyVersion = nil
		} else {
			encrypted, keyVer, encErr := s.encryptor.Encrypt([]byte(contact))
			if encErr != nil {
				return nil, common.NewInternalError("failed to encrypt emergency contact")
			}
			profile.EmergencyContactEncrypted = encrypted
			profile.EmergencyContactKeyVersion = &keyVer
		}
	}
	if req.LeaseStart != nil {
		if *req.LeaseStart == "" {
			profile.LeaseStart = nil
		} else {
			t, parseErr := time.Parse("2006-01-02", *req.LeaseStart)
			if parseErr != nil {
				return nil, common.NewValidationError("Validation failed",
					common.FieldError{Field: "lease_start", Message: "must be in YYYY-MM-DD format"})
			}
			profile.LeaseStart = &t
		}
	}
	if req.LeaseEnd != nil {
		if *req.LeaseEnd == "" {
			profile.LeaseEnd = nil
		} else {
			t, parseErr := time.Parse("2006-01-02", *req.LeaseEnd)
			if parseErr != nil {
				return nil, common.NewValidationError("Validation failed",
					common.FieldError{Field: "lease_end", Message: "must be in YYYY-MM-DD format"})
			}
			profile.LeaseEnd = &t
		}
	}
	if req.MoveInDate != nil {
		if *req.MoveInDate == "" {
			profile.MoveInDate = nil
		} else {
			t, parseErr := time.Parse("2006-01-02", *req.MoveInDate)
			if parseErr != nil {
				return nil, common.NewValidationError("Validation failed",
					common.FieldError{Field: "move_in_date", Message: "must be in YYYY-MM-DD format"})
			}
			profile.MoveInDate = &t
		}
	}
	if req.Notes != nil {
		if *req.Notes == "" {
			profile.Notes = nil
		} else {
			profile.Notes = req.Notes
		}
	}

	if err := s.repo.Update(profile); err != nil {
		return nil, common.NewInternalError("failed to update tenant profile")
	}

	s.audit.Log(actorID, common.AuditActionUpdate, "TenantProfile", profile.ID,
		fmt.Sprintf("Updated tenant profile %d", profile.ID), ip, requestID)

	contact := s.decryptContact(profile)
	resp := ToTenantProfileResponse(profile, contact, actorID == profile.UserID)
	return &resp, nil
}

// GetByID retrieves a tenant profile by ID with object-level authorization.
func (s *Service) GetByID(profileID uint64, actorID uint64, roles []string) (*TenantProfileResponse, *common.AppError) {
	profile, err := s.repo.FindByID(profileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("TenantProfile")
		}
		return nil, common.NewInternalError("failed to find tenant profile")
	}

	if !s.canAccessProfile(actorID, roles, profile) {
		return nil, common.NewForbiddenError("access denied")
	}

	contact := s.decryptContact(profile)
	revealContact := actorID == profile.UserID || hasRole(roles, common.RoleSystemAdmin)
	resp := ToTenantProfileResponse(profile, contact, revealContact)
	return &resp, nil
}

// GetByUserID retrieves a tenant profile by user ID with object-level authorization.
func (s *Service) GetByUserID(userID uint64, actorID uint64, roles []string) (*TenantProfileResponse, *common.AppError) {
	profile, err := s.repo.FindByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("TenantProfile")
		}
		return nil, common.NewInternalError("failed to find tenant profile")
	}

	if !s.canAccessProfile(actorID, roles, profile) {
		return nil, common.NewForbiddenError("access denied")
	}

	contact := s.decryptContact(profile)
	revealContact := actorID == profile.UserID || hasRole(roles, common.RoleSystemAdmin)
	resp := ToTenantProfileResponse(profile, contact, revealContact)
	return &resp, nil
}

// ListByProperty lists tenant profiles for a given property with pagination.
func (s *Service) ListByProperty(propertyID uint64, page, perPage int) ([]TenantProfileResponse, int64, *common.AppError) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	profiles, total, err := s.repo.FindByProperty(propertyID, page, perPage)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list tenant profiles")
	}

	responses := make([]TenantProfileResponse, 0, len(profiles))
	for i := range profiles {
		contact := s.decryptContact(&profiles[i])
		responses = append(responses, ToTenantProfileResponse(&profiles[i], contact, false))
	}

	return responses, total, nil
}

// IsPMForProperty returns true if pmID is an active staff member for propertyID.
// Used by handlers to enforce PM scope on list-by-property endpoints.
func (s *Service) IsPMForProperty(propertyID, pmID uint64) bool {
	return s.repo.IsPMForProperty(propertyID, pmID)
}

// canAccessProfile checks if the actor has permission to access the tenant profile.
// Tenants can only access their own profile. SystemAdmin can access any. PropertyManager
// can only access profiles for tenants whose unit belongs to a property they manage.
func (s *Service) canAccessProfile(actorID uint64, roles []string, profile *TenantProfile) bool {
	// Owner can always access their own profile.
	if actorID == profile.UserID {
		return true
	}
	// SystemAdmin can access any profile.
	if hasRole(roles, common.RoleSystemAdmin) {
		return true
	}
	// PropertyManager may only access profiles for tenants in their managed properties.
	if hasRole(roles, common.RolePropertyManager) && profile.UnitID != nil {
		return s.repo.IsPMForUnit(*profile.UnitID, actorID)
	}
	return false
}

// decryptContact decrypts the emergency contact field if present.
func (s *Service) decryptContact(profile *TenantProfile) string {
	if profile.EmergencyContactEncrypted == nil || profile.EmergencyContactKeyVersion == nil {
		return ""
	}
	decrypted, err := s.encryptor.Decrypt(profile.EmergencyContactEncrypted, *profile.EmergencyContactKeyVersion)
	if err != nil {
		return ""
	}
	return string(decrypted)
}

// hasRole checks if a slice of role names contains the specified role.
func hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
