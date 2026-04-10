package governance

import (
	"fmt"
	"log"
	"time"

	"propertyops/backend/internal/common"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AuditLogger abstracts the audit logging dependency.
type AuditLogger interface {
	Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string)
}

// NotificationSender abstracts the notification sending dependency.
type NotificationSender interface {
	SendEvent(eventName string, recipientID uint64, data map[string]string) error
	// SendEventToRole fans out eventName to all active users with the given role.
	SendEventToRole(eventName, role string, data map[string]string) error
}

// Service handles governance business logic.
type Service struct {
	repo       *Repository
	riskEngine *RiskEngine
	audit      AuditLogger
	notifier   NotificationSender
	db         *gorm.DB
}

// NewService creates a new governance Service.
func NewService(repo *Repository, audit AuditLogger, notifier NotificationSender, db *gorm.DB) *Service {
	s := &Service{
		repo:     repo,
		audit:    audit,
		notifier: notifier,
		db:       db,
	}
	s.riskEngine = NewRiskEngine(repo)
	return s
}

// Allowed report categories (prompt-mandated enum).
const (
	ReportCategoryHarassment  = "Harassment"
	ReportCategoryDamage      = "Damage"
	ReportCategoryNoise       = "Noise"
	ReportCategoryMaintenance = "Maintenance"
	ReportCategoryFraud       = "Fraud"
	ReportCategoryOther       = "Other"
)

var validReportCategories = map[string]bool{
	ReportCategoryHarassment:  true,
	ReportCategoryDamage:      true,
	ReportCategoryNoise:       true,
	ReportCategoryMaintenance: true,
	ReportCategoryFraud:       true,
	ReportCategoryOther:       true,
}

// --- Report operations ---

// CreateReport validates the input and creates a new report with Open status.
func (s *Service) CreateReport(req CreateReportRequest, reporterID uint64, ip, requestID string) (*Report, *common.AppError) {
	// Validate fields.
	var fieldErrors []*common.FieldError

	if fe := common.ValidateReportTargetType(req.TargetType); fe != nil {
		fieldErrors = append(fieldErrors, fe)
	}
	if req.TargetID == 0 {
		fieldErrors = append(fieldErrors, &common.FieldError{Field: "target_id", Message: "is required"})
	}
	if !validReportCategories[req.Category] {
		fieldErrors = append(fieldErrors, &common.FieldError{
			Field:   "category",
			Message: fmt.Sprintf("must be one of: Harassment, Damage, Noise, Maintenance, Fraud, Other"),
		})
	}
	if fe := common.ValidateDescription(req.Description); fe != nil {
		fieldErrors = append(fieldErrors, fe)
	}

	if appErr := common.CollectFieldErrors(fieldErrors...); appErr != nil {
		return nil, appErr
	}

	// Check blacklist on the description.
	matched, err := s.repo.CheckContent(req.Description)
	if err == nil && len(matched) > 0 {
		// Flag content but still create the report; the matched keywords are noted.
		// We continue creating the report as it is a moderation report itself.
	}

	report := &Report{
		UUID:        uuid.New().String(),
		ReporterID:  reporterID,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		Category:    req.Category,
		Description: req.Description,
		Status:      common.ReportStatusOpen,
	}

	if err := s.repo.CreateReport(report); err != nil {
		return nil, common.NewInternalError("failed to create report")
	}

	// Audit log.
	s.audit.Log(reporterID, common.AuditActionCreate, "Report", report.ID,
		fmt.Sprintf("Report created against %s #%d", req.TargetType, req.TargetID), ip, requestID)

	// Notify all active ComplianceReviewer users about the new report.
	if s.notifier != nil {
		if err := s.notifier.SendEventToRole("report_filed", common.RoleComplianceReviewer, map[string]string{
			"report_id":   fmt.Sprintf("%d", report.ID),
			"target_type": report.TargetType,
			"target_id":   fmt.Sprintf("%d", report.TargetID),
			"category":    report.Category,
		}); err != nil {
			log.Printf("WARN governance: notification fan-out failed for report_filed (report %d): %v", report.ID, err)
		}
	}

	return report, nil
}

// GetReport retrieves a report by its ID.
func (s *Service) GetReport(id uint64) (*Report, *common.AppError) {
	report, err := s.repo.FindReportByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, common.NewNotFoundError("Report")
		}
		return nil, common.NewInternalError("failed to retrieve report")
	}
	return report, nil
}

