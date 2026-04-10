package governance

import (
	"encoding/json"
	"testing"

	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database with governance tables.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("SQLite unavailable (CGO disabled or no C compiler): %v", err)
	}
	if err := db.AutoMigrate(&KeywordBlacklist{}, &RiskRule{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestRiskEngine_KeywordMatching_SingleMatch(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Seed keywords.
	db.Create(&KeywordBlacklist{Keyword: "scam", Category: "Fraud", Severity: "High", IsActive: true})
	db.Create(&KeywordBlacklist{Keyword: "threat", Category: "Violence", Severity: "Medium", IsActive: true})

	engine := NewRiskEngine(repo)
	result := engine.EvaluateContent("this is a scam message", 1)

	if len(result.Keywords) != 1 {
		t.Fatalf("expected 1 matched keyword, got %d", len(result.Keywords))
	}
	if result.Keywords[0] != "scam" {
		t.Errorf("expected keyword 'scam', got %q", result.Keywords[0])
	}
	if result.Score != 30 {
		t.Errorf("expected score 30 for High severity, got %d", result.Score)
	}
}

func TestRiskEngine_KeywordMatching_MultipleMatches(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	db.Create(&KeywordBlacklist{Keyword: "scam", Category: "Fraud", Severity: "High", IsActive: true})
	db.Create(&KeywordBlacklist{Keyword: "threat", Category: "Violence", Severity: "Medium", IsActive: true})
	db.Create(&KeywordBlacklist{Keyword: "spam", Category: "General", Severity: "Low", IsActive: true})

	engine := NewRiskEngine(repo)
	result := engine.EvaluateContent("this is a scam threat with spam", 1)

	if len(result.Keywords) != 3 {
		t.Fatalf("expected 3 matched keywords, got %d", len(result.Keywords))
	}
	// High(30) + Medium(20) + Low(10) = 60
	if result.Score != 60 {
		t.Errorf("expected score 60, got %d", result.Score)
	}
}

func TestRiskEngine_KeywordMatching_CaseInsensitive(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	db.Create(&KeywordBlacklist{Keyword: "SCAM", Category: "Fraud", Severity: "High", IsActive: true})

	engine := NewRiskEngine(repo)
	result := engine.EvaluateContent("This is a Scam Message", 1)

	if len(result.Keywords) != 1 {
		t.Fatalf("expected 1 matched keyword, got %d", len(result.Keywords))
	}
}

func TestRiskEngine_KeywordMatching_InactiveIgnored(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	db.Create(&KeywordBlacklist{Keyword: "scam", Category: "Fraud", Severity: "High", IsActive: false})

	engine := NewRiskEngine(repo)
	result := engine.EvaluateContent("this is a scam message", 1)

	if len(result.Keywords) != 0 {
		t.Errorf("expected 0 matched keywords for inactive keyword, got %d", len(result.Keywords))
	}
}

func TestRiskEngine_KeywordMatching_NoMatch(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	db.Create(&KeywordBlacklist{Keyword: "scam", Category: "Fraud", Severity: "High", IsActive: true})

	engine := NewRiskEngine(repo)
	result := engine.EvaluateContent("this is a perfectly normal message", 1)

	if len(result.Keywords) != 0 {
		t.Errorf("expected 0 matched keywords, got %d", len(result.Keywords))
	}
	if result.Score != 0 {
		t.Errorf("expected score 0, got %d", result.Score)
	}
}

func TestRiskEngine_KeywordMatchRule(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	params, _ := json.Marshal(KeywordMatchParams{Keywords: []string{"urgent", "act now"}})
	db.Create(&RiskRule{
		UUID:            "test-uuid-1",
		Name:            "Urgent language detector",
		ConditionType:   "KeywordMatch",
		ConditionParams: datatypes.JSON(params),
		ActionType:      "Warning",
		IsActive:        true,
	})

	engine := NewRiskEngine(repo)
	result := engine.EvaluateContent("You must act now before it is too late", 1)

	if len(result.MatchedRules) != 1 {
		t.Fatalf("expected 1 matched rule, got %d", len(result.MatchedRules))
	}
	if result.MatchedRules[0].RuleName != "Urgent language detector" {
		t.Errorf("expected rule name 'Urgent language detector', got %q", result.MatchedRules[0].RuleName)
	}
	if result.MatchedRules[0].ActionType != "Warning" {
		t.Errorf("expected action type 'Warning', got %q", result.MatchedRules[0].ActionType)
	}
	if result.Score != 25 {
		t.Errorf("expected score 25, got %d", result.Score)
	}
}

func TestRiskEngine_KeywordMatchRule_NoMatch(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	params, _ := json.Marshal(KeywordMatchParams{Keywords: []string{"urgent", "act now"}})
	db.Create(&RiskRule{
		UUID:            "test-uuid-2",
		Name:            "Urgent language detector",
		ConditionType:   "KeywordMatch",
		ConditionParams: datatypes.JSON(params),
		ActionType:      "Warning",
		IsActive:        true,
	})

	engine := NewRiskEngine(repo)
	result := engine.EvaluateContent("A completely normal description of a maintenance issue", 1)

	if len(result.MatchedRules) != 0 {
		t.Errorf("expected 0 matched rules, got %d", len(result.MatchedRules))
	}
}

func TestRiskEngine_FrequencyThreshold_StructureParsing(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	params, _ := json.Marshal(FrequencyThresholdParams{MaxCount: 5, WindowMinutes: 60})
	db.Create(&RiskRule{
		UUID:            "test-uuid-3",
		Name:            "Rapid submission detector",
		ConditionType:   "FrequencyThreshold",
		ConditionParams: datatypes.JSON(params),
		ActionType:      "RateLimit",
		IsActive:        true,
	})

	engine := NewRiskEngine(repo)
	// The frequency threshold evaluation is a placeholder that returns false
	// since it requires external state. This test confirms no panic/error.
	result := engine.EvaluateContent("some content", 1)

	if len(result.MatchedRules) != 0 {
		t.Errorf("expected 0 matched rules from frequency threshold placeholder, got %d", len(result.MatchedRules))
	}
}

func TestRiskEngine_CombinedKeywordAndRuleMatch(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Blacklist keyword.
	db.Create(&KeywordBlacklist{Keyword: "fraud", Category: "Financial", Severity: "High", IsActive: true})

	// Risk rule with different keyword.
	params, _ := json.Marshal(KeywordMatchParams{Keywords: []string{"money laundering"}})
	db.Create(&RiskRule{
		UUID:            "test-uuid-4",
		Name:            "Financial crime detector",
		ConditionType:   "KeywordMatch",
		ConditionParams: datatypes.JSON(params),
		ActionType:      "Suspension",
		IsActive:        true,
	})

	engine := NewRiskEngine(repo)
	result := engine.EvaluateContent("this involves fraud and money laundering", 1)

	// Blacklist: High(30) + Rule: 25 = 55
	if result.Score != 55 {
		t.Errorf("expected combined score 55, got %d", result.Score)
	}
	if len(result.Keywords) != 1 {
		t.Errorf("expected 1 blacklist keyword, got %d", len(result.Keywords))
	}
	if len(result.MatchedRules) != 1 {
		t.Errorf("expected 1 matched rule, got %d", len(result.MatchedRules))
	}
}
