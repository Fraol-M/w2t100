package payments

import (
	"encoding/json"
	"time"
)

// --- Request DTOs ---

// CreateIntentRequest is the payload for creating a payment intent.
type CreateIntentRequest struct {
	WorkOrderID *uint64 `json:"work_order_id,omitempty"`
	TenantID    *uint64 `json:"tenant_id,omitempty"`
	UnitID      *uint64 `json:"unit_id,omitempty"`
	PropertyID  uint64  `json:"property_id" binding:"required"`
	Amount      float64 `json:"amount" binding:"required"`
	Description string  `json:"description,omitempty"`
}

// MarkPaidRequest is the payload for marking a payment intent as paid.
type MarkPaidRequest struct {
	Notes string `json:"notes,omitempty"`
}

// CreateSettlementRequest is the payload for creating a settlement posting.
type CreateSettlementRequest struct {
	PaymentID   uint64  `json:"payment_id" binding:"required"`
	Amount      float64 `json:"amount" binding:"required"`
	Description string  `json:"description,omitempty"`
}

// CreateReversalRequest is the payload for reversing a payment.
type CreateReversalRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// CreateMakeupRequest is the payload for creating a makeup posting.
type CreateMakeupRequest struct {
	Amount      float64 `json:"amount" binding:"required"`
	Description string  `json:"description,omitempty"`
}

// ApprovePaymentRequest is the payload for approving a payment.
type ApprovePaymentRequest struct {
	Notes string `json:"notes,omitempty"`
}

// RunReconciliationRequest is the payload for triggering a reconciliation run.
type RunReconciliationRequest struct {
	RunDate string `json:"run_date" binding:"required"` // YYYY-MM-DD format
}

// ListPaymentsRequest holds filter and pagination parameters for listing payments.
type ListPaymentsRequest struct {
	PropertyID          *uint64  `form:"property_id"`
	PropertyIDs         []uint64 `form:"-"` // set programmatically for PM scoping
	ScopedToPropertyIDs bool     `form:"-"` // set when PM scope is active (empty list = no results)
	Status              string   `form:"status"`
	Kind                string   `form:"kind"`
	TenantID            *uint64  `form:"tenant_id"`
	Page                int      `form:"page"`
	PerPage             int      `form:"per_page"`
}

// --- Response DTOs ---

// PaymentResponse is the API response DTO for a payment.
type PaymentResponse struct {
	ID               uint64     `json:"id"`
	UUID             string     `json:"uuid"`
	WorkOrderID      *uint64    `json:"work_order_id,omitempty"`
	TenantID         *uint64    `json:"tenant_id,omitempty"`
	UnitID           *uint64    `json:"unit_id,omitempty"`
	PropertyID       uint64     `json:"property_id"`
	Kind             string     `json:"kind"`
	Amount           float64    `json:"amount"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"`
	Description      *string    `json:"description,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	PaidBy           *uint64    `json:"paid_by,omitempty"`
	ReversedAt       *time.Time `json:"reversed_at,omitempty"`
	ReversalReason   *string    `json:"reversal_reason,omitempty"`
	RelatedPaymentID *uint64    `json:"related_payment_id,omitempty"`
	CreatedBy        uint64     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// PaymentApprovalResponse is the API response DTO for a payment approval.
type PaymentApprovalResponse struct {
	ID            uint64    `json:"id"`
	PaymentID     uint64    `json:"payment_id"`
	ApproverID    uint64    `json:"approver_id"`
	ApprovalOrder uint8     `json:"approval_order"`
	Notes         *string   `json:"notes,omitempty"`
	ApprovedAt    time.Time `json:"approved_at"`
}

// ReconciliationResponse is the API response DTO for a reconciliation run.
type ReconciliationResponse struct {
	ID                uint64          `json:"id"`
	UUID              string          `json:"uuid"`
	RunDate           time.Time       `json:"run_date"`
	Status            string          `json:"status"`
	TotalExpected     float64         `json:"total_expected"`
	TotalActual       float64         `json:"total_actual"`
	DiscrepancyCount  int             `json:"discrepancy_count"`
	Summary           json.RawMessage `json:"summary,omitempty"`
	StatementFilePath *string         `json:"statement_file_path,omitempty"`
	GeneratedBy       uint64          `json:"generated_by"`
	StartedAt         time.Time       `json:"started_at"`
	CompletedAt       *time.Time      `json:"completed_at,omitempty"`
}

// --- Converters ---

// ToPaymentResponse converts a Payment model to its API response DTO.
func ToPaymentResponse(p *Payment) PaymentResponse {
	return PaymentResponse{
		ID:               p.ID,
		UUID:             p.UUID,
		WorkOrderID:      p.WorkOrderID,
		TenantID:         p.TenantID,
		UnitID:           p.UnitID,
		PropertyID:       p.PropertyID,
		Kind:             p.Kind,
		Amount:           p.Amount,
		Currency:         p.Currency,
		Status:           p.Status,
		Description:      p.Description,
		ExpiresAt:        p.ExpiresAt,
		PaidAt:           p.PaidAt,
		PaidBy:           p.PaidBy,
		ReversedAt:       p.ReversedAt,
		ReversalReason:   p.ReversalReason,
		RelatedPaymentID: p.RelatedPaymentID,
		CreatedBy:        p.CreatedBy,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}

// ToPaymentApprovalResponse converts a PaymentApproval model to its API response DTO.
func ToPaymentApprovalResponse(a *PaymentApproval) PaymentApprovalResponse {
	return PaymentApprovalResponse{
		ID:            a.ID,
		PaymentID:     a.PaymentID,
		ApproverID:    a.ApproverID,
		ApprovalOrder: a.ApprovalOrder,
		Notes:         a.Notes,
		ApprovedAt:    a.ApprovedAt,
	}
}

// ToReconciliationResponse converts a ReconciliationRun model to its API response DTO.
func ToReconciliationResponse(r *ReconciliationRun) ReconciliationResponse {
	return ReconciliationResponse{
		ID:                r.ID,
		UUID:              r.UUID,
		RunDate:           r.RunDate,
		Status:            r.Status,
		TotalExpected:     r.TotalExpected,
		TotalActual:       r.TotalActual,
		DiscrepancyCount:  r.DiscrepancyCount,
		Summary:           json.RawMessage(r.Summary),
		StatementFilePath: r.StatementFilePath,
		GeneratedBy:       r.GeneratedBy,
		StartedAt:         r.StartedAt,
		CompletedAt:       r.CompletedAt,
	}
}
