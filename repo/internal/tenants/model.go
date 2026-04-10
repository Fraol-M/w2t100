package tenants

import (
	"time"

	"gorm.io/gorm"
)

// TenantProfile represents a tenant's profile linked to a user account and unit.
type TenantProfile struct {
	ID                          uint64         `gorm:"primaryKey" json:"id"`
	UUID                        string         `gorm:"size:36;uniqueIndex" json:"uuid"`
	UserID                      uint64         `gorm:"uniqueIndex" json:"user_id"`
	UnitID                      *uint64        `json:"unit_id"`
	EmergencyContactEncrypted   []byte         `gorm:"type:varbinary(512)" json:"-"`
	EmergencyContactKeyVersion  *int           `json:"-"`
	LeaseStart                  *time.Time     `gorm:"type:date" json:"lease_start"`
	LeaseEnd                    *time.Time     `gorm:"type:date" json:"lease_end"`
	MoveInDate                  *time.Time     `gorm:"type:date" json:"move_in_date"`
	Notes                       *string        `gorm:"type:text" json:"notes"`
	CreatedAt                   time.Time      `json:"created_at"`
	UpdatedAt                   time.Time      `json:"updated_at"`
	DeletedAt                   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the database table name for TenantProfile.
func (TenantProfile) TableName() string {
	return "tenant_profiles"
}
