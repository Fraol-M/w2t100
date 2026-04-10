package payments

import (
	"fmt"
	"log"
	"time"

	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLogger abstracts the audit logging dependency.
type AuditLogger interface {
	Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string)
}

// NotificationSender abstracts the notification sending dependency.
type NotificationSender interface {
	SendEvent(eventName string, recipientID uint64, data map[string]string) error
}

// Service handles payment business logic.
type Service struct {
	repo     *Repository
	recon    *ReconciliationService
	audit    AuditLogger
	notifier NotificationSender
	db       *gorm.DB
	cfg      *config.Config
}

// NewService creates a new payment Service.
func NewService(repo *Repository, audit AuditLogger, notifier NotificationSender, db *gorm.DB, cfg *config.Config) *Service {
	return &Service{
		repo:     repo,
		recon:    NewReconciliationService(repo, cfg),
		audit:    audit,
		notifier: notifier,
		db:       db,
		cfg:      cfg,
	}
}

// GetManagedPropertyIDs returns the property IDs where userID has an active PropertyManager assignment.
func (s *Service) GetManagedPropertyIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	err := s.db.Table("property_staff_assignments").
		Where("user_id = ? AND role = ? AND is_active = ?", userID, "PropertyManager", true).
		Pluck("property_id", &ids).Error
	return ids, err
}

// IsManagedBy checks whether userID has an active PropertyManager assignment on the given property.
func (s *Service) IsManagedBy(propertyID, userID uint64) (bool, error) {
	var count int64
	err := s.db.Table("property_staff_assignments").
		Where("property_id = ? AND user_id = ? AND role = ? AND is_active = ?", propertyID, userID, "PropertyManager", true).
		Count(&count).Error
	return count > 0, err
}

// CreateIntent creates a new payment intent with Pending status and an expiration time.
func (s *Service) CreateIntent(req CreateIntentRequest, actorID uint64, ip, requestID string) (*Payment, *common.AppError) {
	// Validate amount.
	if fe := common.ValidateMoneyAmount("amount", req.Amount); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	if req.PropertyID == 0 {
		return nil, common.NewFieldValidationError("property_id", "is required")
	}

	expiryMinutes := 30
	if s.cfg != nil && s.cfg.Payment.IntentExpiryMinutes > 0 {
		expiryMinutes = s.cfg.Payment.IntentExpiryMinutes
	}

	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(expiryMinutes) * time.Minute)

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	payment := &Payment{
		UUID:        uuid.New().String(),
		WorkOrderID: req.WorkOrderID,
		TenantID:    req.TenantID,
		UnitID:      req.UnitID,
		PropertyID:  req.PropertyID,
		Kind:        common.PaymentKindIntent,
		Amount:      req.Amount,
		Currency:    "USD",
		Status:      common.PaymentStatusPending,
		Description: desc,
		ExpiresAt:   &expiresAt,
		CreatedBy:   actorID,
	}

	if err := s.repo.Create(payment); err != nil {
		return nil, common.NewInternalError("failed to create payment intent")
	}

	s.audit.Log(actorID, common.AuditActionCreate, "Payment", payment.ID,
		fmt.Sprintf("Payment intent created: $%.2f", req.Amount), ip, requestID)

	return payment, nil
}

// GetPayment retrieves a payment by its ID.
func (s *Service) GetPayment(id uint64) (*Payment, *common.AppError) {
	payment, err := s.repo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, common.NewNotFoundError("Payment")
		}
		return nil, common.NewInternalError("failed to retrieve payment")
	}
	return payment, nil
}

// ListPayments retrieves a paginated list of payments matching the given filters.
func (s *Service) ListPayments(req ListPaymentsRequest) ([]Payment, int64, *common.AppError) {
	page, perPage := req.Page, req.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	filters := PaymentFilters{
		PropertyID:          req.PropertyID,
		PropertyIDs:         req.PropertyIDs,
		ScopedToPropertyIDs: req.ScopedToPropertyIDs,
		Status:              req.Status,
		Kind:                req.Kind,
		TenantID:            req.TenantID,
	}

	offset := (page - 1) * perPage
	payments, total, err := s.repo.List(filters, offset, perPage)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list payments")
	}

	return payments, total, nil
}

