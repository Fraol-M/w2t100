package governance

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// Repository handles all governance database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new governance Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// --- Report operations ---

// CreateReport inserts a new report record.
func (r *Repository) CreateReport(report *Report) error {
	return r.db.Create(report).Error
}

// FindReportByID loads a report by its primary key.
func (r *Repository) FindReportByID(id uint64) (*Report, error) {
	var report Report
	err := r.db.Where("id = ?", id).First(&report).Error
	if err != nil {
		return nil, err
	}
	return &report, nil
}

// ReportFilters holds optional filters for listing reports.
type ReportFilters struct {
	Status     string
	TargetType string
	TargetID   *uint64
	ReporterID *uint64
}

// ListReports retrieves reports matching the given filters with pagination.
func (r *Repository) ListReports(filters ReportFilters, offset, limit int) ([]Report, int64, error) {
	query := r.db.Model(&Report{})

	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.TargetType != "" {
		query = query.Where("target_type = ?", filters.TargetType)
	}
	if filters.TargetID != nil {
		query = query.Where("target_id = ?", *filters.TargetID)
	}
	if filters.ReporterID != nil {
		query = query.Where("reporter_id = ?", *filters.ReporterID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var reports []Report
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&reports).Error
	if err != nil {
		return nil, 0, err
	}

	return reports, total, nil
}

// UpdateReportStatus updates the status and related fields of a report.
func (r *Repository) UpdateReportStatus(report *Report) error {
	return r.db.Save(report).Error
}

// --- Enforcement operations ---

// CreateEnforcement inserts a new enforcement action record.
func (r *Repository) CreateEnforcement(action *EnforcementAction) error {
	return r.db.Create(action).Error
}

// FindEnforcementByID loads an enforcement action by its primary key.
func (r *Repository) FindEnforcementByID(id uint64) (*EnforcementAction, error) {
	var action EnforcementAction
	err := r.db.Where("id = ?", id).First(&action).Error
	if err != nil {
		return nil, err
	}
	return &action, nil
}

// FindActiveByUser retrieves all active enforcement actions for a user.
func (r *Repository) FindActiveByUser(userID uint64) ([]EnforcementAction, error) {
	var actions []EnforcementAction
	err := r.db.Where("user_id = ? AND is_active = ?", userID, true).
		Order("created_at DESC").
		Find(&actions).Error
	return actions, err
}

// FindActiveByUserAndType retrieves active enforcement actions for a user of a specific type.
func (r *Repository) FindActiveByUserAndType(userID uint64, actionType string) ([]EnforcementAction, error) {
	var actions []EnforcementAction
	err := r.db.Where("user_id = ? AND action_type = ? AND is_active = ?", userID, actionType, true).
		Find(&actions).Error
	return actions, err
}

// RevokeEnforcement updates an enforcement action to mark it as revoked.
func (r *Repository) RevokeEnforcement(action *EnforcementAction) error {
	return r.db.Save(action).Error
}

// ListActiveEnforcements retrieves all active enforcement actions with pagination.
func (r *Repository) ListActiveEnforcements(offset, limit int) ([]EnforcementAction, int64, error) {
	query := r.db.Model(&EnforcementAction{}).Where("is_active = ?", true)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var actions []EnforcementAction
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&actions).Error
	if err != nil {
		return nil, 0, err
	}

	return actions, total, nil
}

// --- Keyword blacklist operations ---

// CreateKeyword inserts a new blacklist keyword record.
func (r *Repository) CreateKeyword(keyword *KeywordBlacklist) error {
	return r.db.Create(keyword).Error
}

// FindByKeyword loads a keyword entry by its keyword value.
func (r *Repository) FindByKeyword(keyword string) (*KeywordBlacklist, error) {
	var kw KeywordBlacklist
	err := r.db.Where("keyword = ?", keyword).First(&kw).Error
	if err != nil {
		return nil, err
	}
	return &kw, nil
}

// ListKeywords retrieves all blacklist keywords with pagination.
func (r *Repository) ListKeywords(offset, limit int) ([]KeywordBlacklist, int64, error) {
	query := r.db.Model(&KeywordBlacklist{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var keywords []KeywordBlacklist
	err := query.Order("keyword ASC").Offset(offset).Limit(limit).Find(&keywords).Error
	if err != nil {
		return nil, 0, err
	}

	return keywords, total, nil
}

// DeleteKeyword deletes a keyword record by its ID.
func (r *Repository) DeleteKeyword(id uint64) error {
	return r.db.Delete(&KeywordBlacklist{}, id).Error
}

// CheckContent scans text against all active keywords and returns matched keywords.
func (r *Repository) CheckContent(text string) ([]KeywordBlacklist, error) {
	var allKeywords []KeywordBlacklist
	err := r.db.Where("is_active = ?", true).Find(&allKeywords).Error
	if err != nil {
		return nil, err
	}

	lower := strings.ToLower(text)
	var matched []KeywordBlacklist
	for _, kw := range allKeywords {
		if strings.Contains(lower, strings.ToLower(kw.Keyword)) {
			matched = append(matched, kw)
		}
	}
	return matched, nil
}

// --- Risk rule operations ---

// CreateRiskRule inserts a new risk rule record.
func (r *Repository) CreateRiskRule(rule *RiskRule) error {
	return r.db.Create(rule).Error
}

// FindRiskRuleByID loads a risk rule by its primary key.
func (r *Repository) FindRiskRuleByID(id uint64) (*RiskRule, error) {
	var rule RiskRule
	err := r.db.Where("id = ?", id).First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListRiskRules retrieves all risk rules with pagination.
func (r *Repository) ListRiskRules(offset, limit int) ([]RiskRule, int64, error) {
	query := r.db.Model(&RiskRule{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rules []RiskRule
	err := query.Order("name ASC").Offset(offset).Limit(limit).Find(&rules).Error
	if err != nil {
		return nil, 0, err
	}

	return rules, total, nil
}

// UpdateRiskRule saves changes to an existing risk rule.
func (r *Repository) UpdateRiskRule(rule *RiskRule) error {
	return r.db.Save(rule).Error
}

// DeleteRiskRule deletes a risk rule by its ID.
func (r *Repository) DeleteRiskRule(id uint64) error {
	return r.db.Delete(&RiskRule{}, id).Error
}

// FindActiveRules retrieves all active risk rules.
func (r *Repository) FindActiveRules() ([]RiskRule, error) {
	var rules []RiskRule
	err := r.db.Where("is_active = ?", true).Find(&rules).Error
	return rules, err
}

// CountRecentReports counts how many reports a user has filed within the last windowMinutes minutes.
// This is used by the FrequencyThreshold risk rule to detect excessive report filing.
func (r *Repository) CountRecentReports(userID uint64, windowMinutes int) (int64, error) {
	var count int64
	since := time.Now().UTC().Add(-time.Duration(windowMinutes) * time.Minute)
	err := r.db.Model(&Report{}).
		Where("reporter_id = ? AND created_at >= ?", userID, since).
		Count(&count).Error
	return count, err
}
