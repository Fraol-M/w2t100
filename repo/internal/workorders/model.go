package workorders

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// WorkOrder represents a maintenance work order in the system.
type WorkOrder struct {
	ID                       uint64          `gorm:"primaryKey" json:"id"`
	UUID                     string          `gorm:"size:36;uniqueIndex" json:"uuid"`
	PropertyID               uint64          `gorm:"index" json:"property_id"`
	UnitID                   *uint64         `json:"unit_id,omitempty"`
	TenantID                 uint64          `gorm:"index" json:"tenant_id"`
	AssignedTo               *uint64         `gorm:"index" json:"assigned_to,omitempty"`
	Title                    string          `gorm:"size:255" json:"title"`
	Description              string          `gorm:"type:text" json:"description"`
	Priority                 string          `gorm:"size:20;default:Normal" json:"priority"`
	Status                   string          `gorm:"size:30;default:New" json:"status"`
	IssueType                string          `gorm:"size:100" json:"issue_type,omitempty"`
	SkillTag                 string          `gorm:"size:100" json:"skill_tag,omitempty"`
	Tags                     datatypes.JSON  `gorm:"type:json" json:"tags,omitempty"`
	PreferredAccessDate      *time.Time      `gorm:"type:date" json:"preferred_access_date,omitempty"`
	PreferredAccessStartTime string          `gorm:"size:20" json:"preferred_access_start_time,omitempty"`
	PreferredAccessEndTime   string          `gorm:"size:20" json:"preferred_access_end_time,omitempty"`
	SLADueAt                 *time.Time      `json:"sla_due_at,omitempty"`
	SLABreachedAt            *time.Time      `json:"sla_breached_at,omitempty"`
	Rating                   *uint8          `json:"rating,omitempty"`
	Feedback                 *string         `gorm:"type:text" json:"feedback,omitempty"`
	CompletedAt              *time.Time      `json:"completed_at,omitempty"`
	ArchivedAt               *time.Time      `json:"archived_at,omitempty"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
	DeletedAt                gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName overrides the default table name.
func (WorkOrder) TableName() string {
	return "work_orders"
}

// WorkOrderEvent is an append-only audit trail entry for work order changes.
type WorkOrderEvent struct {
	ID          uint64         `gorm:"primaryKey" json:"id"`
	WorkOrderID uint64         `gorm:"index" json:"work_order_id"`
	ActorID     *uint64        `json:"actor_id,omitempty"`
	EventType   string         `gorm:"size:50" json:"event_type"`
	FromStatus  string         `gorm:"size:30" json:"from_status,omitempty"`
	ToStatus    string         `gorm:"size:30" json:"to_status,omitempty"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
	Metadata    datatypes.JSON `gorm:"type:json" json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// TableName overrides the default table name.
func (WorkOrderEvent) TableName() string {
	return "work_order_events"
}

// CostItem represents a cost line item associated with a work order.
type CostItem struct {
	ID             uint64    `gorm:"primaryKey" json:"id"`
	UUID           string    `gorm:"size:36;uniqueIndex" json:"uuid"`
	WorkOrderID    uint64    `gorm:"index" json:"work_order_id"`
	CostType       string    `gorm:"size:20" json:"cost_type"`
	Description    string    `gorm:"size:500" json:"description"`
	Amount         float64   `gorm:"type:decimal(12,2)" json:"amount"`
	Responsibility string    `gorm:"size:20;default:Property" json:"responsibility"`
	Notes          *string   `gorm:"type:text" json:"notes,omitempty"`
	CreatedBy      uint64    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TableName overrides the default table name.
func (CostItem) TableName() string {
	return "cost_items"
}