// MarkPaid transitions an Intent from Pending to Paid. Only PropertyManager/SystemAdmin.
func (s *Service) MarkPaid(paymentID uint64, req MarkPaidRequest, actorID uint64, ip, requestID string) (*Payment, *common.AppError) {
	payment, appErr := s.GetPayment(paymentID)
	if appErr != nil {
		return nil, appErr
	}

	if payment.Kind != common.PaymentKindIntent {
		return nil, common.NewValidationError("only Intent payments can be marked as paid")
	}
	if payment.Status != common.PaymentStatusPending {
		return nil, common.NewValidationError("only Pending intents can be marked as paid")
	}

	// Check if expired.
	if payment.ExpiresAt != nil && payment.ExpiresAt.Before(time.Now().UTC()) {
		return nil, common.NewValidationError("payment intent has expired")
	}

	now := time.Now().UTC()
	payment.Status = common.PaymentStatusPaid
	payment.PaidAt = &now
	payment.PaidBy = &actorID

	if err := s.repo.Update(payment); err != nil {
		return nil, common.NewInternalError("failed to mark payment as paid")
	}

	s.audit.Log(actorID, common.AuditActionStatusChange, "Payment", payment.ID,
		"Payment intent marked as paid", ip, requestID)

	// Notify tenant if applicable.
	if s.notifier != nil && payment.TenantID != nil {
		if err := s.notifier.SendEvent("payment_marked_paid", *payment.TenantID, map[string]string{
			"payment_id": fmt.Sprintf("%d", payment.ID),
			"amount":     fmt.Sprintf("%.2f", payment.Amount),
		}); err != nil {
			log.Printf("WARN payments: notification send failed for payment_marked_paid (payment %d): %v", payment.ID, err)
		}
	}

	return payment, nil
}

// CreateSettlement creates a SettlementPosting linked to a paid intent.
func (s *Service) CreateSettlement(req CreateSettlementRequest, actorID uint64, ip, requestID string) (*Payment, *common.AppError) {
	if fe := common.ValidateMoneyAmount("amount", req.Amount); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	// Verify related payment exists and is paid.
	related, appErr := s.GetPayment(req.PaymentID)
	if appErr != nil {
		return nil, appErr
	}
	if related.Status != common.PaymentStatusPaid {
		return nil, common.NewValidationError("settlement can only be created for paid payments")
	}

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	settlement := &Payment{
		UUID:             uuid.New().String(),
		WorkOrderID:      related.WorkOrderID,
		TenantID:         related.TenantID,
		UnitID:           related.UnitID,
		PropertyID:       related.PropertyID,
		Kind:             common.PaymentKindSettlementPosting,
		Amount:           req.Amount,
		Currency:         "USD",
		Status:           common.PaymentStatusSettled,
		Description:      desc,
		RelatedPaymentID: &related.ID,
		CreatedBy:        actorID,
	}

	if err := s.repo.Create(settlement); err != nil {
		return nil, common.NewInternalError("failed to create settlement")
	}

	s.audit.Log(actorID, common.AuditActionCreate, "Payment", settlement.ID,
		fmt.Sprintf("Settlement posting created: $%.2f for payment %d", req.Amount, req.PaymentID), ip, requestID)

	return settlement, nil
}

// dualApprovalThreshold returns the configured dual-approval amount threshold.
func (s *Service) dualApprovalThreshold() float64 {
	if s.cfg != nil && s.cfg.Payment.DualApprovalThreshold > 0 {
		return s.cfg.Payment.DualApprovalThreshold
	}
	return 500.00
}

