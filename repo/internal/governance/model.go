package governance

import (
	"time"

	"gorm.io/datatypes"
)

// Report represents a user-submitted report against a tenant, work order, or thread.
type Report struct {
	ID              uint64     `gorm:"primaryKey" json:"id"`
	UUID            string     `gorm:"size:36;uniqueIndex" json:"uuid"`
	ReporterID      uint64     `gorm:"index" json:"reporter_id"`
	TargetType      string     `gorm:"size:30" json:"target_type"`
	TargetID        uint64     `json:"target_id"`
	Category        string     `gorm:"size:50" json:"category"`
	Description     string     `gorm:"type:text" json:"description"`
	Status          string     `gorm:"size:20;default:Open" json:"status"`
	ReviewerID      *uint64    `json:"reviewer_id,omitempty"`
	ResolutionNotes *string    `gorm:"type:text" json:"resolution_notes,omitempty"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TableName overrides the default table name.
func (Report) TableName() string {
	return "reports"
}

// EnforcementAction represents an enforcement action taken against a user.
type EnforcementAction struct {
	ID                     uint64     `gorm:"primaryKey" json:"id"`
	UUID                   string     `gorm:"size:36;uniqueIndex" json:"uuid"`
	UserID                 uint64     `gorm:"index" json:"user_id"`
	ReportID               *uint64    `json:"report_id,omitempty"`
	ActionType             string     `gorm:"size:30" json:"action_type"`
	Reason                 string     `gorm:"type:text" json:"reason"`
	StartsAt               time.Time  `json:"starts_at"`
	EndsAt                 *time.Time `json:"ends_at,omitempty"`
	IsActive               bool       `gorm:"default:true" json:"is_active"`
	RateLimitMax           *int       `json:"rate_limit_max,omitempty"`
	RateLimitWindowMinutes *int       `json:"rate_limit_window_minutes,omitempty"`
	CreatedBy              uint64     `json:"created_by"`
	RevokedAt              *time.Time `json:"revoked_at,omitempty"`
	RevokedBy              *uint64    `json:"revoked_by,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// TableName overrides the default table name.
func (EnforcementAction) TableName() string {
	return "enforcement_actions"
}

// KeywordBlacklist represents a blacklisted keyword for content moderation.
type KeywordBlacklist struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Keyword   string    `gorm:"size:255;uniqueIndex" json:"keyword"`
	Category  string    `gorm:"size:50;default:General" json:"category"`
	Severity  string    `gorm:"size:20;default:Medium" json:"severity"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName overrides the default table name.
func (KeywordBlacklist) TableName() string {
	return "keywords_blacklist"
}

// RiskRule defines an automated risk assessment rule.
type RiskRule struct {
	ID              uint64         `gorm:"primaryKey" json:"id"`
	UUID            string         `gorm:"size:36;uniqueIndex" json:"uuid"`
	Name            string         `gorm:"size:255;uniqueIndex" json:"name"`
	Description     *string        `gorm:"type:text" json:"description,omitempty"`
	ConditionType   string         `gorm:"size:50" json:"condition_type"`
	ConditionParams datatypes.JSON `gorm:"type:json" json:"condition_params"`
	ActionType      string         `gorm:"size:30" json:"action_type"`
	ActionParams    datatypes.JSON `gorm:"type:json" json:"action_params,omitempty"`
	IsActive        bool           `gorm:"default:true" json:"is_active"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// TableName overrides the default table name.
func (RiskRule) TableName() string {
	return "risk_rules"
}