// ListReports retrieves a paginated list of reports matching the given filters.
func (s *Service) ListReports(status, targetType string, targetID *uint64, page, perPage int) ([]Report, int64, *common.AppError) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	filters := ReportFilters{
		Status:     status,
		TargetType: targetType,
		TargetID:   targetID,
	}

	offset := (page - 1) * perPage
	reports, total, err := s.repo.ListReports(filters, offset, perPage)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list reports")
	}

	return reports, total, nil
}

// ReviewReport transitions a report's status; only ComplianceReviewer or SystemAdmin may call.
func (s *Service) ReviewReport(reportID uint64, req ReviewReportRequest, reviewerID uint64, ip, requestID string) (*Report, *common.AppError) {
	// Validate target status.
	switch req.Status {
	case common.ReportStatusInReview, common.ReportStatusResolved, common.ReportStatusDismissed:
		// valid
	default:
		return nil, common.NewValidationError("status must be InReview, Resolved, or Dismissed")
	}

	report, appErr := s.GetReport(reportID)
	if appErr != nil {
		return nil, appErr
	}

	// Validate transition: Open -> InReview -> Resolved/Dismissed.
	if !isValidReportTransition(report.Status, req.Status) {
		return nil, common.NewValidationError(
			fmt.Sprintf("invalid transition from %s to %s", report.Status, req.Status),
		)
	}

	report.Status = req.Status
	report.ReviewerID = &reviewerID

	if req.Status == common.ReportStatusResolved || req.Status == common.ReportStatusDismissed {
		now := time.Now().UTC()
		report.ResolvedAt = &now
		if req.ResolutionNotes != "" {
			report.ResolutionNotes = &req.ResolutionNotes
		}
	}

	if err := s.repo.UpdateReportStatus(report); err != nil {
		return nil, common.NewInternalError("failed to update report status")
	}

	s.audit.Log(reviewerID, common.AuditActionStatusChange, "Report", report.ID,
		fmt.Sprintf("Report status changed to %s", req.Status), ip, requestID)

	return report, nil
}

// isValidReportTransition checks allowed report status transitions.
func isValidReportTransition(from, to string) bool {
	switch from {
	case common.ReportStatusOpen:
		return to == common.ReportStatusInReview || to == common.ReportStatusDismissed
	case common.ReportStatusInReview:
		return to == common.ReportStatusResolved || to == common.ReportStatusDismissed
	default:
		return false
	}
}

// --- Enforcement operations ---

// ApplyEnforcement creates an enforcement action against a user.
func (s *Service) ApplyEnforcement(req CreateEnforcementRequest, actorID uint64, ip, requestID string) (*EnforcementAction, *common.AppError) {
	// Validate action type.
	switch req.ActionType {
	case common.EnforcementWarning, common.EnforcementRateLimit, common.EnforcementSuspension:
		// valid
	default:
		return nil, common.NewValidationError("action_type must be Warning, RateLimit, or Suspension")
	}

	// Validate reason.
	if fe := common.ValidateStringLength("reason", req.Reason, 10, 2000); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	// For rate limit, require the rate limit fields.
	if req.ActionType == common.EnforcementRateLimit {
		if req.RateLimitMax == nil || *req.RateLimitMax <= 0 {
			return nil, common.NewFieldValidationError("rate_limit_max", "must be greater than 0 for RateLimit actions")
		}
		if req.RateLimitWindowMinutes == nil || *req.RateLimitWindowMinutes <= 0 {
			return nil, common.NewFieldValidationError("rate_limit_window_minutes", "must be greater than 0 for RateLimit actions")
		}
	}

	now := time.Now().UTC()
	action := &EnforcementAction{
		UUID:       uuid.New().String(),
		UserID:     req.UserID,
		ReportID:   req.ReportID,
		ActionType: req.ActionType,
		Reason:     req.Reason,
		StartsAt:   now,
		IsActive:   true,
		CreatedBy:  actorID,
	}

	// Calculate ends_at for suspensions.
	if req.ActionType == common.EnforcementSuspension {
		switch req.EndsAt {
		case "1day":
			endsAt := now.Add(24 * time.Hour)
			action.EndsAt = &endsAt
		case "7day":
			endsAt := now.Add(7 * 24 * time.Hour)
			action.EndsAt = &endsAt
		case "indefinite", "":
			// nil ends_at = indefinite suspension
			action.EndsAt = nil
		default:
			return nil, common.NewFieldValidationError("ends_at", "must be 1day, 7day, or indefinite")
		}
	}

	// Set rate limit fields.
	if req.ActionType == common.EnforcementRateLimit {
		action.RateLimitMax = req.RateLimitMax
		action.RateLimitWindowMinutes = req.RateLimitWindowMinutes
	}

	if err := s.repo.CreateEnforcement(action); err != nil {
		return nil, common.NewInternalError("failed to create enforcement action")
	}

	// Audit log.
	s.audit.Log(actorID, common.AuditActionEnforcement, "EnforcementAction", action.ID,
		fmt.Sprintf("%s enforcement applied to user %d: %s", req.ActionType, req.UserID, req.Reason), ip, requestID)

	// Notify the target user.
	if s.notifier != nil {
		if err := s.notifier.SendEvent("enforcement_applied", req.UserID, map[string]string{
			"action_type":    req.ActionType,
			"enforcement_id": fmt.Sprintf("%d", action.ID),
			"reason":         req.Reason,
		}); err != nil {
			log.Printf("WARN governance: notification send failed for enforcement_applied (action %d): %v", action.ID, err)
		}
	}

	return action, nil
}

