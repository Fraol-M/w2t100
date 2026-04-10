package auth

import "time"

// Session represents a DB-backed user session with idle and absolute expiry.
type Session struct {
	ID                uint64     `gorm:"primaryKey"`
	TokenHash         string     `gorm:"size:128;uniqueIndex"`
	UserID            uint64     `gorm:"index"`
	IPAddress         string     `gorm:"size:45"`
	UserAgent         string     `gorm:"size:512"`
	CreatedAt         time.Time
	LastActiveAt      time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
}

// TableName overrides the default table name.
func (Session) TableName() string {
	return "sessions"
}

// User mirrors the users table for authentication queries.
// This is a read-only model used by the auth package; the canonical
// user model lives in the users package.
type User struct {
	ID           uint64     `gorm:"primaryKey"`
	UUID         string     `gorm:"size:36"`
	Username     string     `gorm:"size:100"`
	Email        string     `gorm:"size:255"`
	PasswordHash string     `gorm:"size:255"`
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time `gorm:"index"`
	Roles        []Role     `gorm:"many2many:user_roles;"`
}

// TableName overrides the default table name.
func (User) TableName() string {
	return "users"
}

// Role mirrors the roles table.
type Role struct {
	ID   uint64 `gorm:"primaryKey"`
	Name string `gorm:"size:50"`
}

// TableName overrides the default table name.
func (Role) TableName() string {
	return "roles"
}

// RoleNames returns a string slice of role names.
func (u *User) RoleNames() []string {
	names := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		names[i] = r.Name
	}
	return names
}
