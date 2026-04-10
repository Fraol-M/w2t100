package users

import (
	"errors"
	"fmt"
	"strings"

	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
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

// Service provides business logic for user management.
type Service struct {
	repo       Repository
	audit      AuditLogger
	encryptor  FieldEncryptor
	authConfig config.AuthConfig
}

// NewService creates a new UserService.
func NewService(repo Repository, audit AuditLogger, encryptor FieldEncryptor, authCfg config.AuthConfig) *Service {
	return &Service{
		repo:       repo,
		audit:      audit,
		encryptor:  encryptor,
		authConfig: authCfg,
	}
}

// Create registers a new user with bcrypt-hashed password and optional role assignment.
func (s *Service) Create(req CreateUserRequest, actorID uint64, ip, requestID string) (*UserResponse, *common.AppError) {
	// Validate required fields
	var fieldErrs []common.FieldError
	if strings.TrimSpace(req.Username) == "" {
		fieldErrs = append(fieldErrs, common.FieldError{Field: "username", Message: "is required"})
	}
	if strings.TrimSpace(req.Email) == "" {
		fieldErrs = append(fieldErrs, common.FieldError{Field: "email", Message: "is required"})
	}
	if len(req.Password) < 8 {
		fieldErrs = append(fieldErrs, common.FieldError{Field: "password", Message: "must be at least 8 characters"})
	}
	if len(fieldErrs) > 0 {
		return nil, common.NewValidationError("Validation failed", fieldErrs...)
	}

	// Check username uniqueness
	if existing, _ := s.repo.FindByUsername(req.Username); existing != nil {
		return nil, common.NewConflictError("username already exists")
	}

	// Check email uniqueness
	if existing, _ := s.repo.FindByEmail(req.Email); existing != nil {
		return nil, common.NewConflictError("email already exists")
	}

	// Hash password with configured bcrypt cost
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.authConfig.BcryptCost)
	if err != nil {
		return nil, common.NewInternalError("failed to hash password")
	}

	user := &User{
		UUID:         uuid.New().String(),
		Username:     strings.TrimSpace(req.Username),
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		PasswordHash: string(hash),
		FirstName:    strings.TrimSpace(req.FirstName),
		LastName:     strings.TrimSpace(req.LastName),
		IsActive:     true,
	}

	// Encrypt phone if provided
	if req.Phone != "" {
		encrypted, keyVer, encErr := s.encryptor.Encrypt([]byte(req.Phone))
		if encErr != nil {
			return nil, common.NewInternalError("failed to encrypt phone number")
		}
		user.PhoneEncrypted = encrypted
		user.PhoneKeyVersion = &keyVer
	}

	if err := s.repo.Create(user); err != nil {
		return nil, common.NewInternalError("failed to create user")
	}

	// Assign roles if specified
	for _, roleName := range req.RoleNames {
		role, roleErr := s.repo.FindRoleByName(roleName)
		if roleErr != nil {
			continue // skip invalid roles
		}
		_ = s.repo.AssignRole(user.ID, role.ID)
	}

	// Re-fetch to get roles loaded
	user, err = s.repo.FindByID(user.ID)
	if err != nil {
		return nil, common.NewInternalError("failed to retrieve created user")
	}

	s.audit.Log(actorID, common.AuditActionCreate, "User", user.ID,
		fmt.Sprintf("Created user %s", user.Username), ip, requestID)

	phone := s.decryptPhone(user)
	resp := ToUserResponse(user, phone, false)
	return &resp, nil
}

// Update modifies an existing user's profile fields.
func (s *Service) Update(userID uint64, req UpdateUserRequest, actorID uint64, ip, requestID string) (*UserResponse, *common.AppError) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("User")
		}
		return nil, common.NewInternalError("failed to find user")
	}

	if req.FirstName != nil {
		user.FirstName = strings.TrimSpace(*req.FirstName)
	}
	if req.LastName != nil {
		user.LastName = strings.TrimSpace(*req.LastName)
	}
	if req.Email != nil {
		newEmail := strings.ToLower(strings.TrimSpace(*req.Email))
		if newEmail != user.Email {
			if existing, _ := s.repo.FindByEmail(newEmail); existing != nil && existing.ID != user.ID {
				return nil, common.NewConflictError("email already exists")
			}
			user.Email = newEmail
		}
	}
	if req.Phone != nil {
		phone := strings.TrimSpace(*req.Phone)
		if phone == "" {
			user.PhoneEncrypted = nil
			user.PhoneKeyVersion = nil
		} else {
			encrypted, keyVer, encErr := s.encryptor.Encrypt([]byte(phone))
			if encErr != nil {
				return nil, common.NewInternalError("failed to encrypt phone number")
			}
			user.PhoneEncrypted = encrypted
			user.PhoneKeyVersion = &keyVer
		}
	}

	if err := s.repo.Update(user); err != nil {
		return nil, common.NewInternalError("failed to update user")
	}

	s.audit.Log(actorID, common.AuditActionUpdate, "User", user.ID,
		fmt.Sprintf("Updated user %s", user.Username), ip, requestID)

	phone := s.decryptPhone(user)
	// Reveal phone if the actor is updating their own profile
	revealPhone := actorID == user.ID
	resp := ToUserResponse(user, phone, revealPhone)
	return &resp, nil
}