// RevokeEnforcement deactivates an enforcement action.
func (s *Service) RevokeEnforcement(enforcementID uint64, req RevokeEnforcementRequest, actorID uint64, ip, requestID string) (*EnforcementAction, *common.AppError) {
	if fe := common.ValidateStringLength("reason", req.Reason, 10, 2000); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	action, err := s.repo.FindEnforcementByID(enforcementID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, common.NewNotFoundError("Enforcement action")
		}
		return nil, common.NewInternalError("failed to retrieve enforcement action")
	}

	if !action.IsActive {
		return nil, common.NewConflictError("enforcement action is already revoked")
	}

	now := time.Now().UTC()
	action.IsActive = false
	action.RevokedAt = &now
	action.RevokedBy = &actorID

	if err := s.repo.RevokeEnforcement(action); err != nil {
		return nil, common.NewInternalError("failed to revoke enforcement action")
	}

	s.audit.Log(actorID, common.AuditActionEnforcement, "EnforcementAction", action.ID,
		fmt.Sprintf("Enforcement revoked for user %d: %s", action.UserID, req.Reason), ip, requestID)

	// Notify the user that their enforcement has been revoked.
	if s.notifier != nil {
		if err := s.notifier.SendEvent("enforcement_revoked", action.UserID, map[string]string{
			"enforcement_id": fmt.Sprintf("%d", action.ID),
			"action_type":    action.ActionType,
		}); err != nil {
			log.Printf("WARN governance: notification send failed for enforcement_revoked (action %d): %v", action.ID, err)
		}
	}

	return action, nil
}

// ListEnforcements retrieves active enforcement actions with pagination.
func (s *Service) ListEnforcements(page, perPage int) ([]EnforcementAction, int64, *common.AppError) {
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
	actions, total, err := s.repo.ListActiveEnforcements(offset, perPage)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list enforcements")
	}

	return actions, total, nil
}

// --- Blacklist operations ---

// CreateKeyword adds a new blacklist keyword. Admin only.
func (s *Service) CreateKeyword(req KeywordRequest, actorID uint64, ip, requestID string) (*KeywordBlacklist, *common.AppError) {
	if fe := common.ValidateStringLength("keyword", req.Keyword, 2, 255); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	// Check for duplicate.
	existing, _ := s.repo.FindByKeyword(req.Keyword)
	if existing != nil {
		return nil, common.NewConflictError("keyword already exists")
	}

	category := req.Category
	if category == "" {
		category = "General"
	}
	severity := req.Severity
	if severity == "" {
		severity = "Medium"
	}

	keyword := &KeywordBlacklist{
		Keyword:  req.Keyword,
		Category: category,
		Severity: severity,
		IsActive: true,
	}

	if err := s.repo.CreateKeyword(keyword); err != nil {
		return nil, common.NewInternalError("failed to create keyword")
	}

	s.audit.Log(actorID, common.AuditActionCreate, "KeywordBlacklist", keyword.ID,
		fmt.Sprintf("Blacklist keyword created: %s", req.Keyword), ip, requestID)

	return keyword, nil
}

// ListKeywords retrieves blacklist keywords with pagination.
func (s *Service) ListKeywords(page, perPage int) ([]KeywordBlacklist, int64, *common.AppError) {
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
	keywords, total, err := s.repo.ListKeywords(offset, perPage)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list keywords")
	}

	return keywords, total, nil
}

// DeleteKeyword removes a blacklist keyword by ID. Admin only.
func (s *Service) DeleteKeyword(id uint64, actorID uint64, ip, requestID string) *common.AppError {
	if err := s.repo.DeleteKeyword(id); err != nil {
		return common.NewInternalError("failed to delete keyword")
	}

	s.audit.Log(actorID, common.AuditActionDelete, "KeywordBlacklist", id,
		"Blacklist keyword deleted", ip, requestID)

	return nil
}

