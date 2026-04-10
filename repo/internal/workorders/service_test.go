package workorders

import (
	"testing"
	"time"

	"propertyops/backend/internal/common"
)

// --- SLA Calculation Tests ---

func TestCalculateSLA_Emergency(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	due := CalculateSLA(common.PriorityEmergency, now)
	expected := now.Add(4 * time.Hour)
	if !due.Equal(expected) {
		t.Errorf("Emergency SLA: expected %v, got %v", expected, due)
	}
}

func TestCalculateSLA_High(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	due := CalculateSLA(common.PriorityHigh, now)
	expected := now.Add(24 * time.Hour)
	if !due.Equal(expected) {
		t.Errorf("High SLA: expected %v, got %v", expected, due)
	}
}

func TestCalculateSLA_Normal(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	due := CalculateSLA(common.PriorityNormal, now)
	expected := now.Add(72 * time.Hour)
	if !due.Equal(expected) {
		t.Errorf("Normal SLA: expected %v, got %v", expected, due)
	}
}

func TestCalculateSLA_Low_WeekdaysOnly(t *testing.T) {
	// Monday 2025-06-16 10:00 UTC
	monday := time.Date(2025, 6, 16, 10, 0, 0, 0, time.UTC)
	due := CalculateSLA(common.PriorityLow, monday)

	// 5 business days from Monday -> next Monday (skip Sat+Sun)
	expected := time.Date(2025, 6, 23, 10, 0, 0, 0, time.UTC)
	if !due.Equal(expected) {
		t.Errorf("Low SLA from Monday: expected %v, got %v", expected, due)
	}
}

func TestCalculateSLA_Low_CrossesWeekend(t *testing.T) {
	// Wednesday 2025-06-18 10:00 UTC
	wednesday := time.Date(2025, 6, 18, 10, 0, 0, 0, time.UTC)
	due := CalculateSLA(common.PriorityLow, wednesday)

	// Wed + 5 business days: Thu, Fri, (skip Sat, Sun), Mon, Tue, Wed
	// Expected: Wed June 25
	expected := time.Date(2025, 6, 25, 10, 0, 0, 0, time.UTC)
	if !due.Equal(expected) {
		t.Errorf("Low SLA from Wednesday: expected %v, got %v", expected, due)
	}
}

func TestCalculateSLA_Low_StartFriday(t *testing.T) {
	// Friday 2025-06-20 10:00 UTC
	friday := time.Date(2025, 6, 20, 10, 0, 0, 0, time.UTC)
	due := CalculateSLA(common.PriorityLow, friday)

	// Fri + 5 business days: Mon, Tue, Wed, Thu, Fri
	// Expected: Friday June 27
	expected := time.Date(2025, 6, 27, 10, 0, 0, 0, time.UTC)
	if !due.Equal(expected) {
		t.Errorf("Low SLA from Friday: expected %v, got %v", expected, due)
	}
}

func TestCalculateSLA_Low_StartSaturday(t *testing.T) {
	// Saturday 2025-06-21 10:00 UTC
	saturday := time.Date(2025, 6, 21, 10, 0, 0, 0, time.UTC)
	due := CalculateSLA(common.PriorityLow, saturday)

	// Sat: Sun(skip), Mon(1), Tue(2), Wed(3), Thu(4), Fri(5)
	// Expected: Friday June 27
	expected := time.Date(2025, 6, 27, 10, 0, 0, 0, time.UTC)
	if !due.Equal(expected) {
		t.Errorf("Low SLA from Saturday: expected %v (Friday), got %v", expected, due)
	}
}

// --- State Machine Transition Tests ---

func TestIsValidTransition_ValidPaths(t *testing.T) {
	validTransitions := []struct {
		from string
		to   string
	}{
		{common.WOStatusNew, common.WOStatusAssigned},
		{common.WOStatusAssigned, common.WOStatusInProgress},
		{common.WOStatusInProgress, common.WOStatusAwaitingApproval},
		{common.WOStatusAwaitingApproval, common.WOStatusCompleted},
		{common.WOStatusAwaitingApproval, common.WOStatusInProgress},
		{common.WOStatusCompleted, common.WOStatusArchived},
	}

	for _, tt := range validTransitions {
		if !IsValidTransition(tt.from, tt.to) {
			t.Errorf("expected transition %s -> %s to be valid", tt.from, tt.to)
		}
	}
}

