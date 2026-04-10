package common

import "time"

// Pagination holds pagination parameters extracted from query strings.
type Pagination struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

// Offset returns the SQL OFFSET value for the current page.
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// TimeRange represents a time-based filter range.
type TimeRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// ContextKey is a typed key for storing values in Gin context to avoid string collisions.
type ContextKey string

const (
	// CtxKeyUserID stores the authenticated user's ID in context.
	CtxKeyUserID ContextKey = "user_id"
	// CtxKeyUsername stores the authenticated user's username in context.
	CtxKeyUsername ContextKey = "username"
	// CtxKeyRoles stores the authenticated user's roles in context.
	CtxKeyRoles ContextKey = "roles"
	// CtxKeyRequestID stores the unique request ID in context.
	CtxKeyRequestID ContextKey = "request_id"
	// CtxKeySessionID stores the session ID in context.
	CtxKeySessionID ContextKey = "session_id"
)
