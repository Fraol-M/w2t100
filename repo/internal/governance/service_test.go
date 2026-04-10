package governance

import (
	"testing"
	"time"

	"propertyops/backend/internal/common"

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

func (m *mockNotifier) SendEventToRole(eventName, role string, data map[string]string) error {
	// Record as a synthetic event with recipientID=0 (role-targeted, no single recipient).
	m.events = append(m.events, notifyEvent{
		eventName: eventName,
		data:      data,
	})
	return nil
}

func setupServiceDB(t *testing.T) (*gorm.DB, *Repository, *mockAuditLogger, *mockNotifier) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("SQLite unavailable (CGO disabled or no C compiler): %v", err)
	}
	if err := db.AutoMigrate(&Report{}, &EnforcementAction{}, &KeywordBlacklist{}, &RiskRule{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	repo := NewRepository(db)
	audit := &mockAuditLogger{}
	notifier := &mockNotifier{}
	return db, repo, audit, notifier
}

// --- Report creation tests ---

func TestCreateReport_Success(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	req := CreateReportRequest{
		TargetType:  common.ReportTargetTenant,
		TargetID:    42,
		Category:    "Harassment",
		Description: "This tenant has been harassing other residents repeatedly over the past week.",
	}

	report, appErr := svc.CreateReport(req, 1, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if report.Status != common.ReportStatusOpen {
		t.Errorf("expected status %s, got %s", common.ReportStatusOpen, report.Status)
	}
	if report.ReporterID != 1 {
		t.Errorf("expected reporter_id 1, got %d", report.ReporterID)
	}
	if report.TargetType != common.ReportTargetTenant {
		t.Errorf("expected target_type %s, got %s", common.ReportTargetTenant, report.TargetType)
	}
	if report.UUID == "" {
		t.Error("expected non-empty UUID")
	}

	// Check audit log was called.
	if len(audit.calls) != 1 {
		t.Errorf("expected 1 audit call, got %d", len(audit.calls))
	}
	if audit.calls[0].action != common.AuditActionCreate {
		t.Errorf("expected audit action %s, got %s", common.AuditActionCreate, audit.calls[0].action)
	}

	// Check notification was sent.
	if len(notifier.events) != 1 {
		t.Errorf("expected 1 notification, got %d", len(notifier.events))
	}
}

func TestCreateReport_InvalidTargetType(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	req := CreateReportRequest{
		TargetType:  "InvalidType",
		TargetID:    42,
		Category:    "Harassment",
		Description: "This tenant has been harassing other residents repeatedly over the past week.",
	}

	_, appErr := svc.CreateReport(req, 1, "127.0.0.1", "req-1")
	if appErr == nil {
		t.Fatal("expected validation error for invalid target_type")
	}
	if appErr.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", appErr.Code)
	}
}

func TestCreateReport_DescriptionTooShort(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	req := CreateReportRequest{
		TargetType:  common.ReportTargetWorkOrder,
		TargetID:    1,
		Category:    "Quality",
		Description: "Too short",
	}

	_, appErr := svc.CreateReport(req, 1, "127.0.0.1", "req-1")
	if appErr == nil {
		t.Fatal("expected validation error for short description")
	}
}

// --- Report status transition tests ---

func TestReviewReport_OpenToInReview(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	// Create a report first.
	req := CreateReportRequest{
		TargetType:  common.ReportTargetTenant,
		TargetID:    1,
		Category:    "Harassment",
		Description: "This tenant has been causing issues for all residents in the building.",
	}
	report, _ := svc.CreateReport(req, 1, "127.0.0.1", "req-1")

	// Transition to InReview.
	reviewed, appErr := svc.ReviewReport(report.ID, ReviewReportRequest{
		Status: common.ReportStatusInReview,
	}, 10, "127.0.0.1", "req-2")

	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if reviewed.Status != common.ReportStatusInReview {
		t.Errorf("expected status %s, got %s", common.ReportStatusInReview, reviewed.Status)
	}
	if reviewed.ReviewerID == nil || *reviewed.ReviewerID != 10 {
		t.Error("expected reviewer_id to be set to 10")
	}
}

func TestReviewReport_InReviewToResolved(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	req := CreateReportRequest{
		TargetType:  common.ReportTargetTenant,
		TargetID:    1,
		Category:    "Harassment",
		Description: "This tenant has been causing issues for all residents in the building.",
	}
	report, _ := svc.CreateReport(req, 1, "127.0.0.1", "req-1")

	// Open -> InReview.
	svc.ReviewReport(report.ID, ReviewReportRequest{Status: common.ReportStatusInReview}, 10, "", "")

	// InReview -> Resolved.
	resolved, appErr := svc.ReviewReport(report.ID, ReviewReportRequest{
		Status:          common.ReportStatusResolved,
		ResolutionNotes: "Issue addressed with tenant directly.",
	}, 10, "127.0.0.1", "req-3")

	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if resolved.Status != common.ReportStatusResolved {
		t.Errorf("expected status %s, got %s", common.ReportStatusResolved, resolved.Status)
	}
	if resolved.ResolvedAt == nil {
		t.Error("expected resolved_at to be set")
	}
	if resolved.ResolutionNotes == nil || *resolved.ResolutionNotes != "Issue addressed with tenant directly." {
		t.Error("expected resolution_notes to be set")
	}
}

func TestReviewReport_InvalidTransition_ResolvedToInReview(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	req := CreateReportRequest{
		TargetType:  common.ReportTargetThread,
		TargetID:    1,
		Category:    "Spam",
		Description: "This thread contains spam messages that need to be reviewed immediately.",
	}
	report, _ := svc.CreateReport(req, 1, "", "")

	// Open -> InReview.
	svc.ReviewReport(report.ID, ReviewReportRequest{Status: common.ReportStatusInReview}, 10, "", "")
	// InReview -> Resolved.
	svc.ReviewReport(report.ID, ReviewReportRequest{Status: common.ReportStatusResolved}, 10, "", "")

	// Resolved -> InReview should fail.
	_, appErr := svc.ReviewReport(report.ID, ReviewReportRequest{Status: common.ReportStatusInReview}, 10, "", "")
	if appErr == nil {
		t.Fatal("expected error for invalid transition from Resolved to InReview")
	}
}

// --- Enforcement tests ---

func TestApplyEnforcement_Warning(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	req := CreateEnforcementRequest{
		UserID:     5,
		ActionType: common.EnforcementWarning,
		Reason:     "Repeated violations of community guidelines observed.",
	}

	action, appErr := svc.ApplyEnforcement(req, 10, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if action.ActionType != common.EnforcementWarning {
		t.Errorf("expected action_type %s, got %s", common.EnforcementWarning, action.ActionType)
	}
	if action.UserID != 5 {
		t.Errorf("expected user_id 5, got %d", action.UserID)
	}
	if !action.IsActive {
		t.Error("expected enforcement to be active")
	}
	if action.EndsAt != nil {
		t.Error("expected nil ends_at for warning")
	}
}

func TestApplyEnforcement_RateLimit(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	max := 5
	window := 60
	req := CreateEnforcementRequest{
		UserID:                 5,
		ActionType:             common.EnforcementRateLimit,
		Reason:                 "Excessive submissions detected from this user account.",
		RateLimitMax:           &max,
		RateLimitWindowMinutes: &window,
	}

	action, appErr := svc.ApplyEnforcement(req, 10, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if action.ActionType != common.EnforcementRateLimit {
		t.Errorf("expected action_type %s, got %s", common.EnforcementRateLimit, action.ActionType)
	}
	if action.RateLimitMax == nil || *action.RateLimitMax != 5 {
		t.Error("expected rate_limit_max to be 5")
	}
	if action.RateLimitWindowMinutes == nil || *action.RateLimitWindowMinutes != 60 {
		t.Error("expected rate_limit_window_minutes to be 60")
	}
}

func TestApplyEnforcement_RateLimit_MissingFields(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	req := CreateEnforcementRequest{
		UserID:     5,
		ActionType: common.EnforcementRateLimit,
		Reason:     "Excessive submissions detected from this user account.",
	}

	_, appErr := svc.ApplyEnforcement(req, 10, "127.0.0.1", "req-1")
	if appErr == nil {
		t.Fatal("expected validation error for missing rate limit fields")
	}
}

func TestApplyEnforcement_Suspension_1Day(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	req := CreateEnforcementRequest{
		UserID:     5,
		ActionType: common.EnforcementSuspension,
		Reason:     "Serious violation of terms of service warranting a temporary suspension.",
		EndsAt:     "1day",
	}

	action, appErr := svc.ApplyEnforcement(req, 10, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if action.EndsAt == nil {
		t.Fatal("expected ends_at to be set for 1day suspension")
	}

	// Should be approximately 24 hours from now.
	diff := action.EndsAt.Sub(action.StartsAt)
	if diff < 23*time.Hour || diff > 25*time.Hour {
		t.Errorf("expected ends_at ~24h from starts_at, got %v", diff)
	}
}

func TestApplyEnforcement_Suspension_7Day(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	req := CreateEnforcementRequest{
		UserID:     5,
		ActionType: common.EnforcementSuspension,
		Reason:     "Multiple violations of community standards requiring extended suspension.",
		EndsAt:     "7day",
	}

	action, appErr := svc.ApplyEnforcement(req, 10, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if action.EndsAt == nil {
		t.Fatal("expected ends_at to be set for 7day suspension")
	}

	diff := action.EndsAt.Sub(action.StartsAt)
	if diff < 6*24*time.Hour || diff > 8*24*time.Hour {
		t.Errorf("expected ends_at ~7 days from starts_at, got %v", diff)
	}
}

func TestApplyEnforcement_Suspension_Indefinite(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	req := CreateEnforcementRequest{
		UserID:     5,
		ActionType: common.EnforcementSuspension,
		Reason:     "Severe and repeated violations requiring indefinite suspension from the platform.",
		EndsAt:     "indefinite",
	}

	action, appErr := svc.ApplyEnforcement(req, 10, "127.0.0.1", "req-1")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if action.EndsAt != nil {
		t.Error("expected nil ends_at for indefinite suspension")
	}
}

func TestApplyEnforcement_InvalidActionType(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	req := CreateEnforcementRequest{
		UserID:     5,
		ActionType: "Ban",
		Reason:     "Some reason that is at least ten characters long.",
	}

	_, appErr := svc.ApplyEnforcement(req, 10, "127.0.0.1", "req-1")
	if appErr == nil {
		t.Fatal("expected validation error for invalid action type")
	}
}

// --- Revocation tests ---

func TestRevokeEnforcement_Success(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	// Create enforcement first.
	createReq := CreateEnforcementRequest{
		UserID:     5,
		ActionType: common.EnforcementWarning,
		Reason:     "Repeated violations of community guidelines observed.",
	}
	action, _ := svc.ApplyEnforcement(createReq, 10, "", "")

	// Revoke it.
	revokeReq := RevokeEnforcementRequest{
		Reason: "Enforcement was applied in error and should be revoked.",
	}
	revoked, appErr := svc.RevokeEnforcement(action.ID, revokeReq, 10, "127.0.0.1", "req-2")

	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if revoked.IsActive {
		t.Error("expected enforcement to be inactive after revocation")
	}
	if revoked.RevokedAt == nil {
		t.Error("expected revoked_at to be set")
	}
	if revoked.RevokedBy == nil || *revoked.RevokedBy != 10 {
		t.Error("expected revoked_by to be 10")
	}
}

func TestRevokeEnforcement_AlreadyRevoked(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	createReq := CreateEnforcementRequest{
		UserID:     5,
		ActionType: common.EnforcementWarning,
		Reason:     "Repeated violations of community guidelines observed.",
	}
	action, _ := svc.ApplyEnforcement(createReq, 10, "", "")

	revokeReq := RevokeEnforcementRequest{
		Reason: "Enforcement was applied in error and should be revoked.",
	}
	svc.RevokeEnforcement(action.ID, revokeReq, 10, "", "")

	// Try to revoke again.
	_, appErr := svc.RevokeEnforcement(action.ID, revokeReq, 10, "", "")
	if appErr == nil {
		t.Fatal("expected conflict error when revoking already-revoked enforcement")
	}
	if appErr.Code != "CONFLICT" {
		t.Errorf("expected CONFLICT error, got %s", appErr.Code)
	}
}

// --- Blacklist checking tests ---

func TestCheckBlacklist_MatchesKeywords(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	// Seed keywords.
	svc.CreateKeyword(KeywordRequest{Keyword: "profanity", Category: "Language", Severity: "Medium"}, 1, "", "")
	svc.CreateKeyword(KeywordRequest{Keyword: "slur", Category: "Hate", Severity: "High"}, 1, "", "")

	matched, err := svc.CheckBlacklist("This message contains profanity and a slur")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matched) != 2 {
		t.Errorf("expected 2 matched keywords, got %d", len(matched))
	}
}

func TestCheckBlacklist_NoMatch(t *testing.T) {
	db, repo, audit, notifier := setupServiceDB(t)
	svc := NewService(repo, audit, notifier, db)

	svc.CreateKeyword(KeywordRequest{Keyword: "profanity", Category: "Language", Severity: "Medium"}, 1, "", "")

	matched, err := svc.CheckBlacklist("A perfectly clean message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matched) != 0 {
		t.Errorf("expected 0 matched keywords, got %d", len(matched))
	}
}
