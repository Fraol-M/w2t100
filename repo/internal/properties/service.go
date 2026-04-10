package properties

import (
	"errors"
	"fmt"
	"strings"

	"propertyops/backend/internal/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLogger defines the interface for logging audit events.
type AuditLogger interface {
	Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string)
}

// Service provides business logic for property management.
type Service struct {
	repo  Repository
	audit AuditLogger
}

// NewService creates a new PropertyService.
func NewService(repo Repository, audit AuditLogger) *Service {
	return &Service{
		repo:  repo,
		audit: audit,
	}
}

// --- Property operations ---

// CreateProperty creates a new property.
func (s *Service) CreateProperty(req CreatePropertyRequest, actorID uint64, ip, requestID string) (*PropertyResponse, *common.AppError) {
	var fieldErrs []common.FieldError
	if strings.TrimSpace(req.Name) == "" {
		fieldErrs = append(fieldErrs, common.FieldError{Field: "name", Message: "is required"})
	}
	if strings.TrimSpace(req.AddressLine1) == "" {
		fieldErrs = append(fieldErrs, common.FieldError{Field: "address_line1", Message: "is required"})
	}
	if strings.TrimSpace(req.City) == "" {
		fieldErrs = append(fieldErrs, common.FieldError{Field: "city", Message: "is required"})
	}
	if strings.TrimSpace(req.State) == "" {
		fieldErrs = append(fieldErrs, common.FieldError{Field: "state", Message: "is required"})
	}
	if strings.TrimSpace(req.ZipCode) == "" {
		fieldErrs = append(fieldErrs, common.FieldError{Field: "zip_code", Message: "is required"})
	}
	if len(fieldErrs) > 0 {
		return nil, common.NewValidationError("Validation failed", fieldErrs...)
	}

	tz := strings.TrimSpace(req.Timezone)
	if tz == "" {
		tz = "America/New_York"
	}

	property := &Property{
		UUID:         uuid.New().String(),
		Name:         strings.TrimSpace(req.Name),
		AddressLine1: strings.TrimSpace(req.AddressLine1),
		City:         strings.TrimSpace(req.City),
		State:        strings.TrimSpace(req.State),
		ZipCode:      strings.TrimSpace(req.ZipCode),
		ManagerID:    req.ManagerID,
		Timezone:     tz,
		IsActive:     true,
	}

	if req.AddressLine2 != "" {
		addr2 := strings.TrimSpace(req.AddressLine2)
		property.AddressLine2 = &addr2
	}

	if err := s.repo.CreateProperty(property); err != nil {
		return nil, common.NewInternalError("failed to create property")
	}

	s.audit.Log(actorID, common.AuditActionCreate, "Property", property.ID,
		fmt.Sprintf("Created property %s", property.Name), ip, requestID)

	resp := ToPropertyResponse(property)
	return &resp, nil
}

// UpdateProperty updates an existing property.
func (s *Service) UpdateProperty(propertyID uint64, req UpdatePropertyRequest, actorID uint64, roles []string, ip, requestID string) (*PropertyResponse, *common.AppError) {
	property, err := s.repo.FindPropertyByID(propertyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("Property")
		}
		return nil, common.NewInternalError("failed to find property")
	}

	// Authorization: only admin or the assigned manager can update
	if !s.canManageProperty(actorID, roles, property) {
		return nil, common.NewForbiddenError("you do not have permission to manage this property")
	}

	if req.Name != nil {
		property.Name = strings.TrimSpace(*req.Name)
	}
	if req.AddressLine1 != nil {
		property.AddressLine1 = strings.TrimSpace(*req.AddressLine1)
	}
	if req.AddressLine2 != nil {
		addr2 := strings.TrimSpace(*req.AddressLine2)
		if addr2 == "" {
			property.AddressLine2 = nil
		} else {
			property.AddressLine2 = &addr2
		}
	}
	if req.City != nil {
		property.City = strings.TrimSpace(*req.City)
	}
	if req.State != nil {
		property.State = strings.TrimSpace(*req.State)
	}
	if req.ZipCode != nil {
		property.ZipCode = strings.TrimSpace(*req.ZipCode)
	}
	if req.ManagerID != nil {
		property.ManagerID = req.ManagerID
	}
	if req.Timezone != nil {
		property.Timezone = strings.TrimSpace(*req.Timezone)
	}
	if req.IsActive != nil {
		property.IsActive = *req.IsActive
	}

	if err := s.repo.UpdateProperty(property); err != nil {
		return nil, common.NewInternalError("failed to update property")
	}

	s.audit.Log(actorID, common.AuditActionUpdate, "Property", property.ID,
		fmt.Sprintf("Updated property %s", property.Name), ip, requestID)

	resp := ToPropertyResponse(property)
	return &resp, nil
}

