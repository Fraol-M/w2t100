package notifications

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"propertyops/backend/internal/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLogger abstracts the audit logging dependency.
type AuditLogger interface {
	Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string)
}

// WorkOrderChecker abstracts the work-order lookups needed to enforce thread-creation scope.
// Implemented by *workorders.Repository; pass nil to skip the check (e.g. in unit tests).
type WorkOrderChecker interface {
	GetWorkOrderOwnerID(woID uint64) (uint64, error)
	GetWorkOrderAssignedTo(woID uint64) (*uint64, error)
	// GetWorkOrderPropertyID returns the property_id of the work order.
	GetWorkOrderPropertyID(woID uint64) (uint64, error)
	// IsManagedBy returns true if userID has an active PropertyManager assignment on propertyID.
	IsManagedBy(propertyID, userID uint64) (bool, error)
}

// RepositoryInterface abstracts the database operations for testing.
type RepositoryInterface interface {
	CreateNotification(n *Notification) error
	FindByID(id uint64) (*Notification, error)
	FindByRecipient(userID uint64, filters NotificationFilters, offset, limit int) ([]Notification, int64, error)
	CountUnread(userID uint64) (int64, error)
	UpdateNotificationStatus(id uint64, status string) error
	FindPendingNotifications(limit int) ([]Notification, error)
	IncrementRetry(id uint64) error
	CreateReceipt(receipt *NotificationReceipt) error
	FindReceipt(notificationID, userID uint64) (*NotificationReceipt, error)
	FindTemplateByName(name string) (*NotificationTemplate, error)
	FindUserIDsByRole(role string) ([]uint64, error)
	CreateThread(thread *MessageThread) error
	FindThreadByID(id uint64) (*MessageThread, error)
	ListThreadsByWorkOrder(workOrderID uint64) ([]MessageThread, error)
	ListThreadsByUser(userID uint64, offset, limit int) ([]MessageThread, int64, error)
	AddParticipant(participant *ThreadParticipant) error
	IsParticipant(threadID, userID uint64) (bool, error)
	ListParticipants(threadID uint64) ([]ThreadParticipant, error)
	CreateThreadMessage(msg *ThreadMessage) error
	ListThreadMessages(threadID uint64, offset, limit int) ([]ThreadMessage, int64, error)
	UpdateThreadTimestamp(threadID uint64) error
}

// Service handles notification and messaging business logic.
type Service struct {
	repo      RepositoryInterface
	audit     AuditLogger
	woChecker WorkOrderChecker // optional; nil skips thread WO-access enforcement
}

// NewService creates a new notification Service.
func NewService(repo RepositoryInterface, audit AuditLogger) *Service {
	return &Service{
		repo:  repo,
		audit: audit,
	}
}

// WithWorkOrderChecker wires up the work-order access checker used to enforce scope
// when creating threads linked to a specific work order.
func (s *Service) WithWorkOrderChecker(wc WorkOrderChecker) *Service {
	s.woChecker = wc
	return s
}

// --- Notification sending ---

// SendDirect creates a new notification with Pending status for direct delivery.
func (s *Service) SendDirect(recipientID uint64, subject, body, category string) (*Notification, *common.AppError) {
	if subject == "" {
		return nil, common.NewValidationError("subject is required")
	}
	if body == "" {
		return nil, common.NewValidationError("body is required")
	}
	if category == "" {
		category = "System"
	}

	n := &Notification{
		UUID:        uuid.New().String(),
		RecipientID: recipientID,
		Subject:     subject,
		Body:        body,
		Category:    category,
		Status:      common.NotificationStatusPending,
	}

	if err := s.repo.CreateNotification(n); err != nil {
		return nil, common.NewInternalError("failed to create notification")
	}

	s.audit.Log(0, common.AuditActionCreate, "Notification", n.ID,
		fmt.Sprintf("Notification sent to user %d: %s", recipientID, subject), "", "")

	return n, nil
}

