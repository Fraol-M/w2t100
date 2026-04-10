package audit

import (
	"encoding/json"
	"log"
	"time"

	"propertyops/backend/internal/common"

	"github.com/google/uuid"
)

// Service provides audit logging operations.
type Service struct {
	repo *Repository
}

// NewService creates a new audit Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Log creates an audit log entry. Implements the AuditLogger interface used by middleware
// and other packages throughout the application.
// Sensitive fields such as passwords, tokens, and raw PII must never appear in the
// description or resource identifiers passed to this method.
func (s *Service) Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string) {
	var aID *uint64
	if actorID != 0 {
		id := actorID
		aID = &id
	}
	var rID *uint64
	if resourceID != 0 {
		id := resourceID
		rID = &id
	}

	entry := &AuditLog{
		UUID:         uuid.New().String(),
		ActorID:      aID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   rID,
		Description:  description,
		IPAddress:    ip,
		RequestID:    requestID,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.repo.Create(entry); err != nil {
		log.Printf("ERROR audit.Service.Log: failed to persist audit entry action=%s resource=%s: %v", action, resourceType, err)
	}
}

// LogWithValues creates an audit log entry that captures the before and after state of
// a resource. oldValues and newValues are marshalled to JSON; if either is nil the
// corresponding column is left as a JSON null.
// Sensitive fields (passwords, tokens, encryption keys, raw phone numbers) must be
// omitted or masked by the caller before passing values here.
func (s *Service) LogWithValues(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string, oldValues, newValues interface{}) {
	var aID *uint64
	if actorID != 0 {
		id := actorID
		aID = &id
	}
	var rID *uint64
	if resourceID != 0 {
		id := resourceID
		rID = &id
	}

	entry := &AuditLog{
		UUID:         uuid.New().String(),
		ActorID:      aID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   rID,
		Description:  description,
		IPAddress:    ip,
		RequestID:    requestID,
		CreatedAt:    time.Now().UTC(),
	}

	if oldValues != nil {
		if b, err := json.Marshal(oldValues); err == nil {
			entry.OldValues = b
		} else {
			log.Printf("WARN audit.Service.LogWithValues: failed to marshal oldValues: %v", err)
		}
	}
	if newValues != nil {
		if b, err := json.Marshal(newValues); err == nil {
			entry.NewValues = b
		} else {
			log.Printf("WARN audit.Service.LogWithValues: failed to marshal newValues: %v", err)
		}
	}

	if err := s.repo.Create(entry); err != nil {
		log.Printf("ERROR audit.Service.LogWithValues: failed to persist audit entry action=%s resource=%s: %v", action, resourceType, err)
	}
}

// GetByID returns a single audit log by primary key.
func (s *Service) GetByID(id uint64) (*AuditLog, *common.AppError) {
	return s.repo.GetByID(id)
}

// List returns paginated audit logs matching the given filters.
func (s *Service) List(filters ListFilters) ([]AuditLog, int64, error) {
	return s.repo.List(filters)
}