// GetPropertyByID retrieves a property by ID.
func (s *Service) GetPropertyByID(propertyID uint64) (*PropertyResponse, *common.AppError) {
	property, err := s.repo.FindPropertyByID(propertyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("Property")
		}
		return nil, common.NewInternalError("failed to find property")
	}

	resp := ToPropertyResponse(property)
	return &resp, nil
}

// ListProperties retrieves a paginated list of properties.
func (s *Service) ListProperties(page, perPage int, managerID *uint64, active *bool) ([]PropertyResponse, int64, *common.AppError) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	properties, total, err := s.repo.ListProperties(page, perPage, managerID, active)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list properties")
	}

	responses := make([]PropertyResponse, 0, len(properties))
	for i := range properties {
		responses = append(responses, ToPropertyResponse(&properties[i]))
	}

	return responses, total, nil
}

// --- Unit operations ---

// CreateUnit creates a new unit within a property.
func (s *Service) CreateUnit(propertyID uint64, req CreateUnitRequest, actorID uint64, roles []string, ip, requestID string) (*UnitResponse, *common.AppError) {
	property, err := s.repo.FindPropertyByID(propertyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("Property")
		}
		return nil, common.NewInternalError("failed to find property")
	}

	if !s.canManageProperty(actorID, roles, property) {
		return nil, common.NewForbiddenError("you do not have permission to manage this property")
	}

	status := req.Status
	if status == "" {
		status = common.UnitStatusVacant
	}

	unit := &Unit{
		UUID:       uuid.New().String(),
		PropertyID: propertyID,
		UnitNumber: strings.TrimSpace(req.UnitNumber),
		Floor:      req.Floor,
		Bedrooms:   req.Bedrooms,
		Bathrooms:  req.Bathrooms,
		SquareFeet: req.SquareFeet,
		Status:     status,
	}

	if err := s.repo.CreateUnit(unit); err != nil {
		return nil, common.NewInternalError("failed to create unit")
	}

	s.audit.Log(actorID, common.AuditActionCreate, "Unit", unit.ID,
		fmt.Sprintf("Created unit %s in property %s", unit.UnitNumber, property.Name), ip, requestID)

	resp := ToUnitResponse(unit)
	return &resp, nil
}

// UpdateUnit updates an existing unit.
func (s *Service) UpdateUnit(unitID uint64, req UpdateUnitRequest, actorID uint64, roles []string, ip, requestID string) (*UnitResponse, *common.AppError) {
	unit, err := s.repo.FindUnitByID(unitID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("Unit")
		}
		return nil, common.NewInternalError("failed to find unit")
	}

	// Check authorization via property
	property, err := s.repo.FindPropertyByID(unit.PropertyID)
	if err != nil {
		return nil, common.NewInternalError("failed to find property for unit")
	}
	if !s.canManageProperty(actorID, roles, property) {
		return nil, common.NewForbiddenError("you do not have permission to manage this property")
	}

	if req.UnitNumber != nil {
		unit.UnitNumber = strings.TrimSpace(*req.UnitNumber)
	}
	if req.Floor != nil {
		unit.Floor = req.Floor
	}
	if req.Bedrooms != nil {
		unit.Bedrooms = req.Bedrooms
	}
	if req.Bathrooms != nil {
		unit.Bathrooms = req.Bathrooms
	}
	if req.SquareFeet != nil {
		unit.SquareFeet = req.SquareFeet
	}
	if req.Status != nil {
		unit.Status = *req.Status
	}

	if err := s.repo.UpdateUnit(unit); err != nil {
		return nil, common.NewInternalError("failed to update unit")
	}

	s.audit.Log(actorID, common.AuditActionUpdate, "Unit", unit.ID,
		fmt.Sprintf("Updated unit %s", unit.UnitNumber), ip, requestID)

	resp := ToUnitResponse(unit)
	return &resp, nil
}

// GetUnitByID retrieves a unit by ID.
func (s *Service) GetUnitByID(unitID uint64) (*UnitResponse, *common.AppError) {
	unit, err := s.repo.FindUnitByID(unitID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("Unit")
		}
		return nil, common.NewInternalError("failed to find unit")
	}

	resp := ToUnitResponse(unit)
	return &resp, nil
}

// ListUnitsByProperty lists units in a property.
func (s *Service) ListUnitsByProperty(propertyID uint64, page, perPage int) ([]UnitResponse, int64, *common.AppError) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	units, total, err := s.repo.ListUnitsByProperty(propertyID, page, perPage)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list units")
	}

	responses := make([]UnitResponse, 0, len(units))
	for i := range units {
		responses = append(responses, ToUnitResponse(&units[i]))
	}

	return responses, total, nil
}

// --- Staff Assignment operations ---