// SendFromTemplate looks up a notification template by name, renders the subject and body
// using the provided data map, and creates a pending notification.
func (s *Service) SendFromTemplate(templateName string, recipientID uint64, data map[string]string) (*Notification, *common.AppError) {
	tmpl, err := s.repo.FindTemplateByName(templateName)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, common.NewNotFoundError("Notification template")
		}
		return nil, common.NewInternalError("failed to look up template")
	}

	subject, appErr := renderTemplate("subject", tmpl.SubjectTemplate, data)
	if appErr != nil {
		return nil, appErr
	}

	body, appErr := renderTemplate("body", tmpl.BodyTemplate, data)
	if appErr != nil {
		return nil, appErr
	}

	n := &Notification{
		UUID:        uuid.New().String(),
		RecipientID: recipientID,
		TemplateID:  &tmpl.ID,
		Subject:     subject,
		Body:        body,
		Category:    tmpl.Category,
		Status:      common.NotificationStatusPending,
	}

	if err := s.repo.CreateNotification(n); err != nil {
		return nil, common.NewInternalError("failed to create notification from template")
	}

	s.audit.Log(0, common.AuditActionCreate, "Notification", n.ID,
		fmt.Sprintf("Notification sent to user %d via template %q: %s", recipientID, templateName, subject), "", "")

	return n, nil
}

// SendEvent is a convenience method for event-triggered notifications.
// It wraps SendFromTemplate and satisfies the NotificationSender interface
// expected by the workorders package.
func (s *Service) SendEvent(eventName string, recipientID uint64, data map[string]string) error {
	_, appErr := s.SendFromTemplate(eventName, recipientID, data)
	if appErr != nil {
		return fmt.Errorf("%s", appErr.Message)
	}
	return nil
}

// SendEventToRole sends an event-triggered notification to all active users with the given role.
// Errors from individual sends are logged but do not abort the fan-out.
func (s *Service) SendEventToRole(eventName, role string, data map[string]string) error {
	ids, err := s.repo.FindUserIDsByRole(role)
	if err != nil {
		return fmt.Errorf("SendEventToRole: failed to resolve recipients for role %q: %w", role, err)
	}
	for _, id := range ids {
		if _, appErr := s.SendFromTemplate(eventName, id, data); appErr != nil {
			// Best-effort: log individual failures but continue fan-out.
			_ = appErr
		}
	}
	return nil
}

// ScheduleNotification creates a notification that will be sent at the specified time.
func (s *Service) ScheduleNotification(recipientID uint64, subject, body, category string, scheduledFor time.Time) (*Notification, *common.AppError) {
	if subject == "" {
		return nil, common.NewValidationError("subject is required")
	}
	if body == "" {
		return nil, common.NewValidationError("body is required")
	}
	if category == "" {
		category = "System"
	}

	n := &Notification{
		UUID:         uuid.New().String(),
		RecipientID:  recipientID,
		Subject:      subject,
		Body:         body,
		Category:     category,
		Status:       common.NotificationStatusPending,
		ScheduledFor: &scheduledFor,
	}

	if err := s.repo.CreateNotification(n); err != nil {
		return nil, common.NewInternalError("failed to schedule notification")
	}

	s.audit.Log(0, common.AuditActionCreate, "Notification", n.ID,
		fmt.Sprintf("Notification scheduled for user %d at %s: %s", recipientID, scheduledFor.Format(time.RFC3339), subject), "", "")

	return n, nil
}

// --- Reading and listing ---

