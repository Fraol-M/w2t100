package properties

import (
	"time"
)

// --- Property DTOs ---

// CreatePropertyRequest is the payload to create a new property.
type CreatePropertyRequest struct {
	Name         string  `json:"name" binding:"required"`
	AddressLine1 string  `json:"address_line1" binding:"required"`
	AddressLine2 string  `json:"address_line2"`
	City         string  `json:"city" binding:"required"`
	State        string  `json:"state" binding:"required"`
	ZipCode      string  `json:"zip_code" binding:"required"`
	ManagerID    *uint64 `json:"manager_id"`
	Timezone     string  `json:"timezone"`
}

// UpdatePropertyRequest is the payload to update a property.
type UpdatePropertyRequest struct {
	Name         *string `json:"name"`
	AddressLine1 *string `json:"address_line1"`
	AddressLine2 *string `json:"address_line2"`
	City         *string `json:"city"`
	State        *string `json:"state"`
	ZipCode      *string `json:"zip_code"`
	ManagerID    *uint64 `json:"manager_id"`
	Timezone     *string `json:"timezone"`
	IsActive     *bool   `json:"is_active"`
}

// PropertyResponse is the public representation of a property.
type PropertyResponse struct {
	ID           uint64    `json:"id"`
	UUID         string    `json:"uuid"`
	Name         string    `json:"name"`
	AddressLine1 string    `json:"address_line1"`
	AddressLine2 *string   `json:"address_line2,omitempty"`
	City         string    `json:"city"`
	State        string    `json:"state"`
	ZipCode      string    `json:"zip_code"`
	ManagerID    *uint64   `json:"manager_id"`
	Timezone     string    `json:"timezone"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ToPropertyResponse converts a Property model to a PropertyResponse DTO.
func ToPropertyResponse(p *Property) PropertyResponse {
	return PropertyResponse{
		ID:           p.ID,
		UUID:         p.UUID,
		Name:         p.Name,
		AddressLine1: p.AddressLine1,
		AddressLine2: p.AddressLine2,
		City:         p.City,
		State:        p.State,
		ZipCode:      p.ZipCode,
		ManagerID:    p.ManagerID,
		Timezone:     p.Timezone,
		IsActive:     p.IsActive,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

// --- Unit DTOs ---

// CreateUnitRequest is the payload to create a new unit.
type CreateUnitRequest struct {
	UnitNumber string `json:"unit_number" binding:"required"`
	Floor      *int   `json:"floor"`
	Bedrooms   *int   `json:"bedrooms"`
	Bathrooms  *int   `json:"bathrooms"`
	SquareFeet *int   `json:"square_feet"`
	Status     string `json:"status"`
}

// UpdateUnitRequest is the payload to update a unit.
type UpdateUnitRequest struct {
	UnitNumber *string `json:"unit_number"`
	Floor      *int    `json:"floor"`
	Bedrooms   *int    `json:"bedrooms"`
	Bathrooms  *int    `json:"bathrooms"`
	SquareFeet *int    `json:"square_feet"`
	Status     *string `json:"status"`
}

// UnitResponse is the public representation of a unit.
type UnitResponse struct {
	ID          uint64    `json:"id"`
	UUID        string    `json:"uuid"`
	PropertyID  uint64    `json:"property_id"`
	UnitNumber  string    `json:"unit_number"`
	Floor       *int      `json:"floor,omitempty"`
	Bedrooms    *int      `json:"bedrooms,omitempty"`
	Bathrooms   *int      `json:"bathrooms,omitempty"`
	SquareFeet  *int      `json:"square_feet,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ToUnitResponse converts a Unit model to a UnitResponse DTO.
func ToUnitResponse(u *Unit) UnitResponse {
	return UnitResponse{
		ID:          u.ID,
		UUID:        u.UUID,
		PropertyID:  u.PropertyID,
		UnitNumber:  u.UnitNumber,
		Floor:       u.Floor,
		Bedrooms:    u.Bedrooms,
		Bathrooms:   u.Bathrooms,
		SquareFeet:  u.SquareFeet,
		Status:      u.Status,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

// --- Staff Assignment DTOs ---

// AssignStaffRequest is the payload to assign a staff member to a property.
type AssignStaffRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"`
}

// StaffAssignmentResponse is the public representation of a staff assignment.
type StaffAssignmentResponse struct {
	ID         uint64    `json:"id"`
	PropertyID uint64    `json:"property_id"`
	UserID     uint64    `json:"user_id"`
	Role       string    `json:"role"`
	CreatedAt  time.Time `json:"created_at"`
}

// ToStaffAssignmentResponse converts a PropertyStaffAssignment model to a response DTO.
func ToStaffAssignmentResponse(a *PropertyStaffAssignment) StaffAssignmentResponse {
	return StaffAssignmentResponse{
		ID:         a.ID,
		PropertyID: a.PropertyID,
		UserID:     a.UserID,
		Role:       a.Role,
		CreatedAt:  a.CreatedAt,
	}
}

// --- Skill Tag DTOs ---

// AddSkillTagRequest is the payload to add a skill tag to a technician.
type AddSkillTagRequest struct {
	Tag string `json:"tag" binding:"required"`
}

// SkillTagResponse is the public representation of a skill tag.
type SkillTagResponse struct {
	ID        uint64    `json:"id"`
	UserID    uint64    `json:"user_id"`
	Tag       string    `json:"tag"`
	CreatedAt time.Time `json:"created_at"`
}

// ToSkillTagResponse converts a TechnicianSkillTag model to a response DTO.
func ToSkillTagResponse(s *TechnicianSkillTag) SkillTagResponse {
	return SkillTagResponse{
		ID:        s.ID,
		UserID:    s.UserID,
		Tag:       s.Tag,
		CreatedAt: s.CreatedAt,
	}
}
