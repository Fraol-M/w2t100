package properties

import (
	"time"

	"gorm.io/gorm"
)

// Property represents a managed property.
type Property struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	UUID         string         `gorm:"size:36;uniqueIndex" json:"uuid"`
	Name         string         `gorm:"size:255" json:"name"`
	AddressLine1 string         `gorm:"size:255" json:"address_line1"`
	AddressLine2 *string        `gorm:"size:255" json:"address_line2,omitempty"`
	City         string         `gorm:"size:100" json:"city"`
	State        string         `gorm:"size:50" json:"state"`
	ZipCode      string         `gorm:"size:20" json:"zip_code"`
	ManagerID    *uint64        `json:"manager_id"`
	Timezone     string         `gorm:"size:50;default:America/New_York" json:"timezone"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the database table name for Property.
func (Property) TableName() string {
	return "properties"
}

// Unit represents a unit within a property.
type Unit struct {
	ID          uint64         `gorm:"primaryKey" json:"id"`
	UUID        string         `gorm:"size:36;uniqueIndex" json:"uuid"`
	PropertyID  uint64         `json:"property_id"`
	UnitNumber  string         `gorm:"size:50" json:"unit_number"`
	Floor       *int           `json:"floor,omitempty"`
	Bedrooms    *int           `json:"bedrooms,omitempty"`
	Bathrooms   *int           `json:"bathrooms,omitempty"`
	SquareFeet  *int           `json:"square_feet,omitempty"`
	Status      string         `gorm:"size:20;default:Vacant" json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the database table name for Unit.
func (Unit) TableName() string {
	return "units"
}

// PropertyStaffAssignment represents a staff member assigned to a property.
type PropertyStaffAssignment struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	PropertyID uint64    `json:"property_id"`
	UserID     uint64    `json:"user_id"`
	Role       string    `gorm:"size:50" json:"role"`
	IsActive   bool      `gorm:"default:true" json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName returns the database table name for PropertyStaffAssignment.
func (PropertyStaffAssignment) TableName() string {
	return "property_staff_assignments"
}

// TechnicianSkillTag represents a skill tag assigned to a technician.
type TechnicianSkillTag struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	UserID    uint64    `json:"user_id"`
	Tag       string    `gorm:"size:100" json:"tag"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName returns the database table name for TechnicianSkillTag.
func (TechnicianSkillTag) TableName() string {
	return "technician_skill_tags"
}

// DispatchCursor tracks round-robin assignment state for a property and skill tag.
type DispatchCursor struct {
	ID                 uint64    `gorm:"primaryKey" json:"id"`
	PropertyID         uint64    `json:"property_id"`
	SkillTag           string    `gorm:"size:100" json:"skill_tag"`
	LastAssignedUserID *uint64   `json:"last_assigned_user_id"`
	CursorPosition     int       `gorm:"default:0" json:"cursor_position"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// TableName returns the database table name for DispatchCursor.
func (DispatchCursor) TableName() string {
	return "dispatch_cursors"
}