func TestIsValidTransition_InvalidPaths(t *testing.T) {
	invalidTransitions := []struct {
		from string
		to   string
	}{
		{common.WOStatusNew, common.WOStatusInProgress},
		{common.WOStatusNew, common.WOStatusCompleted},
		{common.WOStatusAssigned, common.WOStatusCompleted},
		{common.WOStatusAssigned, common.WOStatusAwaitingApproval},
		{common.WOStatusInProgress, common.WOStatusCompleted},
		{common.WOStatusInProgress, common.WOStatusAssigned},
		{common.WOStatusCompleted, common.WOStatusInProgress},
		{common.WOStatusCompleted, common.WOStatusNew},
		{common.WOStatusArchived, common.WOStatusNew},
		{common.WOStatusArchived, common.WOStatusCompleted},
		// Skip a step.
		{common.WOStatusNew, common.WOStatusAwaitingApproval},
		{common.WOStatusNew, common.WOStatusArchived},
	}

	for _, tt := range invalidTransitions {
		if IsValidTransition(tt.from, tt.to) {
			t.Errorf("expected transition %s -> %s to be invalid", tt.from, tt.to)
		}
	}
}

func TestIsValidTransition_UnknownStatus(t *testing.T) {
	if IsValidTransition("Unknown", common.WOStatusAssigned) {
		t.Error("expected transition from unknown status to be invalid")
	}
	if IsValidTransition(common.WOStatusNew, "Unknown") {
		t.Error("expected transition to unknown status to be invalid")
	}
}

// --- Rating Validation Tests ---

func TestValidateRating_Valid(t *testing.T) {
	for _, r := range []int{1, 2, 3, 4, 5} {
		if fe := common.ValidateRating(r); fe != nil {
			t.Errorf("expected rating %d to be valid, got error: %s", r, fe.Message)
		}
	}
}

func TestValidateRating_Invalid(t *testing.T) {
	for _, r := range []int{0, -1, 6, 100} {
		if fe := common.ValidateRating(r); fe == nil {
			t.Errorf("expected rating %d to be invalid", r)
		}
	}
}

// --- Reassign Reason Validation Tests ---

func TestValidateReassignReason_Valid(t *testing.T) {
	validReasons := []string{
		"Reassigning due to scheduling conflict with technician",
		"0123456789", // exactly 10 chars
	}
	for _, reason := range validReasons {
		if fe := common.ValidateReassignReason(reason); fe != nil {
			t.Errorf("expected reason %q to be valid, got error: %s", reason, fe.Message)
		}
	}
}

func TestValidateReassignReason_TooShort(t *testing.T) {
	if fe := common.ValidateReassignReason("short"); fe == nil {
		t.Error("expected short reason to be invalid")
	}
}

func TestValidateReassignReason_TooLong(t *testing.T) {
	longReason := make([]byte, 501)
	for i := range longReason {
		longReason[i] = 'a'
	}
	if fe := common.ValidateReassignReason(string(longReason)); fe == nil {
		t.Error("expected 501-char reason to be invalid")
	}
}

// --- Feedback Validation Tests ---

func TestValidateFeedback_Valid(t *testing.T) {
	if fe := common.ValidateFeedback("Great job fixing the leak!"); fe != nil {
		t.Errorf("expected valid feedback, got error: %s", fe.Message)
	}
	if fe := common.ValidateFeedback(""); fe != nil {
		t.Errorf("expected empty feedback to be valid, got error: %s", fe.Message)
	}
}

func TestValidateFeedback_TooLong(t *testing.T) {
	long := make([]byte, 1001)
	for i := range long {
		long[i] = 'a'
	}
	if fe := common.ValidateFeedback(string(long)); fe == nil {
		t.Error("expected feedback > 1000 chars to be invalid")
	}
}

// --- addBusinessDays helper Tests ---

func TestAddBusinessDays_FromMonday(t *testing.T) {
	monday := time.Date(2025, 6, 16, 12, 0, 0, 0, time.UTC)
	result := addBusinessDays(monday, 5)
	expected := time.Date(2025, 6, 23, 12, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestAddBusinessDays_FromThursday(t *testing.T) {
	thursday := time.Date(2025, 6, 19, 12, 0, 0, 0, time.UTC)
	result := addBusinessDays(thursday, 3)
	// Thu->Fri(1), Sat(skip), Sun(skip), Mon(2), Tue(3)
	expected := time.Date(2025, 6, 24, 12, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestAddBusinessDays_Zero(t *testing.T) {
	monday := time.Date(2025, 6, 16, 12, 0, 0, 0, time.UTC)
	result := addBusinessDays(monday, 0)
	if !result.Equal(monday) {
		t.Errorf("expected same date for 0 business days, got %v", result)
	}
}
