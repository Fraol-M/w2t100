package governance

import (
	"encoding/json"
	"strings"
)

// RiskResult holds the output of a risk evaluation.
type RiskResult struct {
	Score        int           `json:"score"`
	MatchedRules []MatchedRule `json:"matched_rules,omitempty"`
	Keywords     []string      `json:"keywords,omitempty"`
}

// MatchedRule represents a risk rule that was triggered during evaluation.
type MatchedRule struct {
	RuleID     uint64 `json:"rule_id"`
	RuleName   string `json:"rule_name"`
	ActionType string `json:"action_type"`
	Score      int    `json:"score"`
}

// FrequencyThresholdParams holds parameters for a FrequencyThreshold condition.
type FrequencyThresholdParams struct {
	MaxCount      int `json:"max_count"`
	WindowMinutes int `json:"window_minutes"`
}

// KeywordMatchParams holds parameters for a KeywordMatch condition.
type KeywordMatchParams struct {
	Keywords []string `json:"keywords"`
}

// RiskEngine evaluates content against risk rules and blacklisted keywords.
type RiskEngine struct {
	repo *Repository
}

// NewRiskEngine creates a new RiskEngine.
func NewRiskEngine(repo *Repository) *RiskEngine {
	return &RiskEngine{repo: repo}
}

// EvaluateContent checks content against blacklisted keywords and active risk rules,
// returning a risk score and list of matched rules.
func (e *RiskEngine) EvaluateContent(content string, userID uint64) *RiskResult {
	result := &RiskResult{
		Score: 0,
	}

	// Step 1: Check blacklist keywords.
	matched, err := e.repo.CheckContent(content)
	if err == nil && len(matched) > 0 {
		for _, kw := range matched {
			result.Keywords = append(result.Keywords, kw.Keyword)
			switch kw.Severity {
			case "High":
				result.Score += 30
			case "Medium":
				result.Score += 20
			case "Low":
				result.Score += 10
			default:
				result.Score += 15
			}
		}
	}

	// Step 2: Evaluate active risk rules.
	rules, err := e.repo.FindActiveRules()
	if err != nil {
		return result
	}

	lower := strings.ToLower(content)

	for _, rule := range rules {
		switch rule.ConditionType {
		case "KeywordMatch":
			if e.evaluateKeywordMatch(lower, rule) {
				score := 25
				result.Score += score
				result.MatchedRules = append(result.MatchedRules, MatchedRule{
					RuleID:     rule.ID,
					RuleName:   rule.Name,
					ActionType: rule.ActionType,
					Score:      score,
				})
			}

		case "FrequencyThreshold":
			if e.evaluateFrequencyThreshold(userID, rule) {
				score := 40
				result.Score += score
				result.MatchedRules = append(result.MatchedRules, MatchedRule{
					RuleID:     rule.ID,
					RuleName:   rule.Name,
					ActionType: rule.ActionType,
					Score:      score,
				})
			}
		}
	}

	return result
}

// evaluateKeywordMatch checks if content contains any keywords defined in the rule.
func (e *RiskEngine) evaluateKeywordMatch(lowerContent string, rule RiskRule) bool {
	var params KeywordMatchParams
	if err := json.Unmarshal(rule.ConditionParams, &params); err != nil {
		return false
	}

	for _, kw := range params.Keywords {
		if strings.Contains(lowerContent, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// evaluateFrequencyThreshold checks if a user has exceeded the frequency threshold
// by counting how many reports they have filed within the configured time window.
func (e *RiskEngine) evaluateFrequencyThreshold(userID uint64, rule RiskRule) bool {
	var params FrequencyThresholdParams
	if err := json.Unmarshal(rule.ConditionParams, &params); err != nil {
		return false
	}
	if params.MaxCount <= 0 || params.WindowMinutes <= 0 {
		return false
	}

	count, err := e.repo.CountRecentReports(userID, params.WindowMinutes)
	if err != nil {
		return false
	}
	return count >= int64(params.MaxCount)
}
