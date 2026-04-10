package users

import (
	"time"

	"gorm.io/gorm"
)

// User represents a system user account.
type User struct {
	ID              uint64         `gorm:"primaryKey" json:"id"`
	UUID            string         `gorm:"size:36;uniqueIndex" json:"uuid"`
	Username        string         `gorm:"size:100;uniqueIndex" json:"username"`
	Email           string         `gorm:"size:255;uniqueIndex" json:"email"`
	PasswordHash    string         `gorm:"size:255" json:"-"`
	FirstName       string         `gorm:"size:100" json:"first_name"`
	LastName        string         `gorm:"size:100" json:"last_name"`
	PhoneEncrypted  []byte         `gorm:"type:varbinary(512)" json:"-"`
	PhoneKeyVersion *int           `json:"-"`
	IsActive        bool           `gorm:"default:true" json:"is_active"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	Roles           []Role         `gorm:"many2many:user_roles;" json:"roles,omitempty"`
}

// TableName returns the database table name for User.
func (User) TableName() string {
	return "users"
}

// Role represents a system role for RBAC.
type Role struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:50;uniqueIndex" json:"name"`
	Description string    `gorm:"size:255" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName returns the database table name for Role.
func (Role) TableName() string {
	return "roles"
}

// UserRole represents the join table between users and roles.
type UserRole struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	UserID    uint64    `json:"user_id"`
	RoleID    uint64    `json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName returns the database table name for UserRole.
func (UserRole) TableName() string {
	return "user_roles"
}