// CreateReversal creates a Reversal payment and marks the original as reversed.
// For amounts above the dual-approval threshold the original payment must have
// received two approvals from distinct approvers before reversal is permitted.
func (s *Service) CreateReversal(paymentID uint64, req CreateReversalRequest, actorID uint64, ip, requestID string) (*Payment, *common.AppError) {
	if fe := common.ValidateStringLength("reason", req.Reason, 10, 2000); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	original, appErr := s.GetPayment(paymentID)
	if appErr != nil {
		return nil, appErr
	}

	if original.Status == common.PaymentStatusReversed {
		return nil, common.NewConflictError("payment has already been reversed")
	}
	if original.Status == common.PaymentStatusExpired {
		return nil, common.NewValidationError("cannot reverse an expired payment")
	}

	// Enforce dual-approval gate for high-value payments.
	if original.Amount > s.dualApprovalThreshold() {
		approvals, err := s.repo.FindApprovalsByPayment(original.ID)
		if err != nil {
			return nil, common.NewInternalError("failed to verify payment approvals")
		}
		if len(approvals) < 2 {
			return nil, common.NewValidationError("payment exceeds approval threshold; dual approval required before reversal")
		}
	}

	now := time.Now().UTC()

	// Create reversal payment.
	reversal := &Payment{
		UUID:             uuid.New().String(),
		WorkOrderID:      original.WorkOrderID,
		TenantID:         original.TenantID,
		UnitID:           original.UnitID,
		PropertyID:       original.PropertyID,
		Kind:             common.PaymentKindReversal,
		Amount:           original.Amount,
		Currency:         "USD",
		Status:           common.PaymentStatusReversed,
		Description:      &req.Reason,
		RelatedPaymentID: &original.ID,
		CreatedBy:        actorID,
	}

	if err := s.repo.Create(reversal); err != nil {
		return nil, common.NewInternalError("failed to create reversal")
	}

	// Mark original as reversed.
	original.Status = common.PaymentStatusReversed
	original.ReversedAt = &now
	original.ReversalReason = &req.Reason

	if err := s.repo.Update(original); err != nil {
		return nil, common.NewInternalError("failed to update original payment")
	}

	s.audit.Log(actorID, common.AuditActionStatusChange, "Payment", original.ID,
		fmt.Sprintf("Payment reversed: %s", req.Reason), ip, requestID)

	return reversal, nil
}

// CreateMakeup creates a MakeupPosting linked to the original payment.
// For makeup amounts above the dual-approval threshold the original payment must
// already have dual approval from two distinct approvers.
func (s *Service) CreateMakeup(paymentID uint64, req CreateMakeupRequest, actorID uint64, ip, requestID string) (*Payment, *common.AppError) {
	if fe := common.ValidateMoneyAmount("amount", req.Amount); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	original, appErr := s.GetPayment(paymentID)
	if appErr != nil {
		return nil, appErr
	}

	// Enforce dual-approval gate for high-value makeup postings.
	if req.Amount > s.dualApprovalThreshold() {
		approvals, err := s.repo.FindApprovalsByPayment(original.ID)
		if err != nil {
			return nil, common.NewInternalError("failed to verify payment approvals")
		}
		if len(approvals) < 2 {
			return nil, common.NewValidationError("makeup amount exceeds approval threshold; dual approval required on the original payment")
		}
	}

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	makeup := &Payment{
		UUID:             uuid.New().String(),
		WorkOrderID:      original.WorkOrderID,
		TenantID:         original.TenantID,
		UnitID:           original.UnitID,
		PropertyID:       original.PropertyID,
		Kind:             common.PaymentKindMakeupPosting,
		Amount:           req.Amount,
		Currency:         "USD",
		Status:           common.PaymentStatusPaid,
		Description:      desc,
		RelatedPaymentID: &original.ID,
		CreatedBy:        actorID,
	}

	if err := s.repo.Create(makeup); err != nil {
		return nil, common.NewInternalError("failed to create makeup posting")
	}

	s.audit.Log(actorID, common.AuditActionCreate, "Payment", makeup.ID,
		fmt.Sprintf("Makeup posting created: $%.2f for payment %d", req.Amount, paymentID), ip, requestID)

	return makeup, nil
}