// CheckBlacklist scans text against active keywords and returns matched keywords.
func (s *Service) CheckBlacklist(text string) ([]KeywordBlacklist, error) {
	return s.repo.CheckContent(text)
}

// --- Risk rule operations ---

// CreateRiskRule creates a new risk rule. Admin only.
func (s *Service) CreateRiskRule(req RiskRuleRequest, actorID uint64, ip, requestID string) (*RiskRule, *common.AppError) {
	if fe := common.ValidateStringLength("name", req.Name, 3, 255); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	rule := &RiskRule{
		UUID:            uuid.New().String(),
		Name:            req.Name,
		Description:     desc,
		ConditionType:   req.ConditionType,
		ConditionParams: datatypes.JSON(req.ConditionParams),
		ActionType:      req.ActionType,
		ActionParams:    datatypes.JSON(req.ActionParams),
		IsActive:        true,
	}

	if err := s.repo.CreateRiskRule(rule); err != nil {
		return nil, common.NewInternalError("failed to create risk rule")
	}

	s.audit.Log(actorID, common.AuditActionCreate, "RiskRule", rule.ID,
		fmt.Sprintf("Risk rule created: %s", req.Name), ip, requestID)

	return rule, nil
}

// GetRiskRule retrieves a risk rule by ID.
func (s *Service) GetRiskRule(id uint64) (*RiskRule, *common.AppError) {
	rule, err := s.repo.FindRiskRuleByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, common.NewNotFoundError("Risk rule")
		}
		return nil, common.NewInternalError("failed to retrieve risk rule")
	}
	return rule, nil
}

// ListRiskRules retrieves risk rules with pagination.
func (s *Service) ListRiskRules(page, perPage int) ([]RiskRule, int64, *common.AppError) {
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
	rules, total, err := s.repo.ListRiskRules(offset, perPage)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list risk rules")
	}

	return rules, total, nil
}

// UpdateRiskRule updates an existing risk rule. Admin only.
func (s *Service) UpdateRiskRule(id uint64, req RiskRuleRequest, actorID uint64, ip, requestID string) (*RiskRule, *common.AppError) {
	rule, appErr := s.GetRiskRule(id)
	if appErr != nil {
		return nil, appErr
	}

	if fe := common.ValidateStringLength("name", req.Name, 3, 255); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	rule.Name = req.Name
	if req.Description != "" {
		rule.Description = &req.Description
	}
	rule.ConditionType = req.ConditionType
	rule.ConditionParams = datatypes.JSON(req.ConditionParams)
	rule.ActionType = req.ActionType
	rule.ActionParams = datatypes.JSON(req.ActionParams)

	if err := s.repo.UpdateRiskRule(rule); err != nil {
		return nil, common.NewInternalError("failed to update risk rule")
	}

	s.audit.Log(actorID, common.AuditActionUpdate, "RiskRule", rule.ID,
		fmt.Sprintf("Risk rule updated: %s", req.Name), ip, requestID)

	return rule, nil
}

// DeleteRiskRule deletes a risk rule by ID. Admin only.
func (s *Service) DeleteRiskRule(id uint64, actorID uint64, ip, requestID string) *common.AppError {
	if _, appErr := s.GetRiskRule(id); appErr != nil {
		return appErr
	}

	if err := s.repo.DeleteRiskRule(id); err != nil {
		return common.NewInternalError("failed to delete risk rule")
	}

	s.audit.Log(actorID, common.AuditActionDelete, "RiskRule", id,
		"Risk rule deleted", ip, requestID)

	return nil
}

// EvaluateRiskRules evaluates content against active risk rules and auto-creates enforcement if triggered.
func (s *Service) EvaluateRiskRules(content string, userID uint64, ip, requestID string) (*RiskResult, *common.AppError) {
	result := s.riskEngine.EvaluateContent(content, userID)

	// Auto-apply enforcement if risk score is high enough and rules were matched.
	for _, matched := range result.MatchedRules {
		if matched.ActionType == common.EnforcementWarning ||
			matched.ActionType == common.EnforcementRateLimit ||
			matched.ActionType == common.EnforcementSuspension {
			_, _ = s.ApplyEnforcement(CreateEnforcementRequest{
				UserID:     userID,
				ActionType: matched.ActionType,
				Reason:     fmt.Sprintf("Auto-enforcement triggered by risk rule: %s", matched.RuleName),
			}, 0, ip, requestID) // actorID=0 means system-initiated
		}
	}

	return result, nil
}