// AssignStaff assigns a staff member to a property.
func (s *Service) AssignStaff(propertyID uint64, req AssignStaffRequest, actorID uint64, roles []string, ip, requestID string) (*StaffAssignmentResponse, *common.AppError) {
	property, err := s.repo.FindPropertyByID(propertyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("Property")
		}
		return nil, common.NewInternalError("failed to find property")
	}

	if !s.canManageProperty(actorID, roles, property) {
		return nil, common.NewForbiddenError("you do not have permission to manage this property")
	}

	assignment := &PropertyStaffAssignment{
		PropertyID: propertyID,
		UserID:     req.UserID,
		Role:       req.Role,
	}

	if err := s.repo.AssignStaff(assignment); err != nil {
		return nil, common.NewInternalError("failed to assign staff")
	}

	s.audit.Log(actorID, common.AuditActionCreate, "PropertyStaffAssignment", assignment.ID,
		fmt.Sprintf("Assigned user %d as %s to property %s", req.UserID, req.Role, property.Name), ip, requestID)

	resp := ToStaffAssignmentResponse(assignment)
	return &resp, nil
}

// RemoveStaff removes a staff member from a property.
func (s *Service) RemoveStaff(propertyID, userID uint64, role string, actorID uint64, roles []string, ip, requestID string) *common.AppError {
	property, err := s.repo.FindPropertyByID(propertyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.NewNotFoundError("Property")
		}
		return common.NewInternalError("failed to find property")
	}

	if !s.canManageProperty(actorID, roles, property) {
		return common.NewForbiddenError("you do not have permission to manage this property")
	}

	if err := s.repo.RemoveStaff(propertyID, userID, role); err != nil {
		return common.NewInternalError("failed to remove staff")
	}

	s.audit.Log(actorID, common.AuditActionDelete, "PropertyStaffAssignment", 0,
		fmt.Sprintf("Removed user %d as %s from property %s", userID, role, property.Name), ip, requestID)

	return nil
}

// ListStaffByProperty lists all staff assigned to a property.
func (s *Service) ListStaffByProperty(propertyID uint64) ([]StaffAssignmentResponse, *common.AppError) {
	assignments, err := s.repo.ListStaffByProperty(propertyID)
	if err != nil {
		return nil, common.NewInternalError("failed to list staff")
	}

	responses := make([]StaffAssignmentResponse, 0, len(assignments))
	for i := range assignments {
		responses = append(responses, ToStaffAssignmentResponse(&assignments[i]))
	}

	return responses, nil
}

// --- Skill Tag operations ---

// AddSkillTag adds a skill tag to a technician.
func (s *Service) AddSkillTag(userID uint64, req AddSkillTagRequest, actorID uint64, ip, requestID string) (*SkillTagResponse, *common.AppError) {
	tag := &TechnicianSkillTag{
		UserID: userID,
		Tag:    strings.TrimSpace(req.Tag),
	}

	if err := s.repo.AddSkillTag(tag); err != nil {
		return nil, common.NewInternalError("failed to add skill tag")
	}

	s.audit.Log(actorID, common.AuditActionCreate, "TechnicianSkillTag", tag.ID,
		fmt.Sprintf("Added skill tag %s to user %d", req.Tag, userID), ip, requestID)

	resp := ToSkillTagResponse(tag)
	return &resp, nil
}

// RemoveSkillTag removes a skill tag from a technician.
func (s *Service) RemoveSkillTag(userID uint64, tagName string, actorID uint64, ip, requestID string) *common.AppError {
	if err := s.repo.RemoveSkillTag(userID, tagName); err != nil {
		return common.NewInternalError("failed to remove skill tag")
	}

	s.audit.Log(actorID, common.AuditActionDelete, "TechnicianSkillTag", 0,
		fmt.Sprintf("Removed skill tag %s from user %d", tagName, userID), ip, requestID)

	return nil
}

// ListSkillTagsByUser lists all skill tags for a technician.
func (s *Service) ListSkillTagsByUser(userID uint64) ([]SkillTagResponse, *common.AppError) {
	tags, err := s.repo.ListSkillTagsByUser(userID)
	if err != nil {
		return nil, common.NewInternalError("failed to list skill tags")
	}

	responses := make([]SkillTagResponse, 0, len(tags))
	for i := range tags {
		responses = append(responses, ToSkillTagResponse(&tags[i]))
	}

	return responses, nil
}

// CanManageProperty returns (true, nil) if the actor can manage the given property.
// Used by handlers for object-level authorization on property read/write operations.
func (s *Service) CanManageProperty(actorID uint64, roles []string, propertyID uint64) (bool, *common.AppError) {
	property, err := s.repo.FindPropertyByID(propertyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, common.NewNotFoundError("Property")
		}
		return false, common.NewInternalError("failed to find property")
	}
	return s.canManageProperty(actorID, roles, property), nil
}

// canManageProperty checks if the actor can manage the property.
// Admin can manage any property. The assigned manager can manage their own property.
func (s *Service) canManageProperty(actorID uint64, roles []string, property *Property) bool {
	if hasRole(roles, common.RoleSystemAdmin) {
		return true
	}
	if hasRole(roles, common.RolePropertyManager) && property.ManagerID != nil && *property.ManagerID == actorID {
		return true
	}
	return false
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