// MarkRead creates a receipt for the given notification and user, marking it as read.
func (s *Service) MarkRead(notificationID, userID uint64) *common.AppError {
	// Verify the notification exists and belongs to the user.
	n, err := s.repo.FindByID(notificationID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return common.NewNotFoundError("Notification")
		}
		return common.NewInternalError("failed to find notification")
	}

	if n.RecipientID != userID {
		return common.NewForbiddenError("you can only mark your own notifications as read")
	}

	// Check if already read.
	existing, _ := s.repo.FindReceipt(notificationID, userID)
	if existing != nil {
		return nil // Already marked as read; idempotent.
	}

	now := time.Now().UTC()
	receipt := &NotificationReceipt{
		NotificationID: notificationID,
		UserID:         userID,
		ReadAt:         &now,
	}

	if err := s.repo.CreateReceipt(receipt); err != nil {
		return common.NewInternalError("failed to mark notification as read")
	}

	s.audit.Log(userID, common.AuditActionUpdate, "Notification", notificationID,
		"Notification marked as read", "", "")

	return nil
}

// GetUnreadCount returns the number of unread notifications for a user.
func (s *Service) GetUnreadCount(userID uint64) (int64, *common.AppError) {
	count, err := s.repo.CountUnread(userID)
	if err != nil {
		return 0, common.NewInternalError("failed to count unread notifications")
	}
	return count, nil
}

// GetByID retrieves a notification by ID, enforcing that it belongs to the requesting user.
func (s *Service) GetByID(notificationID, userID uint64) (*Notification, bool, *common.AppError) {
	n, err := s.repo.FindByID(notificationID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, common.NewNotFoundError("Notification")
		}
		return nil, false, common.NewInternalError("failed to find notification")
	}

	if n.RecipientID != userID {
		return nil, false, common.NewForbiddenError("you can only view your own notifications")
	}

	// Check if read.
	receipt, _ := s.repo.FindReceipt(notificationID, userID)
	isRead := receipt != nil

	return n, isRead, nil
}

// ListNotifications retrieves a paginated list of notifications for a user with filters.
func (s *Service) ListNotifications(userID uint64, filters NotificationFilters, page, perPage int) ([]Notification, []bool, int64, *common.AppError) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	offset := (page - 1) * perPage
	notifications, total, err := s.repo.FindByRecipient(userID, filters, offset, perPage)
	if err != nil {
		return nil, nil, 0, common.NewInternalError("failed to list notifications")
	}

	// Determine read status for each notification.
	readStatuses := make([]bool, len(notifications))
	for i, n := range notifications {
		receipt, _ := s.repo.FindReceipt(n.ID, userID)
		readStatuses[i] = receipt != nil
	}

	return notifications, readStatuses, total, nil
}

// --- Thread operations ---

// CreateThread creates a new message thread and adds the creator as a participant.
// roles must be the caller's roles so that work-order access can be enforced when
// req.WorkOrderID is non-nil and the service has a WorkOrderChecker configured.
// Privileged roles (PropertyManager, ComplianceReviewer, SystemAdmin) bypass the check.
// Technicians must be the assigned technician; Tenants must be the work-order creator.
func (s *Service) CreateThread(creatorID uint64, roles []string, req CreateThreadRequest) (*MessageThread, *common.AppError) {
	if req.Subject == "" {
		return nil, common.NewValidationError("subject is required")
	}

	// Enforce work-order access when a WorkOrderID is supplied.
	if req.WorkOrderID != nil && s.woChecker != nil {
		if appErr := s.checkWOAccess(*req.WorkOrderID, creatorID, roles); appErr != nil {
			return nil, appErr
		}
	}

	thread := &MessageThread{
		UUID:        uuid.New().String(),
		WorkOrderID: req.WorkOrderID,
		Subject:     req.Subject,
		CreatedBy:   creatorID,
	}

	if err := s.repo.CreateThread(thread); err != nil {
		return nil, common.NewInternalError("failed to create thread")
	}

	// Add creator as participant.
	participant := &ThreadParticipant{
		ThreadID: thread.ID,
		UserID:   creatorID,
		JoinedAt: time.Now().UTC(),
	}
	if err := s.repo.AddParticipant(participant); err != nil {
		return nil, common.NewInternalError("failed to add thread participant")
	}

	s.audit.Log(creatorID, common.AuditActionCreate, "MessageThread", thread.ID,
		fmt.Sprintf("Message thread created: %s", req.Subject), "", "")

	return thread, nil
}

