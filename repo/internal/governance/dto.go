package governance

import (
	"encoding/json"
	"time"
)

// --- Request DTOs ---

// CreateReportRequest is the payload for creating a new report.
type CreateReportRequest struct {
	TargetType  string `json:"target_type" binding:"required"`
	TargetID    uint64 `json:"target_id" binding:"required"`
	Category    string `json:"category" binding:"required"`
	Description string `json:"description" binding:"required"`
}

// ReviewReportRequest is the payload for reviewing a report.
type ReviewReportRequest struct {
	Status          string `json:"status" binding:"required"`
	ResolutionNotes string `json:"resolution_notes,omitempty"`
}

// CreateEnforcementRequest is the payload for applying an enforcement action.
type CreateEnforcementRequest struct {
	UserID                 uint64  `json:"user_id" binding:"required"`
	ActionType             string  `json:"action_type" binding:"required"`
	Reason                 string  `json:"reason" binding:"required"`
	EndsAt                 string  `json:"ends_at,omitempty"` // "1day", "7day", "indefinite" for suspensions
	RateLimitMax           *int    `json:"rate_limit_max,omitempty"`
	RateLimitWindowMinutes *int    `json:"rate_limit_window_minutes,omitempty"`
	ReportID               *uint64 `json:"report_id,omitempty"`
}

// RevokeEnforcementRequest is the payload for revoking an enforcement action.
type RevokeEnforcementRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// KeywordRequest is the payload for creating or updating a blacklist keyword.
type KeywordRequest struct {
	Keyword  string `json:"keyword" binding:"required"`
	Category string `json:"category,omitempty"`
	Severity string `json:"severity,omitempty"`
}

// RiskRuleRequest is the payload for creating or updating a risk rule.
type RiskRuleRequest struct {
	Name            string          `json:"name" binding:"required"`
	Description     string          `json:"description,omitempty"`
	ConditionType   string          `json:"condition_type" binding:"required"`
	ConditionParams json.RawMessage `json:"condition_params" binding:"required"`
	ActionType      string          `json:"action_type" binding:"required"`
	ActionParams    json.RawMessage `json:"action_params,omitempty"`
}

// --- Response DTOs ---

// ReportResponse is the API response DTO for a report.
type ReportResponse struct {
	ID              uint64     `json:"id"`
	UUID            string     `json:"uuid"`
	ReporterID      uint64     `json:"reporter_id"`
	TargetType      string     `json:"target_type"`
	TargetID        uint64     `json:"target_id"`
	Category        string     `json:"category"`
	Description     string     `json:"description"`
	Status          string     `json:"status"`
	ReviewerID      *uint64    `json:"reviewer_id,omitempty"`
	ResolutionNotes *string    `json:"resolution_notes,omitempty"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// EnforcementResponse is the API response DTO for an enforcement action.
type EnforcementResponse struct {
	ID                     uint64     `json:"id"`
	UUID                   string     `json:"uuid"`
	UserID                 uint64     `json:"user_id"`
	ReportID               *uint64    `json:"report_id,omitempty"`
	ActionType             string     `json:"action_type"`
	Reason                 string     `json:"reason"`
	StartsAt               time.Time  `json:"starts_at"`
	EndsAt                 *time.Time `json:"ends_at,omitempty"`
	IsActive               bool       `json:"is_active"`
	RateLimitMax           *int       `json:"rate_limit_max,omitempty"`
	RateLimitWindowMinutes *int       `json:"rate_limit_window_minutes,omitempty"`
	CreatedBy              uint64     `json:"created_by"`
	RevokedAt              *time.Time `json:"revoked_at,omitempty"`
	RevokedBy              *uint64    `json:"revoked_by,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// KeywordResponse is the API response DTO for a blacklist keyword.
type KeywordResponse struct {
	ID        uint64    `json:"id"`
	Keyword   string    `json:"keyword"`
	Category  string    `json:"category"`
	Severity  string    `json:"severity"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RiskRuleResponse is the API response DTO for a risk rule.
type RiskRuleResponse struct {
	ID              uint64          `json:"id"`
	UUID            string          `json:"uuid"`
	Name            string          `json:"name"`
	Description     *string         `json:"description,omitempty"`
	ConditionType   string          `json:"condition_type"`
	ConditionParams json.RawMessage `json:"condition_params"`
	ActionType      string          `json:"action_type"`
	ActionParams    json.RawMessage `json:"action_params,omitempty"`
	IsActive        bool            `json:"is_active"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// --- Converters ---

// ToReportResponse converts a Report model to its API response DTO.
func ToReportResponse(r *Report) ReportResponse {
	return ReportResponse{
		ID:              r.ID,
		UUID:            r.UUID,
		ReporterID:      r.ReporterID,
		TargetType:      r.TargetType,
		TargetID:        r.TargetID,
		Category:        r.Category,
		Description:     r.Description,
		Status:          r.Status,
		ReviewerID:      r.ReviewerID,
		ResolutionNotes: r.ResolutionNotes,
		ResolvedAt:      r.ResolvedAt,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

// ToEnforcementResponse converts an EnforcementAction model to its API response DTO.
func ToEnforcementResponse(e *EnforcementAction) EnforcementResponse {
	return EnforcementResponse{
		ID:                     e.ID,
		UUID:                   e.UUID,
		UserID:                 e.UserID,
		ReportID:               e.ReportID,
		ActionType:             e.ActionType,
		Reason:                 e.Reason,
		StartsAt:               e.StartsAt,
		EndsAt:                 e.EndsAt,
		IsActive:               e.IsActive,
		RateLimitMax:           e.RateLimitMax,
		RateLimitWindowMinutes: e.RateLimitWindowMinutes,
		CreatedBy:              e.CreatedBy,
		RevokedAt:              e.RevokedAt,
		RevokedBy:              e.RevokedBy,
		CreatedAt:              e.CreatedAt,
		UpdatedAt:              e.UpdatedAt,
	}
}

// ToKeywordResponse converts a KeywordBlacklist model to its API response DTO.
func ToKeywordResponse(k *KeywordBlacklist) KeywordResponse {
	return KeywordResponse{
		ID:        k.ID,
		Keyword:   k.Keyword,
		Category:  k.Category,
		Severity:  k.Severity,
		IsActive:  k.IsActive,
		CreatedAt: k.CreatedAt,
		UpdatedAt: k.UpdatedAt,
	}
}

// ToRiskRuleResponse converts a RiskRule model to its API response DTO.
func ToRiskRuleResponse(r *RiskRule) RiskRuleResponse {
	return RiskRuleResponse{
		ID:              r.ID,
		UUID:            r.UUID,
		Name:            r.Name,
		Description:     r.Description,
		ConditionType:   r.ConditionType,
		ConditionParams: json.RawMessage(r.ConditionParams),
		ActionType:      r.ActionType,
		ActionParams:    json.RawMessage(r.ActionParams),
		IsActive:        r.IsActive,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}
