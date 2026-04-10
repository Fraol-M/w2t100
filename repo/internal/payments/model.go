package payments

import (
	"time"

	"gorm.io/datatypes"
)

// Payment represents a financial transaction in the system.
type Payment struct {
	ID               uint64     `gorm:"primaryKey" json:"id"`
	UUID             string     `gorm:"size:36;uniqueIndex" json:"uuid"`
	WorkOrderID      *uint64    `gorm:"index" json:"work_order_id,omitempty"`
	TenantID         *uint64    `gorm:"index" json:"tenant_id,omitempty"`
	UnitID           *uint64    `json:"unit_id,omitempty"`
	PropertyID       uint64     `gorm:"index" json:"property_id"`
	Kind             string     `gorm:"size:30" json:"kind"`
	Amount           float64    `gorm:"type:decimal(12,2)" json:"amount"`
	Currency         string     `gorm:"size:3;default:USD" json:"currency"`
	Status           string     `gorm:"size:20;default:Pending" json:"status"`
	Description      *string    `gorm:"size:500" json:"description,omitempty"`
	ExpiresAt        *time.Time `gorm:"index" json:"expires_at,omitempty"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	PaidBy           *uint64    `json:"paid_by,omitempty"`
	ReversedAt       *time.Time `json:"reversed_at,omitempty"`
	ReversalReason   *string    `gorm:"type:text" json:"reversal_reason,omitempty"`
	RelatedPaymentID *uint64    `json:"related_payment_id,omitempty"`
	CreatedBy        uint64     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// TableName overrides the default table name.
func (Payment) TableName() string {
	return "payments"
}

// PaymentApproval tracks approvals for a payment (dual-approval for > threshold).
type PaymentApproval struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	PaymentID     uint64    `gorm:"index" json:"payment_id"`
	ApproverID    uint64    `gorm:"index" json:"approver_id"`
	ApprovalOrder uint8     `json:"approval_order"`
	Notes         *string   `gorm:"type:text" json:"notes,omitempty"`
	ApprovedAt    time.Time `json:"approved_at"`
}

// TableName overrides the default table name.
func (PaymentApproval) TableName() string {
	return "payment_approvals"
}

// ReconciliationRun represents a daily reconciliation run.
type ReconciliationRun struct {
	ID                uint64         `gorm:"primaryKey" json:"id"`
	UUID              string         `gorm:"size:36;uniqueIndex" json:"uuid"`
	RunDate           time.Time      `gorm:"type:date;index" json:"run_date"`
	Status            string         `gorm:"size:20;default:Running" json:"status"`
	TotalExpected     float64        `gorm:"type:decimal(12,2);default:0" json:"total_expected"`
	TotalActual       float64        `gorm:"type:decimal(12,2);default:0" json:"total_actual"`
	DiscrepancyCount  int            `gorm:"default:0" json:"discrepancy_count"`
	Summary           datatypes.JSON `gorm:"type:json" json:"summary,omitempty"`
	StatementFilePath *string        `gorm:"size:512" json:"statement_file_path,omitempty"`
	GeneratedBy       uint64         `json:"generated_by"`
	StartedAt         time.Time      `json:"started_at"`
	CompletedAt       *time.Time     `json:"completed_at,omitempty"`
}

// TableName overrides the default table name.
func (ReconciliationRun) TableName() string {
	return "reconciliation_runs"
}
