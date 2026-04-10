package payments

import (
	"testing"
	"time"

	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// mockAuditLogger implements AuditLogger for testing.
type mockAuditLogger struct {
	calls []auditCall
}

type auditCall struct {
	actorID      uint64
	action       string
	resourceType string
	resourceID   uint64
	description  string
}

func (m *mockAuditLogger) Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string) {
	m.calls = append(m.calls, auditCall{
		actorID:      actorID,
		action:       action,
		resourceType: resourceType,
		resourceID:   resourceID,
		description:  description,
	})
}

// mockNotifier implements NotificationSender for testing.
type mockNotifier struct {
	events []notifyEvent
}

type notifyEvent struct {
	eventName   string
	recipientID uint64
	data        map[string]string
}

func (m *mockNotifier) SendEvent(eventName string, recipientID uint64, data map[string]string) error {
	m.events = append(m.events, notifyEvent{
		eventName:   eventName,
		recipientID: recipientID,
		data:        data,
	})
	return nil
}

func setupServiceDB(t *testing.T) (*gorm.DB, *Repository, *mockAuditLogger, *mockNotifier, *config.Config) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("SQLite unavailable (CGO disabled or no C compiler): %v", err)
	}
	if err := db.AutoMigrate(&Payment{}, &PaymentApproval{}, &ReconciliationRun{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	repo := NewRepository(db)
	audit := &mockAuditLogger{}
	notifier := &mockNotifier{}
	cfg := &config.Config{
		Storage: config.StorageConfig{Root: t.TempDir()},
		Payment: config.PaymentConfig{
			IntentExpiryMinutes:   30,
			DualApprovalThreshold: 500.00,
		},
	}
	return db, repo, audit, notifier, cfg
}

// --- Intent creation tests ---

func TestCreateIntent_Success(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	req := CreateIntentRequest{
		PropertyID: 1,
		Amount:     250.00,
	}

	payment, appErr := svc.CreateIntent(req, 10, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if payment.Kind != common.PaymentKindIntent {
		t.Errorf("expected kind %s, got %s", common.PaymentKindIntent, payment.Kind)
	}
	if payment.Status != common.PaymentStatusPending {
		t.Errorf("expected status %s, got %s", common.PaymentStatusPending, payment.Status)
	}
	if payment.Amount != 250.00 {
		t.Errorf("expected amount 250.00, got %.2f", payment.Amount)
	}
	if payment.UUID == "" {
		t.Error("expected non-empty UUID")
	}
}

func TestCreateIntent_WithExpiry(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	before := time.Now().UTC()
	req := CreateIntentRequest{
		PropertyID: 1,
		Amount:     100.00,
	}

	payment, appErr := svc.CreateIntent(req, 10, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if payment.ExpiresAt == nil {
		t.Fatal("expected expires_at to be set")
	}

	// Should expire 30 minutes from now (with some tolerance).
	expectedExpiry := before.Add(30 * time.Minute)
	diff := payment.ExpiresAt.Sub(expectedExpiry)
	if diff < -2*time.Second || diff > 2*time.Second {
		t.Errorf("expected expires_at ~30 min from now, got diff %v", diff)
	}
}

func TestCreateIntent_InvalidAmount(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	req := CreateIntentRequest{
		PropertyID: 1,
		Amount:     -50.00,
	}

	_, appErr := svc.CreateIntent(req, 10, "127.0.0.1", "req-1")
	if appErr == nil {
		t.Fatal("expected validation error for negative amount")
	}
	if appErr.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", appErr.Code)
	}
}

// --- Intent expiration tests ---

func TestExpireStaleIntents(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	// Create an intent that is already expired.
	pastExpiry := time.Now().UTC().Add(-1 * time.Hour)
	db.Create(&Payment{
		UUID:       "expired-intent",
		PropertyID: 1,
		Kind:       common.PaymentKindIntent,
		Amount:     100.00,
		Currency:   "USD",
		Status:     common.PaymentStatusPending,
		ExpiresAt:  &pastExpiry,
		CreatedBy:  10,
	})

	// Create a non-expired intent.
	futureExpiry := time.Now().UTC().Add(1 * time.Hour)
	db.Create(&Payment{
		UUID:       "active-intent",
		PropertyID: 1,
		Kind:       common.PaymentKindIntent,
		Amount:     200.00,
		Currency:   "USD",
		Status:     common.PaymentStatusPending,
		ExpiresAt:  &futureExpiry,
		CreatedBy:  10,
	})

	count, appErr := svc.ExpireStaleIntents()
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if count != 1 {
		t.Errorf("expected 1 expired intent, got %d", count)
	}

	// Verify the expired one has Expired status.
	var expired Payment
	db.Where("uuid = ?", "expired-intent").First(&expired)
	if expired.Status != common.PaymentStatusExpired {
		t.Errorf("expected status %s, got %s", common.PaymentStatusExpired, expired.Status)
	}

	// Verify the active one is still Pending.
	var active Payment
	db.Where("uuid = ?", "active-intent").First(&active)
	if active.Status != common.PaymentStatusPending {
		t.Errorf("expected status %s, got %s", common.PaymentStatusPending, active.Status)
	}
}

// --- Mark paid tests ---

func TestMarkPaid_Success(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	// Create an intent.
	req := CreateIntentRequest{PropertyID: 1, Amount: 100.00}
	payment, _ := svc.CreateIntent(req, 10, "", "")

	// Mark it as paid.
	paid, appErr := svc.MarkPaid(payment.ID, MarkPaidRequest{}, 10, "127.0.0.1", "req-2")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if paid.Status != common.PaymentStatusPaid {
		t.Errorf("expected status %s, got %s", common.PaymentStatusPaid, paid.Status)
	}
	if paid.PaidAt == nil {
		t.Error("expected paid_at to be set")
	}
	if paid.PaidBy == nil || *paid.PaidBy != 10 {
		t.Error("expected paid_by to be 10")
	}
}

func TestMarkPaid_ExpiredIntent(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	pastExpiry := time.Now().UTC().Add(-1 * time.Hour)
	db.Create(&Payment{
		UUID:       "expired-for-pay",
		PropertyID: 1,
		Kind:       common.PaymentKindIntent,
		Amount:     100.00,
		Currency:   "USD",
		Status:     common.PaymentStatusPending,
		ExpiresAt:  &pastExpiry,
		CreatedBy:  10,
	})

	var p Payment
	db.Where("uuid = ?", "expired-for-pay").First(&p)

	_, appErr := svc.MarkPaid(p.ID, MarkPaidRequest{}, 10, "", "")
	if appErr == nil {
		t.Fatal("expected error for expired intent")
	}
}

func TestMarkPaid_OnlyIntents(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	// Create a settlement (not an intent).
	db.Create(&Payment{
		UUID:       "not-an-intent",
		PropertyID: 1,
		Kind:       common.PaymentKindSettlementPosting,
		Amount:     100.00,
		Currency:   "USD",
		Status:     common.PaymentStatusPending,
		CreatedBy:  10,
	})

	var p Payment
	db.Where("uuid = ?", "not-an-intent").First(&p)

	_, appErr := svc.MarkPaid(p.ID, MarkPaidRequest{}, 10, "", "")
	if appErr == nil {
		t.Fatal("expected error for non-intent payment")
	}
}

// --- Dual approval tests ---

func TestApprovePayment_SingleApproval_Under500(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	// Create and pay an intent for $300.
	req := CreateIntentRequest{PropertyID: 1, Amount: 300.00}
	payment, _ := svc.CreateIntent(req, 10, "", "")
	svc.MarkPaid(payment.ID, MarkPaidRequest{}, 10, "", "")

	// Single approval should be sufficient.
	approval, appErr := svc.ApprovePayment(payment.ID, ApprovePaymentRequest{Notes: "Looks good"}, 20, "127.0.0.1", "req-3")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if approval.ApprovalOrder != 1 {
		t.Errorf("expected approval_order 1, got %d", approval.ApprovalOrder)
	}

	// Payment should be settled after single approval.
	p, _ := svc.GetPayment(payment.ID)
	if p.Status != common.PaymentStatusSettled {
		t.Errorf("expected status %s after single approval for $300, got %s", common.PaymentStatusSettled, p.Status)
	}
}

func TestApprovePayment_DualApproval_Over500(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	// Create and pay an intent for $1000.
	req := CreateIntentRequest{PropertyID: 1, Amount: 1000.00}
	payment, _ := svc.CreateIntent(req, 10, "", "")
	svc.MarkPaid(payment.ID, MarkPaidRequest{}, 10, "", "")

	// First approval.
	approval1, appErr := svc.ApprovePayment(payment.ID, ApprovePaymentRequest{}, 20, "", "")
	if appErr != nil {
		t.Fatalf("unexpected error on first approval: %v", appErr)
	}
	if approval1.ApprovalOrder != 1 {
		t.Errorf("expected approval_order 1, got %d", approval1.ApprovalOrder)
	}

	// Payment should NOT be settled yet.
	p, _ := svc.GetPayment(payment.ID)
	if p.Status != common.PaymentStatusPaid {
		t.Errorf("expected status %s after first approval for $1000, got %s", common.PaymentStatusPaid, p.Status)
	}

	// Second approval from a different user.
	approval2, appErr := svc.ApprovePayment(payment.ID, ApprovePaymentRequest{}, 30, "", "")
	if appErr != nil {
		t.Fatalf("unexpected error on second approval: %v", appErr)
	}
	if approval2.ApprovalOrder != 2 {
		t.Errorf("expected approval_order 2, got %d", approval2.ApprovalOrder)
	}

	// Payment should now be settled.
	p, _ = svc.GetPayment(payment.ID)
	if p.Status != common.PaymentStatusSettled {
		t.Errorf("expected status %s after dual approval for $1000, got %s", common.PaymentStatusSettled, p.Status)
	}
}

func TestApprovePayment_SameApproverBlocked(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	// Create and pay an intent for $1000.
	req := CreateIntentRequest{PropertyID: 1, Amount: 1000.00}
	payment, _ := svc.CreateIntent(req, 10, "", "")
	svc.MarkPaid(payment.ID, MarkPaidRequest{}, 10, "", "")

	// First approval.
	svc.ApprovePayment(payment.ID, ApprovePaymentRequest{}, 20, "", "")

	// Same user tries to approve again.
	_, appErr := svc.ApprovePayment(payment.ID, ApprovePaymentRequest{}, 20, "", "")
	if appErr == nil {
		t.Fatal("expected error when same user tries to approve twice")
	}
	if appErr.Code != "CONFLICT" {
		t.Errorf("expected CONFLICT error, got %s", appErr.Code)
	}
}

func TestApprovePayment_OnlyPaidCanBeApproved(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	// Create an intent (still pending, not paid).
	req := CreateIntentRequest{PropertyID: 1, Amount: 100.00}
	payment, _ := svc.CreateIntent(req, 10, "", "")

	_, appErr := svc.ApprovePayment(payment.ID, ApprovePaymentRequest{}, 20, "", "")
	if appErr == nil {
		t.Fatal("expected error for approving non-paid payment")
	}
}

// --- Reversal tests ---

func TestCreateReversal_Success(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	// Create and pay an intent.
	req := CreateIntentRequest{PropertyID: 1, Amount: 150.00}
	payment, _ := svc.CreateIntent(req, 10, "", "")
	svc.MarkPaid(payment.ID, MarkPaidRequest{}, 10, "", "")

	// Reverse it.
	reversal, appErr := svc.CreateReversal(payment.ID, CreateReversalRequest{
		Reason: "Payment was made in error and needs to be reversed immediately.",
	}, 10, "127.0.0.1", "req-4")

	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if reversal.Kind != common.PaymentKindReversal {
		t.Errorf("expected kind %s, got %s", common.PaymentKindReversal, reversal.Kind)
	}
	if reversal.Amount != 150.00 {
		t.Errorf("expected amount 150.00, got %.2f", reversal.Amount)
	}
	if reversal.RelatedPaymentID == nil || *reversal.RelatedPaymentID != payment.ID {
		t.Error("expected related_payment_id to reference original payment")
	}

	// Original should be marked as reversed.
	original, _ := svc.GetPayment(payment.ID)
	if original.Status != common.PaymentStatusReversed {
		t.Errorf("expected original status %s, got %s", common.PaymentStatusReversed, original.Status)
	}
	if original.ReversedAt == nil {
		t.Error("expected reversed_at to be set on original")
	}
	if original.ReversalReason == nil {
		t.Error("expected reversal_reason to be set on original")
	}
}

func TestCreateReversal_RequiresReason(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	req := CreateIntentRequest{PropertyID: 1, Amount: 100.00}
	payment, _ := svc.CreateIntent(req, 10, "", "")
	svc.MarkPaid(payment.ID, MarkPaidRequest{}, 10, "", "")

	_, appErr := svc.CreateReversal(payment.ID, CreateReversalRequest{Reason: "short"}, 10, "", "")
	if appErr == nil {
		t.Fatal("expected validation error for short reason")
	}
}

func TestCreateReversal_CannotReverseAlreadyReversed(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	req := CreateIntentRequest{PropertyID: 1, Amount: 100.00}
	payment, _ := svc.CreateIntent(req, 10, "", "")
	svc.MarkPaid(payment.ID, MarkPaidRequest{}, 10, "", "")

	svc.CreateReversal(payment.ID, CreateReversalRequest{
		Reason: "Payment was made in error and needs to be reversed immediately.",
	}, 10, "", "")

	// Try to reverse again.
	_, appErr := svc.CreateReversal(payment.ID, CreateReversalRequest{
		Reason: "Another attempt to reverse an already reversed payment.",
	}, 10, "", "")
	if appErr == nil {
		t.Fatal("expected conflict error for double reversal")
	}
	if appErr.Code != "CONFLICT" {
		t.Errorf("expected CONFLICT error, got %s", appErr.Code)
	}
}

// --- Makeup posting tests ---

func TestCreateMakeup_Success(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	req := CreateIntentRequest{PropertyID: 1, Amount: 100.00}
	payment, _ := svc.CreateIntent(req, 10, "", "")
	svc.MarkPaid(payment.ID, MarkPaidRequest{}, 10, "", "")

	makeup, appErr := svc.CreateMakeup(payment.ID, CreateMakeupRequest{
		Amount:      25.00,
		Description: "Additional charge for materials",
	}, 10, "127.0.0.1", "req-5")

	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if makeup.Kind != common.PaymentKindMakeupPosting {
		t.Errorf("expected kind %s, got %s", common.PaymentKindMakeupPosting, makeup.Kind)
	}
	if makeup.Amount != 25.00 {
		t.Errorf("expected amount 25.00, got %.2f", makeup.Amount)
	}
	if makeup.RelatedPaymentID == nil || *makeup.RelatedPaymentID != payment.ID {
		t.Error("expected related_payment_id to reference original payment")
	}
}

func TestCreateMakeup_InvalidAmount(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	req := CreateIntentRequest{PropertyID: 1, Amount: 100.00}
	payment, _ := svc.CreateIntent(req, 10, "", "")
	svc.MarkPaid(payment.ID, MarkPaidRequest{}, 10, "", "")

	_, appErr := svc.CreateMakeup(payment.ID, CreateMakeupRequest{Amount: -10.00}, 10, "", "")
	if appErr == nil {
		t.Fatal("expected validation error for negative makeup amount")
	}
}

// --- Approval at threshold boundary tests ---

func TestApprovePayment_ExactlyAt500_SingleApproval(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	// Create and pay an intent for exactly $500.
	req := CreateIntentRequest{PropertyID: 1, Amount: 500.00}
	payment, _ := svc.CreateIntent(req, 10, "", "")
	svc.MarkPaid(payment.ID, MarkPaidRequest{}, 10, "", "")

	// $500 is not > $500, so single approval should suffice.
	svc.ApprovePayment(payment.ID, ApprovePaymentRequest{}, 20, "", "")

	p, _ := svc.GetPayment(payment.ID)
	if p.Status != common.PaymentStatusSettled {
		t.Errorf("expected status %s for exactly $500, got %s", common.PaymentStatusSettled, p.Status)
	}
}

func TestApprovePayment_JustOver500_NeedsDual(t *testing.T) {
	db, repo, audit, notifier, cfg := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db, cfg)

	// Create and pay an intent for $500.01.
	req := CreateIntentRequest{PropertyID: 1, Amount: 500.01}
	payment, _ := svc.CreateIntent(req, 10, "", "")
	svc.MarkPaid(payment.ID, MarkPaidRequest{}, 10, "", "")

	// First approval should NOT settle.
	svc.ApprovePayment(payment.ID, ApprovePaymentRequest{}, 20, "", "")

	p, _ := svc.GetPayment(payment.ID)
	if p.Status != common.PaymentStatusPaid {
		t.Errorf("expected status %s after first approval for $500.01, got %s", common.PaymentStatusPaid, p.Status)
	}

	// Second approval from different user should settle.
	svc.ApprovePayment(payment.ID, ApprovePaymentRequest{}, 30, "", "")

	p, _ = svc.GetPayment(payment.ID)
	if p.Status != common.PaymentStatusSettled {
		t.Errorf("expected status %s after dual approval for $500.01, got %s", common.PaymentStatusSettled, p.Status)
	}
}
