package notifications

import "time"

// --- Notification DTOs ---

// SendNotificationRequest is the payload for sending a direct notification.
type SendNotificationRequest struct {
	RecipientID  uint64  `json:"recipient_id" binding:"required"`
	Subject      string  `json:"subject" binding:"required"`
	Body         string  `json:"body" binding:"required"`
	Category     string  `json:"category,omitempty"`
	ScheduledFor *string `json:"scheduled_for,omitempty"`
}

// NotificationListRequest holds filter and pagination parameters for listing notifications.
type NotificationListRequest struct {
	Status   string `form:"status"`
	Category string `form:"category"`
	ReadFlag string `form:"read"` // "true", "false", or "" (all)
	Page     int    `form:"page"`
	PerPage  int    `form:"per_page"`
}

// MarkReadRequest is the payload for marking a notification as read.
type MarkReadRequest struct {
	// No body fields required; notification ID comes from the URL param.
}

// NotificationResponse is the API response DTO for a single notification.
type NotificationResponse struct {
	ID           uint64     `json:"id"`
	UUID         string     `json:"uuid"`
	RecipientID  uint64     `json:"recipient_id"`
	TemplateID   *uint64    `json:"template_id,omitempty"`
	Subject      string     `json:"subject"`
	Body         string     `json:"body"`
	Category     string     `json:"category"`
	Status       string     `json:"status"`
	IsRead       bool       `json:"is_read"`
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	SentAt       *time.Time `json:"sent_at,omitempty"`
	EntityType   string     `json:"entity_type,omitempty"`
	EntityID     *uint64    `json:"entity_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// UnreadCountResponse is the API response for unread notification count.
type UnreadCountResponse struct {
	Count int64 `json:"count"`
}

// TemplateResponse is the API response DTO for a notification template.
type TemplateResponse struct {
	ID              uint64    `json:"id"`
	Name            string    `json:"name"`
	SubjectTemplate string    `json:"subject_template"`
	BodyTemplate    string    `json:"body_template"`
	Category        string    `json:"category"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
}

// --- Thread DTOs ---

// CreateThreadRequest is the payload for creating a new message thread.
type CreateThreadRequest struct {
	WorkOrderID *uint64 `json:"work_order_id,omitempty"`
	Subject     string  `json:"subject" binding:"required"`
}

// AddParticipantRequest is the payload for adding a user to a thread.
type AddParticipantRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
}

// ParticipantResponse is the API response DTO for a thread participant.
type ParticipantResponse struct {
	ThreadID uint64    `json:"thread_id"`
	UserID   uint64    `json:"user_id"`
	JoinedAt time.Time `json:"joined_at"`
}

// AddMessageRequest is the payload for adding a message to a thread.
type AddMessageRequest struct {
	Body string `json:"body" binding:"required"`
}

// ThreadResponse is the API response DTO for a message thread.
type ThreadResponse struct {
	ID          uint64    `json:"id"`
	UUID        string    `json:"uuid"`
	WorkOrderID *uint64   `json:"work_order_id,omitempty"`
	Subject     string    `json:"subject"`
	CreatedBy   uint64    `json:"created_by"`
	IsClosed    bool      `json:"is_closed"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ThreadMessageResponse is the API response DTO for a single thread message.
type ThreadMessageResponse struct {
	ID        uint64    `json:"id"`
	UUID      string    `json:"uuid"`
	ThreadID  uint64    `json:"thread_id"`
	SenderID  uint64    `json:"sender_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// --- Conversion helpers ---

// ToNotificationResponse converts a Notification model to its API response DTO.
func ToNotificationResponse(n *Notification, isRead bool) NotificationResponse {
	return NotificationResponse{
		ID:           n.ID,
		UUID:         n.UUID,
		RecipientID:  n.RecipientID,
		TemplateID:   n.TemplateID,
		Subject:      n.Subject,
		Body:         n.Body,
		Category:     n.Category,
		Status:       n.Status,
		IsRead:       isRead,
		ScheduledFor: n.ScheduledFor,
		SentAt:       n.SentAt,
		EntityType:   n.EntityType,
		EntityID:     n.EntityID,
		CreatedAt:    n.CreatedAt,
	}
}

// ToTemplateResponse converts a NotificationTemplate model to its API response DTO.
func ToTemplateResponse(t *NotificationTemplate) TemplateResponse {
	return TemplateResponse{
		ID:              t.ID,
		Name:            t.Name,
		SubjectTemplate: t.SubjectTemplate,
		BodyTemplate:    t.BodyTemplate,
		Category:        t.Category,
		IsActive:        t.IsActive,
		CreatedAt:       t.CreatedAt,
	}
}

// ToThreadResponse converts a MessageThread model to its API response DTO.
func ToThreadResponse(t *MessageThread) ThreadResponse {
	return ThreadResponse{
		ID:          t.ID,
		UUID:        t.UUID,
		WorkOrderID: t.WorkOrderID,
		Subject:     t.Subject,
		CreatedBy:   t.CreatedBy,
		IsClosed:    t.IsClosed,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// ToThreadMessageResponse converts a ThreadMessage model to its API response DTO.
func ToThreadMessageResponse(m *ThreadMessage) ThreadMessageResponse {
	return ThreadMessageResponse{
		ID:        m.ID,
		UUID:      m.UUID,
		ThreadID:  m.ThreadID,
		SenderID:  m.SenderID,
		Body:      m.Body,
		CreatedAt: m.CreatedAt,
	}
}