// checkWOAccess verifies that the actor has permission to link to the given work order.
//   - SystemAdmin / ComplianceReviewer: unconditional access (global review scope).
//   - PropertyManager: must manage the property of the work order.
//   - Technician: must be the assigned technician.
//   - Tenant: must be the work-order creator.
func (s *Service) checkWOAccess(woID, actorID uint64, roles []string) *common.AppError {
	for _, r := range roles {
		if r == "SystemAdmin" || r == "ComplianceReviewer" {
			return nil
		}
	}

	for _, r := range roles {
		if r == "PropertyManager" {
			propertyID, err := s.woChecker.GetWorkOrderPropertyID(woID)
			if err != nil {
				return common.NewNotFoundError("Work order")
			}
			managed, err := s.woChecker.IsManagedBy(propertyID, actorID)
			if err != nil {
				return common.NewInternalError("failed to verify property access")
			}
			if !managed {
				return common.NewForbiddenError("you do not manage the property for this work order")
			}
			return nil
		}
	}

	for _, r := range roles {
		if r == "Technician" {
			assignedTo, err := s.woChecker.GetWorkOrderAssignedTo(woID)
			if err != nil {
				return common.NewNotFoundError("Work order")
			}
			if assignedTo == nil || *assignedTo != actorID {
				return common.NewForbiddenError("you can only create threads for work orders assigned to you")
			}
			return nil
		}
	}

	// Tenant: must be the creator.
	ownerID, err := s.woChecker.GetWorkOrderOwnerID(woID)
	if err != nil {
		return common.NewNotFoundError("Work order")
	}
	if ownerID != actorID {
		return common.NewForbiddenError("you do not have access to this work order")
	}
	return nil
}

// AddThreadParticipant adds a user to an existing thread. Only current participants may invite others.
// The thread creator can always add participants. Adding an existing participant is a no-op.
func (s *Service) AddThreadParticipant(threadID, actorID, targetUserID uint64) *common.AppError {
	// Verify the thread exists.
	thread, err := s.repo.FindThreadByID(threadID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return common.NewNotFoundError("Thread")
		}
		return common.NewInternalError("failed to find thread")
	}

	if thread.IsClosed {
		return common.NewValidationError("cannot add participants to a closed thread")
	}

	// Only current participants (including the creator) may add others.
	isParticipant, err := s.repo.IsParticipant(threadID, actorID)
	if err != nil {
		return common.NewInternalError("failed to check thread participation")
	}
	if !isParticipant {
		return common.NewForbiddenError("only thread participants can add new participants")
	}

	// No-op if target is already a participant.
	already, err := s.repo.IsParticipant(threadID, targetUserID)
	if err != nil {
		return common.NewInternalError("failed to check target participation")
	}
	if already {
		return nil // idempotent
	}

	p := &ThreadParticipant{
		ThreadID: threadID,
		UserID:   targetUserID,
		JoinedAt: time.Now().UTC(),
	}
	if err := s.repo.AddParticipant(p); err != nil {
		return common.NewInternalError("failed to add participant")
	}

	s.audit.Log(actorID, common.AuditActionUpdate, "MessageThread", threadID,
		fmt.Sprintf("User %d added to thread %d", targetUserID, threadID), "", "")

	return nil
}

// GetParticipants returns the list of participants in a thread. Any participant can view.
func (s *Service) GetParticipants(threadID, actorID uint64) ([]ThreadParticipant, *common.AppError) {
	isParticipant, err := s.repo.IsParticipant(threadID, actorID)
	if err != nil {
		return nil, common.NewInternalError("failed to check thread participation")
	}
	if !isParticipant {
		return nil, common.NewForbiddenError("only thread participants can view participants")
	}

	participants, err := s.repo.ListParticipants(threadID)
	if err != nil {
		return nil, common.NewInternalError("failed to list participants")
	}
	return participants, nil
}

