package common

import (
	"fmt"
	"net/http"
)

// FieldError represents a validation error on a specific field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// AppError is the standard application error type used across all services and handlers.
type AppError struct {
	Code       string       `json:"code"`
	Message    string       `json:"message"`
	HTTPStatus int          `json:"-"`
	Fields     []FieldError `json:"errors,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// --- Constructor helpers ---

func NewValidationError(message string, fields ...FieldError) *AppError {
	return &AppError{
		Code:       "VALIDATION_ERROR",
		Message:    message,
		HTTPStatus: http.StatusUnprocessableEntity,
		Fields:     fields,
	}
}

func NewFieldValidationError(field, message string) *AppError {
	return &AppError{
		Code:       "VALIDATION_ERROR",
		Message:    "Validation failed",
		HTTPStatus: http.StatusUnprocessableEntity,
		Fields:     []FieldError{{Field: field, Message: message}},
	}
}

func NewNotFoundError(resource string) *AppError {
	return &AppError{
		Code:       "NOT_FOUND",
		Message:    fmt.Sprintf("%s not found", resource),
		HTTPStatus: http.StatusNotFound,
	}
}

func NewUnauthorizedError(message string) *AppError {
	if message == "" {
		message = "Authentication required"
	}
	return &AppError{
		Code:       "UNAUTHORIZED",
		Message:    message,
		HTTPStatus: http.StatusUnauthorized,
	}
}

func NewForbiddenError(message string) *AppError {
	if message == "" {
		message = "Access denied"
	}
	return &AppError{
		Code:       "FORBIDDEN",
		Message:    message,
		HTTPStatus: http.StatusForbidden,
	}
}

func NewConflictError(message string) *AppError {
	return &AppError{
		Code:       "CONFLICT",
		Message:    message,
		HTTPStatus: http.StatusConflict,
	}
}

func NewRateLimitError() *AppError {
	return &AppError{
		Code:       "RATE_LIMITED",
		Message:    "Too many requests. Please try again later.",
		HTTPStatus: http.StatusTooManyRequests,
	}
}

func NewInternalError(message string) *AppError {
	if message == "" {
		message = "Internal server error"
	}
	return &AppError{
		Code:       "INTERNAL_ERROR",
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
	}
}

func NewBadRequestError(message string) *AppError {
	return &AppError{
		Code:       "BAD_REQUEST",
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

func NewPayloadTooLargeError(message string) *AppError {
	return &AppError{
		Code:       "PAYLOAD_TOO_LARGE",
		Message:    message,
		HTTPStatus: http.StatusRequestEntityTooLarge,
	}
}
