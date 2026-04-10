package common

import (
	"net/http"
	"testing"
)

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("bad input", FieldError{Field: "name", Message: "required"})
	if err.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", err.Code)
	}
	if err.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", err.HTTPStatus)
	}
	if len(err.Fields) != 1 {
		t.Errorf("expected 1 field error, got %d", len(err.Fields))
	}
	if err.Fields[0].Field != "name" {
		t.Errorf("expected field 'name', got '%s'", err.Fields[0].Field)
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("Work Order")
	if err.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %s", err.Code)
	}
	if err.HTTPStatus != http.StatusNotFound {
		t.Errorf("expected 404, got %d", err.HTTPStatus)
	}
	if err.Message != "Work Order not found" {
		t.Errorf("unexpected message: %s", err.Message)
	}
}

func TestNewUnauthorizedError(t *testing.T) {
	err := NewUnauthorizedError("")
	if err.Message != "Authentication required" {
		t.Errorf("expected default message, got %s", err.Message)
	}
	if err.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", err.HTTPStatus)
	}
}

func TestNewForbiddenError(t *testing.T) {
	err := NewForbiddenError("insufficient role")
	if err.HTTPStatus != http.StatusForbidden {
		t.Errorf("expected 403, got %d", err.HTTPStatus)
	}
}

func TestNewRateLimitError(t *testing.T) {
	err := NewRateLimitError()
	if err.HTTPStatus != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", err.HTTPStatus)
	}
}

func TestNewConflictError(t *testing.T) {
	err := NewConflictError("duplicate")
	if err.HTTPStatus != http.StatusConflict {
		t.Errorf("expected 409, got %d", err.HTTPStatus)
	}
}

func TestNewPayloadTooLargeError(t *testing.T) {
	err := NewPayloadTooLargeError("file too big")
	if err.HTTPStatus != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", err.HTTPStatus)
	}
}

func TestAppErrorImplementsError(t *testing.T) {
	err := NewInternalError("test")
	var e error = err
	if e.Error() != "[INTERNAL_ERROR] test" {
		t.Errorf("unexpected error string: %s", e.Error())
	}
}