// AddMessage adds a message to an existing thread. Only participants can post messages.
// The sender is automatically added as a participant if not already one.
func (s *Service) AddMessage(threadID, senderID uint64, req AddMessageRequest) (*ThreadMessage, *common.AppError) {
	if req.Body == "" {
		return nil, common.NewValidationError("body is required")
	}

	// Verify thread exists.
	thread, err := s.repo.FindThreadByID(threadID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, common.NewNotFoundError("Thread")
		}
		return nil, common.NewInternalError("failed to find thread")
	}

	if thread.IsClosed {
		return nil, common.NewValidationError("thread is closed")
	}

	// Check participant access.
	isParticipant, err := s.repo.IsParticipant(threadID, senderID)
	if err != nil {
		return nil, common.NewInternalError("failed to check thread participation")
	}
	if !isParticipant {
		return nil, common.NewForbiddenError("only thread participants can post messages")
	}

	msg := &ThreadMessage{
		UUID:     uuid.New().String(),
		ThreadID: threadID,
		SenderID: senderID,
		Body:     req.Body,
	}

	if err := s.repo.CreateThreadMessage(msg); err != nil {
		return nil, common.NewInternalError("failed to create message")
	}

	// Update thread timestamp.
	_ = s.repo.UpdateThreadTimestamp(threadID)

	s.audit.Log(senderID, common.AuditActionCreate, "ThreadMessage", msg.ID,
		fmt.Sprintf("Message sent in thread %d", threadID), "", "")

	return msg, nil
}

// GetThread retrieves a thread by ID. Only participants can view threads.
func (s *Service) GetThread(threadID, userID uint64) (*MessageThread, *common.AppError) {
	thread, err := s.repo.FindThreadByID(threadID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, common.NewNotFoundError("Thread")
		}
		return nil, common.NewInternalError("failed to find thread")
	}

	isParticipant, err := s.repo.IsParticipant(threadID, userID)
	if err != nil {
		return nil, common.NewInternalError("failed to check thread participation")
	}
	if !isParticipant {
		return nil, common.NewForbiddenError("only thread participants can view this thread")
	}

	return thread, nil
}

// ListThreads retrieves a paginated list of threads the user participates in.
func (s *Service) ListThreads(userID uint64, page, perPage int) ([]MessageThread, int64, *common.AppError) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	offset := (page - 1) * perPage
	threads, total, err := s.repo.ListThreadsByUser(userID, offset, perPage)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list threads")
	}

	return threads, total, nil
}

// ListThreadMessages retrieves a paginated list of messages in a thread.
// Only participants can view messages.
func (s *Service) ListThreadMessages(threadID, userID uint64, page, perPage int) ([]ThreadMessage, int64, *common.AppError) {
	// Verify participation.
	isParticipant, err := s.repo.IsParticipant(threadID, userID)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to check thread participation")
	}
	if !isParticipant {
		return nil, 0, common.NewForbiddenError("only thread participants can view messages")
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	offset := (page - 1) * perPage
	messages, total, err := s.repo.ListThreadMessages(threadID, offset, perPage)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list messages")
	}

	return messages, total, nil
}

// --- Template rendering helper ---

// renderTemplate parses and executes a Go text/template string with the given data.
func renderTemplate(name, tmplStr string, data map[string]string) (string, *common.AppError) {
	t, err := template.New(name).Parse(tmplStr)
	if err != nil {
		return "", common.NewInternalError(fmt.Sprintf("failed to parse %s template: %v", name, err))
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", common.NewInternalError(fmt.Sprintf("failed to render %s template: %v", name, err))
	}

	return buf.String(), nil
}
