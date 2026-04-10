package audit

import (
	"time"

	"gorm.io/datatypes"
)

// AuditLog records a single auditable action in the system.
type AuditLog struct {
	ID           uint64         `gorm:"primaryKey"`
	UUID         string         `gorm:"size:36;uniqueIndex"`
	ActorID      *uint64        `gorm:"index"`
	Action       string         `gorm:"size:50;index"`
	ResourceType string         `gorm:"size:50;index"`
	ResourceID   *uint64
	Description  string         `gorm:"type:text"`
	OldValues    datatypes.JSON
	NewValues    datatypes.JSON
	IPAddress    string         `gorm:"size:45"`
	RequestID    string         `gorm:"size:36;index"`
	CreatedAt    time.Time      `gorm:"index"`
}

// TableName overrides the default GORM table name.
func (AuditLog) TableName() string {
	return "audit_logs"
}