// GetByID retrieves a user by ID.
func (s *Service) GetByID(userID uint64, requesterID uint64) (*UserResponse, *common.AppError) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("User")
		}
		return nil, common.NewInternalError("failed to find user")
	}

	phone := s.decryptPhone(user)
	// Reveal phone only if the requester is viewing their own profile
	revealPhone := requesterID == user.ID
	resp := ToUserResponse(user, phone, revealPhone)
	return &resp, nil
}

// List retrieves a paginated list of users with optional filters.
func (s *Service) List(req ListUsersRequest) ([]UserResponse, int64, *common.AppError) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PerPage < 1 {
		req.PerPage = 20
	}
	if req.PerPage > 100 {
		req.PerPage = 100
	}

	users, total, err := s.repo.List(req.Page, req.PerPage, req.Role, req.Search, req.Active)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list users")
	}

	responses := make([]UserResponse, 0, len(users))
	for i := range users {
		phone := s.decryptPhone(&users[i])
		responses = append(responses, ToUserResponse(&users[i], phone, false))
	}

	return responses, total, nil
}

// ToggleActive activates or deactivates a user account.
func (s *Service) ToggleActive(userID uint64, isActive bool, actorID uint64, ip, requestID string) (*UserResponse, *common.AppError) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("User")
		}
		return nil, common.NewInternalError("failed to find user")
	}

	user.IsActive = isActive
	if err := s.repo.Update(user); err != nil {
		return nil, common.NewInternalError("failed to update user status")
	}

	action := "Activated"
	if !isActive {
		action = "Deactivated"
	}
	s.audit.Log(actorID, common.AuditActionStatusChange, "User", user.ID,
		fmt.Sprintf("%s user %s", action, user.Username), ip, requestID)

	phone := s.decryptPhone(user)
	resp := ToUserResponse(user, phone, false)
	return &resp, nil
}

// AssignRole assigns a role to a user by role name.
func (s *Service) AssignRole(userID uint64, roleName string, actorID uint64, ip, requestID string) *common.AppError {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.NewNotFoundError("User")
		}
		return common.NewInternalError("failed to find user")
	}

	role, err := s.repo.FindRoleByName(roleName)
	if err != nil {
		return common.NewNotFoundError("Role")
	}

	// Check if already assigned
	for _, r := range user.Roles {
		if r.ID == role.ID {
			return common.NewConflictError("role already assigned")
		}
	}

	if err := s.repo.AssignRole(userID, role.ID); err != nil {
		return common.NewInternalError("failed to assign role")
	}

	s.audit.Log(actorID, common.AuditActionUpdate, "User", user.ID,
		fmt.Sprintf("Assigned role %s to user %s", roleName, user.Username), ip, requestID)

	return nil
}

// RemoveRole removes a role from a user by role name.
func (s *Service) RemoveRole(userID uint64, roleName string, actorID uint64, ip, requestID string) *common.AppError {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.NewNotFoundError("User")
		}
		return common.NewInternalError("failed to find user")
	}

	role, err := s.repo.FindRoleByName(roleName)
	if err != nil {
		return common.NewNotFoundError("Role")
	}

	if err := s.repo.RemoveRole(userID, role.ID); err != nil {
		return common.NewInternalError("failed to remove role")
	}

	s.audit.Log(actorID, common.AuditActionUpdate, "User", user.ID,
		fmt.Sprintf("Removed role %s from user %s", roleName, user.Username), ip, requestID)

	return nil
}

// FindByUsername retrieves a user by username (used by auth).
func (s *Service) FindByUsername(username string) (*User, error) {
	return s.repo.FindByUsername(username)
}

// decryptPhone decrypts the phone field if present.
func (s *Service) decryptPhone(user *User) string {
	if user.PhoneEncrypted == nil || user.PhoneKeyVersion == nil {
		return ""
	}
	decrypted, err := s.encryptor.Decrypt(user.PhoneEncrypted, *user.PhoneKeyVersion)
	if err != nil {
		return ""
	}
	return string(decrypted)
}
