package common

import (
	"strings"
	"testing"
)

func TestValidateDescription(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"too short", "short", true},
		{"exactly 20", strings.Repeat("a", 20), false},
		{"valid length", strings.Repeat("a", 100), false},
		{"exactly 2000", strings.Repeat("a", 2000), false},
		{"too long", strings.Repeat("a", 2001), true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDescription(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDescription() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateReassignReason(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"too short", "short", true},
		{"exactly 10", strings.Repeat("a", 10), false},
		{"exactly 500", strings.Repeat("a", 500), false},
		{"too long", strings.Repeat("a", 501), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReassignReason(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateReassignReason() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRating(t *testing.T) {
	tests := []struct {
		rating  int
		wantErr bool
	}{
		{0, true}, {1, false}, {3, false}, {5, false}, {6, true}, {-1, true},
	}
	for _, tt := range tests {
		err := ValidateRating(tt.rating)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateRating(%d) error = %v, wantErr %v", tt.rating, err, tt.wantErr)
		}
	}
}

func TestValidateFeedback(t *testing.T) {
	if err := ValidateFeedback("good work"); err != nil {
		t.Errorf("expected no error for short feedback, got %v", err)
	}
	if err := ValidateFeedback(strings.Repeat("a", 1000)); err != nil {
		t.Errorf("expected no error for 1000 chars, got %v", err)
	}
	if err := ValidateFeedback(strings.Repeat("a", 1001)); err == nil {
		t.Error("expected error for 1001 chars")
	}
}

func TestValidatePriority(t *testing.T) {
	valid := []string{"Low", "Normal", "High", "Emergency"}
	for _, p := range valid {
		if err := ValidatePriority(p); err != nil {
			t.Errorf("ValidatePriority(%s) unexpected error: %v", p, err)
		}
	}
	if err := ValidatePriority("Critical"); err == nil {
		t.Error("expected error for invalid priority")
	}
}

func TestValidateDateFormat(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"01/15/2025", false},
		{"12/31/2024", false},
		{"13/01/2025", true},
		{"00/01/2025", true},
		{"2025-01-15", true},
		{"1/15/2025", true},
		{"", true},
	}
	for _, tt := range tests {
		err := ValidateDateFormat("date", tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateDateFormat(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
	}
}

func TestValidateTimeFormat(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"2:30 PM", false},
		{"12:00 AM", false},
		{"11:59 pm", false},
		{"0:30 PM", true},
		{"13:00 PM", true},
		{"2:30", true},
	}
	for _, tt := range tests {
		err := ValidateTimeFormat("time", tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateTimeFormat(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
	}
}

func TestValidateMoneyAmount(t *testing.T) {
	tests := []struct {
		amount  float64
		wantErr bool
	}{
		{100.00, false},
		{0.01, false},
		{99.99, false},
		{0, true},
		{-5.00, true},
		{100.001, true},
	}
	for _, tt := range tests {
		err := ValidateMoneyAmount("amount", tt.amount)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateMoneyAmount(%v) error = %v, wantErr %v", tt.amount, err, tt.wantErr)
		}
	}
}

func TestValidateFileSignature(t *testing.T) {
	// JPEG signature: FF D8 FF
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}
	mime, err := ValidateFileSignature(jpegHeader)
	if err != nil {
		t.Errorf("expected JPEG detected, got error: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mime)
	}

	// PNG signature: 89 50 4E 47 0D 0A 1A 0A
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mime, err = ValidateFileSignature(pngHeader)
	if err != nil {
		t.Errorf("expected PNG detected, got error: %v", err)
	}
	if mime != "image/png" {
		t.Errorf("expected image/png, got %s", mime)
	}

	// Invalid signature
	_, err = ValidateFileSignature([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if err == nil {
		t.Error("expected error for invalid signature")
	}

	// Too short
	_, err = ValidateFileSignature([]byte{0xFF})
	if err == nil {
		t.Error("expected error for too-short header")
	}
}

func TestValidateAttachmentCount(t *testing.T) {
	if err := ValidateAttachmentCount(0, 1, 6); err != nil {
		t.Errorf("expected no error for 0+1<=6, got %v", err)
	}
	if err := ValidateAttachmentCount(5, 1, 6); err != nil {
		t.Errorf("expected no error for 5+1<=6, got %v", err)
	}
	if err := ValidateAttachmentCount(6, 1, 6); err == nil {
		t.Error("expected error for 6+1>6")
	}
}

func TestValidateExportPurpose(t *testing.T) {
	if err := ValidateExportPurpose(""); err == nil {
		t.Error("expected error for empty purpose")
	}
	if err := ValidateExportPurpose("short"); err == nil {
		t.Error("expected error for short purpose")
	}
	if err := ValidateExportPurpose("Legal compliance audit requirement"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCollectFieldErrors(t *testing.T) {
	// All nil — should return nil
	if err := CollectFieldErrors(nil, nil); err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	// Mixed — should collect non-nil
	e1 := &FieldError{Field: "a", Message: "bad"}
	err := CollectFieldErrors(nil, e1, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(err.Fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(err.Fields))
	}
}

func TestValidateImageMIME(t *testing.T) {
	if err := ValidateImageMIME("image/jpeg"); err != nil {
		t.Errorf("expected no error for JPEG, got %v", err)
	}
	if err := ValidateImageMIME("image/png"); err != nil {
		t.Errorf("expected no error for PNG, got %v", err)
	}
	if err := ValidateImageMIME("image/gif"); err == nil {
		t.Error("expected error for GIF")
	}
	if err := ValidateImageMIME("application/pdf"); err == nil {
		t.Error("expected error for PDF")
	}
}
