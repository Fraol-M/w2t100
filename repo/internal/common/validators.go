package common

import (
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// --- String length validators ---

// ValidateDescription checks the description field is 20–2000 characters.
func ValidateDescription(s string) *FieldError {
	l := utf8.RuneCountInString(s)
	if l < 20 {
		return &FieldError{Field: "description", Message: "must be at least 20 characters"}
	}
	if l > 2000 {
		return &FieldError{Field: "description", Message: "must be at most 2000 characters"}
	}
	return nil
}

// ValidateReassignReason checks the reason field is 10–500 characters.
func ValidateReassignReason(s string) *FieldError {
	l := utf8.RuneCountInString(s)
	if l < 10 {
		return &FieldError{Field: "reason", Message: "must be at least 10 characters"}
	}
	if l > 500 {
		return &FieldError{Field: "reason", Message: "must be at most 500 characters"}
	}
	return nil
}

// ValidateRating checks rating is 1–5.
func ValidateRating(rating int) *FieldError {
	if rating < 1 || rating > 5 {
		return &FieldError{Field: "rating", Message: "must be between 1 and 5"}
	}
	return nil
}

// ValidateFeedback checks optional feedback is <= 1000 characters.
func ValidateFeedback(s string) *FieldError {
	if utf8.RuneCountInString(s) > 1000 {
		return &FieldError{Field: "feedback", Message: "must be at most 1000 characters"}
	}
	return nil
}

// ValidateStringLength validates a named string field within min–max rune count.
func ValidateStringLength(field, value string, min, max int) *FieldError {
	l := utf8.RuneCountInString(value)
	if l < min {
		return &FieldError{Field: field, Message: fmt.Sprintf("must be at least %d characters", min)}
	}
	if l > max {
		return &FieldError{Field: field, Message: fmt.Sprintf("must be at most %d characters", max)}
	}
	return nil
}

// --- Enum validators ---

// ValidatePriority checks the priority value is one of the allowed enums.
func ValidatePriority(p string) *FieldError {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityEmergency:
		return nil
	}
	return &FieldError{Field: "priority", Message: "must be Low, Normal, High, or Emergency"}
}

// ValidateCostType checks cost_type is Labor or Material.
func ValidateCostType(t string) *FieldError {
	switch t {
	case CostTypeLabor, CostTypeMaterial:
		return nil
	}
	return &FieldError{Field: "cost_type", Message: "must be Labor or Material"}
}

// ValidateResponsibility checks responsibility is Tenant, Vendor, or Property.
func ValidateResponsibility(r string) *FieldError {
	switch r {
	case ResponsibilityTenant, ResponsibilityVendor, ResponsibilityProperty:
		return nil
	}
	return &FieldError{Field: "responsibility", Message: "must be Tenant, Vendor, or Property"}
}

// ValidateReportTargetType checks the report target type.
func ValidateReportTargetType(t string) *FieldError {
	switch t {
	case ReportTargetTenant, ReportTargetWorkOrder, ReportTargetThread:
		return nil
	}
	return &FieldError{Field: "target_type", Message: "must be Tenant, WorkOrder, or Thread"}
}

// --- Date/time validators ---

var dateRegex = regexp.MustCompile(`^(0[1-9]|1[0-2])/(0[1-9]|[12]\d|3[01])/\d{4}$`)
var time12Regex = regexp.MustCompile(`^(0?[1-9]|1[0-2]):[0-5]\d\s?(AM|PM|am|pm)$`)

// ValidateDateFormat checks date matches MM/DD/YYYY.
func ValidateDateFormat(field, value string) *FieldError {
	if !dateRegex.MatchString(value) {
		return &FieldError{Field: field, Message: "must be in MM/DD/YYYY format"}
	}
	return nil
}

// ValidateTimeFormat checks time matches 12-hour format (e.g., 2:30 PM).
func ValidateTimeFormat(field, value string) *FieldError {
	if !time12Regex.MatchString(value) {
		return &FieldError{Field: field, Message: "must be in 12-hour format (e.g., 2:30 PM)"}
	}
	return nil
}

// ParseDateMMDDYYYY parses a MM/DD/YYYY date string into a time.Time.
func ParseDateMMDDYYYY(s string) (time.Time, error) {
	return time.Parse("01/02/2006", s)
}

// ParseTime12Hour parses a 12-hour time string (e.g., "2:30 PM") into hours and minutes.
func ParseTime12Hour(s string) (hour, minute int, err error) {
	s = strings.TrimSpace(s)
	t, err := time.Parse("3:04 PM", s)
	if err != nil {
		t, err = time.Parse("3:04PM", s)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid 12-hour time format")
		}
	}
	return t.Hour(), t.Minute(), nil
}

// --- Money validator ---

// ValidateMoneyAmount checks a USD amount has at most 2 decimal places and is > 0.
func ValidateMoneyAmount(field string, amount float64) *FieldError {
	if amount <= 0 {
		return &FieldError{Field: field, Message: "must be greater than 0"}
	}
	// Check at most 2 decimal places
	rounded := math.Round(amount*100) / 100
	if math.Abs(amount-rounded) > 0.001 {
		return &FieldError{Field: field, Message: "must have at most 2 decimal places"}
	}
	return nil
}

// --- Attachment validators ---

// ValidateImageMIME checks if the MIME type is an allowed image type.
func ValidateImageMIME(mime string) *FieldError {
	mime = strings.ToLower(strings.TrimSpace(mime))
	if mime != "image/jpeg" && mime != "image/png" {
		return &FieldError{Field: "file", Message: "must be JPEG or PNG image"}
	}
	return nil
}

// ValidateFileSignature checks the magic bytes of a file to confirm it matches the declared MIME type.
// Returns the detected MIME type and any validation error.
func ValidateFileSignature(header []byte) (string, *FieldError) {
	if len(header) < 8 {
		return "", &FieldError{Field: "file", Message: "file too small to validate"}
	}
	// JPEG: FF D8 FF
	if header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF {
		return "image/jpeg", nil
	}
	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if header[0] == 0x89 && header[1] == 0x50 && header[2] == 0x4E && header[3] == 0x47 &&
		header[4] == 0x0D && header[5] == 0x0A && header[6] == 0x1A && header[7] == 0x0A {
		return "image/png", nil
	}
	return "", &FieldError{Field: "file", Message: "file signature does not match JPEG or PNG"}
}

// ValidateAttachmentCount checks if adding more attachments would exceed the limit.
func ValidateAttachmentCount(current, adding, max int) *FieldError {
	if current+adding > max {
		return &FieldError{Field: "attachments", Message: fmt.Sprintf("maximum %d attachments allowed", max)}
	}
	return nil
}

// DetectContentType uses http.DetectContentType on the first 512 bytes.
func DetectContentType(data []byte) string {
	return http.DetectContentType(data)
}

// --- Export purpose validator ---

// ValidateExportPurpose checks that an export purpose string is provided.
func ValidateExportPurpose(purpose string) *FieldError {
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		return &FieldError{Field: "purpose", Message: "export purpose is required"}
	}
	if utf8.RuneCountInString(purpose) < 10 {
		return &FieldError{Field: "purpose", Message: "export purpose must be at least 10 characters"}
	}
	return nil
}

// CollectFieldErrors gathers non-nil field errors into an AppError or returns nil.
func CollectFieldErrors(errs ...*FieldError) *AppError {
	var fields []FieldError
	for _, e := range errs {
		if e != nil {
			fields = append(fields, *e)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return &AppError{
		Code:       "VALIDATION_ERROR",
		Message:    "Validation failed",
		HTTPStatus: 422,
		Fields:     fields,
	}
}