// ApprovePayment implements dual-approval for amounts > threshold.
// - First approver creates PaymentApproval with order=1
// - Second different approver creates PaymentApproval with order=2
// - Same user cannot approve twice
// - Amounts <= threshold need single approval
func (s *Service) ApprovePayment(paymentID uint64, req ApprovePaymentRequest, approverID uint64, ip, requestID string) (*PaymentApproval, *common.AppError) {
	payment, appErr := s.GetPayment(paymentID)
	if appErr != nil {
		return nil, appErr
	}

	if payment.Status != common.PaymentStatusPaid {
		return nil, common.NewValidationError("only Paid payments can be approved")
	}

	// Get existing approvals.
	existingApprovals, err := s.repo.FindApprovalsByPayment(paymentID)
	if err != nil {
		return nil, common.NewInternalError("failed to check existing approvals")
	}

	// Check if this user already approved.
	for _, a := range existingApprovals {
		if a.ApproverID == approverID {
			return nil, common.NewConflictError("you have already approved this payment")
		}
	}

	threshold := 500.00
	if s.cfg != nil && s.cfg.Payment.DualApprovalThreshold > 0 {
		threshold = s.cfg.Payment.DualApprovalThreshold
	}

	needsDualApproval := payment.Amount > threshold
	approvalCount := len(existingApprovals)

	if needsDualApproval && approvalCount >= 2 {
		return nil, common.NewConflictError("payment already has required approvals")
	}
	if !needsDualApproval && approvalCount >= 1 {
		return nil, common.NewConflictError("payment already has required approval")
	}

	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}

	approval := &PaymentApproval{
		PaymentID:     paymentID,
		ApproverID:    approverID,
		ApprovalOrder: uint8(approvalCount + 1),
		Notes:         notes,
		ApprovedAt:    time.Now().UTC(),
	}

	if err := s.repo.CreateApproval(approval); err != nil {
		return nil, common.NewInternalError("failed to create payment approval")
	}

	s.audit.Log(approverID, common.AuditActionApproval, "Payment", paymentID,
		fmt.Sprintf("Payment approved (approval #%d)", approvalCount+1), ip, requestID)

	// Check if approval requirements are now met.
	requiredApprovals := 1
	if needsDualApproval {
		requiredApprovals = 2
	}

	if approvalCount+1 >= requiredApprovals {
		// All approvals obtained — mark as settled.
		payment.Status = common.PaymentStatusSettled
		if err := s.repo.Update(payment); err != nil {
			return nil, common.NewInternalError("failed to update payment status after approval")
		}

		s.audit.Log(approverID, common.AuditActionStatusChange, "Payment", paymentID,
			"Payment fully approved and settled", ip, requestID)
	}

	return approval, nil
}

// ExpireStaleIntents finds all pending intents past their expiration time and marks them as expired.
func (s *Service) ExpireStaleIntents() (int, *common.AppError) {
	expired, err := s.repo.FindExpiredIntents()
	if err != nil {
		return 0, common.NewInternalError("failed to find expired intents")
	}

	count := 0
	for i := range expired {
		expired[i].Status = common.PaymentStatusExpired
		if err := s.repo.Update(&expired[i]); err == nil {
			count++
		}
	}

	return count, nil
}

// RunReconciliation triggers a daily reconciliation run.
func (s *Service) RunReconciliation(runDate time.Time, generatedBy uint64, ip, requestID string) (*ReconciliationRun, *common.AppError) {
	run, err := s.recon.RunDaily(runDate, generatedBy)
	if err != nil {
		return nil, common.NewInternalError(fmt.Sprintf("reconciliation failed: %v", err))
	}

	s.audit.Log(generatedBy, common.AuditActionCreate, "ReconciliationRun", run.ID,
		fmt.Sprintf("Reconciliation run for %s", runDate.Format("2006-01-02")), ip, requestID)

	return run, nil
}

// GetReconciliation retrieves a reconciliation run by its ID.
func (s *Service) GetReconciliation(id uint64) (*ReconciliationRun, *common.AppError) {
	run, err := s.repo.FindReconciliation(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, common.NewNotFoundError("Reconciliation run")
		}
		return nil, common.NewInternalError("failed to retrieve reconciliation run")
	}
	return run, nil
}

// ListReconciliations retrieves reconciliation runs with pagination.
func (s *Service) ListReconciliations(page, perPage int) ([]ReconciliationRun, int64, *common.AppError) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	offset := (page - 1) * perPage
	runs, total, err := s.repo.ListReconciliations(offset, perPage)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list reconciliation runs")
	}

	return runs, total, nil
}
