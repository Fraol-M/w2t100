package notifications

import (
	"fmt"
	"testing"
	"time"

	"propertyops/backend/internal/common"

	"gorm.io/gorm"
)

// --- Mock types ---

// mockAuditLogger is a no-op audit logger for testing.
type mockAuditLogger struct{}

func (m *mockAuditLogger) Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string) {
}

// mockRepo implements RepositoryInterface for testing.
type mockRepo struct {
	notifications  map[uint64]*Notification
	receipts       map[string]*NotificationReceipt // key: "notifID-userID"
	templates      map[string]*NotificationTemplate
	threads        map[uint64]*MessageThread
	participants   map[string]bool // key: "threadID-userID"
	messages       map[uint64][]*ThreadMessage
	nextNotifID    uint64
	nextReceiptID  uint64
	nextThreadID   uint64
	nextMsgID      uint64

	// For error injection.
	createNotifErr    error
	findByIDErr       error
	createReceiptErr  error
	createThreadErr   error
	createMsgErr      error
	addParticipantErr error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		notifications: make(map[uint64]*Notification),
		receipts:      make(map[string]*NotificationReceipt),
		templates:     make(map[string]*NotificationTemplate),
		threads:       make(map[uint64]*MessageThread),
		participants:  make(map[string]bool),
		messages:      make(map[uint64][]*ThreadMessage),
		nextNotifID:   1,
		nextReceiptID: 1,
		nextThreadID:  1,
		nextMsgID:     1,
	}
}

func receiptKey(notifID, userID uint64) string {
	return fmt.Sprintf("%d-%d", notifID, userID)
}

func participantKey(threadID, userID uint64) string {
	return fmt.Sprintf("%d-%d", threadID, userID)
}

func (m *mockRepo) CreateNotification(n *Notification) error {
	if m.createNotifErr != nil {
		return m.createNotifErr
	}
	n.ID = m.nextNotifID
	m.nextNotifID++
	n.CreatedAt = time.Now().UTC()
	n.UpdatedAt = n.CreatedAt
	m.notifications[n.ID] = n
	return nil
}

func (m *mockRepo) FindByID(id uint64) (*Notification, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	n, ok := m.notifications[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return n, nil
}

func (m *mockRepo) FindByRecipient(userID uint64, filters NotificationFilters, offset, limit int) ([]Notification, int64, error) {
	var result []Notification
	for _, n := range m.notifications {
		if n.RecipientID != userID {
			continue
		}
		if filters.Status != "" && n.Status != filters.Status {
			continue
		}
		if filters.Category != "" && n.Category != filters.Category {
			continue
		}
		if filters.ReadFlag == "true" {
			if _, ok := m.receipts[receiptKey(n.ID, userID)]; !ok {
				continue
			}
		} else if filters.ReadFlag == "false" {
			if _, ok := m.receipts[receiptKey(n.ID, userID)]; ok {
				continue
			}
		}
		result = append(result, *n)
	}
	total := int64(len(result))
	if offset >= len(result) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockRepo) CountUnread(userID uint64) (int64, error) {
	var count int64
	for _, n := range m.notifications {
		if n.RecipientID == userID {
			if _, ok := m.receipts[receiptKey(n.ID, userID)]; !ok {
				count++
			}
		}
	}
	return count, nil
}

func (m *mockRepo) UpdateNotificationStatus(id uint64, status string) error {
	n, ok := m.notifications[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	n.Status = status
	if status == "Sent" {
		now := time.Now().UTC()
		n.SentAt = &now
	}
	return nil
}

func (m *mockRepo) FindPendingNotifications(limit int) ([]Notification, error) {
	var result []Notification
	now := time.Now().UTC()
	for _, n := range m.notifications {
		if n.Status == "Pending" && (n.ScheduledFor == nil || !n.ScheduledFor.After(now)) {
			result = append(result, *n)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *mockRepo) IncrementRetry(id uint64) error {
	n, ok := m.notifications[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	n.RetryCount++
	now := time.Now().UTC()
	n.LastRetryAt = &now
	return nil
}

func (m *mockRepo) CreateReceipt(receipt *NotificationReceipt) error {
	if m.createReceiptErr != nil {
		return m.createReceiptErr
	}
	receipt.ID = m.nextReceiptID
	m.nextReceiptID++
	receipt.CreatedAt = time.Now().UTC()
	m.receipts[receiptKey(receipt.NotificationID, receipt.UserID)] = receipt
	return nil
}

func (m *mockRepo) FindReceipt(notificationID, userID uint64) (*NotificationReceipt, error) {
	r, ok := m.receipts[receiptKey(notificationID, userID)]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return r, nil
}

func (m *mockRepo) FindTemplateByName(name string) (*NotificationTemplate, error) {
	t, ok := m.templates[name]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	if !t.IsActive {
		return nil, gorm.ErrRecordNotFound
	}
	return t, nil
}

func (m *mockRepo) FindUserIDsByRole(role string) ([]uint64, error) {
	return nil, nil // not exercised in unit tests
}

func (m *mockRepo) CreateThread(thread *MessageThread) error {
	if m.createThreadErr != nil {
		return m.createThreadErr
	}
	thread.ID = m.nextThreadID
	m.nextThreadID++
	thread.CreatedAt = time.Now().UTC()
	thread.UpdatedAt = thread.CreatedAt
	m.threads[thread.ID] = thread
	return nil
}

func (m *mockRepo) FindThreadByID(id uint64) (*MessageThread, error) {
	t, ok := m.threads[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return t, nil
}

func (m *mockRepo) ListThreadsByWorkOrder(workOrderID uint64) ([]MessageThread, error) {
	var result []MessageThread
	for _, t := range m.threads {
		if t.WorkOrderID != nil && *t.WorkOrderID == workOrderID {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *mockRepo) ListThreadsByUser(userID uint64, offset, limit int) ([]MessageThread, int64, error) {
	var result []MessageThread
	for _, t := range m.threads {
		if m.participants[participantKey(t.ID, userID)] {
			result = append(result, *t)
		}
	}
	total := int64(len(result))
	if offset >= len(result) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockRepo) AddParticipant(participant *ThreadParticipant) error {
	if m.addParticipantErr != nil {
		return m.addParticipantErr
	}
	m.participants[participantKey(participant.ThreadID, participant.UserID)] = true
	return nil
}

func (m *mockRepo) IsParticipant(threadID, userID uint64) (bool, error) {
	return m.participants[participantKey(threadID, userID)], nil
}

func (m *mockRepo) ListParticipants(threadID uint64) ([]ThreadParticipant, error) {
	var result []ThreadParticipant
	for key := range m.participants {
		var tID, uID uint64
		fmt.Sscanf(key, "%d-%d", &tID, &uID)
		if tID == threadID {
			result = append(result, ThreadParticipant{ThreadID: tID, UserID: uID})
		}
	}
	return result, nil
}

func (m *mockRepo) CreateThreadMessage(msg *ThreadMessage) error {
	if m.createMsgErr != nil {
		return m.createMsgErr
	}
	msg.ID = m.nextMsgID
	m.nextMsgID++
	msg.CreatedAt = time.Now().UTC()
	m.messages[msg.ThreadID] = append(m.messages[msg.ThreadID], msg)
	return nil
}

func (m *mockRepo) ListThreadMessages(threadID uint64, offset, limit int) ([]ThreadMessage, int64, error) {
	msgs := m.messages[threadID]
	total := int64(len(msgs))
	if offset >= len(msgs) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(msgs) {
		end = len(msgs)
	}
	result := make([]ThreadMessage, end-offset)
	for i := offset; i < end; i++ {
		result[i-offset] = *msgs[i]
	}
	return result, total, nil
}

func (m *mockRepo) UpdateThreadTimestamp(threadID uint64) error {
	t, ok := m.threads[threadID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// --- Helper to seed a template ---

func (m *mockRepo) seedTemplate(name, subjectTmpl, bodyTmpl, category string, active bool) {
	m.templates[name] = &NotificationTemplate{
		ID:              uint64(len(m.templates) + 1),
		Name:            name,
		SubjectTemplate: subjectTmpl,
		BodyTemplate:    bodyTmpl,
		Category:        category,
		IsActive:        active,
	}
}

// --- Helper to create service with mock ---

func setupService(t *testing.T) (*Service, *mockRepo) {
	t.Helper()
	repo := newMockRepo()
	svc := NewService(repo, &mockAuditLogger{})
	return svc, repo
}

// --- Template rendering tests ---

func TestRenderTemplate_ValidTemplate(t *testing.T) {
	result, appErr := renderTemplate("test", "Hello {{.Name}}, your order #{{.OrderID}} is ready.", map[string]string{
		"Name":    "Alice",
		"OrderID": "42",
	})
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	expected := "Hello Alice, your order #42 is ready."
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestRenderTemplate_MissingVariable(t *testing.T) {
	result, appErr := renderTemplate("test", "Hello {{.Name}}", map[string]string{})
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	expected := "Hello <no value>"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestRenderTemplate_InvalidSyntax(t *testing.T) {
	_, appErr := renderTemplate("test", "Hello {{.Name", map[string]string{})
	if appErr == nil {
		t.Fatal("expected error for invalid template syntax")
	}
}

func TestRenderTemplate_MultipleVariables(t *testing.T) {
	tmpl := "Work order #{{.WorkOrderID}} ({{.Priority}}) has been assigned to you."
	result, appErr := renderTemplate("test", tmpl, map[string]string{
		"WorkOrderID": "100",
		"Priority":    "High",
	})
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	expected := "Work order #100 (High) has been assigned to you."
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// --- SendDirect tests ---

func TestSendDirect_CreatesPendingNotification(t *testing.T) {
	svc, _ := setupService(t)

	n, appErr := svc.SendDirect(1, "Test Subject", "Test Body", "Alert")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if n.Status != common.NotificationStatusPending {
		t.Errorf("expected status %s, got %s", common.NotificationStatusPending, n.Status)
	}
	if n.RecipientID != 1 {
		t.Errorf("expected recipient ID 1, got %d", n.RecipientID)
	}
	if n.Subject != "Test Subject" {
		t.Errorf("expected subject %q, got %q", "Test Subject", n.Subject)
	}
	if n.Body != "Test Body" {
		t.Errorf("expected body %q, got %q", "Test Body", n.Body)
	}
	if n.Category != "Alert" {
		t.Errorf("expected category %q, got %q", "Alert", n.Category)
	}
	if n.UUID == "" {
		t.Error("expected UUID to be generated")
	}
}

func TestSendDirect_DefaultCategory(t *testing.T) {
	svc, _ := setupService(t)

	n, appErr := svc.SendDirect(1, "Test", "Body", "")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if n.Category != "System" {
		t.Errorf("expected default category 'System', got %q", n.Category)
	}
}

func TestSendDirect_EmptySubject(t *testing.T) {
	svc, _ := setupService(t)

	_, appErr := svc.SendDirect(1, "", "Body", "Alert")
	if appErr == nil {
		t.Fatal("expected validation error for empty subject")
	}
}

func TestSendDirect_EmptyBody(t *testing.T) {
	svc, _ := setupService(t)

	_, appErr := svc.SendDirect(1, "Subject", "", "Alert")
	if appErr == nil {
		t.Fatal("expected validation error for empty body")
	}
}

// --- SendFromTemplate tests ---

func TestSendFromTemplate_ValidTemplate(t *testing.T) {
	svc, repo := setupService(t)

	repo.seedTemplate("work_order_assigned",
		"Work Order #{{.WorkOrderID}} Assigned",
		"You have been assigned work order #{{.WorkOrderID}} with priority {{.Priority}}.",
		"WorkOrder", true)

	n, appErr := svc.SendFromTemplate("work_order_assigned", 42, map[string]string{
		"WorkOrderID": "100",
		"Priority":    "High",
	})
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if n.Subject != "Work Order #100 Assigned" {
		t.Errorf("expected rendered subject, got %q", n.Subject)
	}
	expectedBody := "You have been assigned work order #100 with priority High."
	if n.Body != expectedBody {
		t.Errorf("expected rendered body %q, got %q", expectedBody, n.Body)
	}
	if n.TemplateID == nil {
		t.Error("expected template ID to be set")
	}
	if n.Category != "WorkOrder" {
		t.Errorf("expected category from template, got %q", n.Category)
	}
	if n.Status != common.NotificationStatusPending {
		t.Errorf("expected Pending status, got %q", n.Status)
	}
}

func TestSendFromTemplate_TemplateNotFound(t *testing.T) {
	svc, _ := setupService(t)

	_, appErr := svc.SendFromTemplate("nonexistent_template", 1, map[string]string{})
	if appErr == nil {
		t.Fatal("expected error for nonexistent template")
	}
	if appErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND error code, got %q", appErr.Code)
	}
}

func TestSendFromTemplate_InactiveTemplate(t *testing.T) {
	svc, repo := setupService(t)

	repo.seedTemplate("inactive_template", "Test", "Test", "System", false)

	_, appErr := svc.SendFromTemplate("inactive_template", 1, map[string]string{})
	if appErr == nil {
		t.Fatal("expected error for inactive template")
	}
}

// --- SendEvent tests ---

func TestSendEvent_Success(t *testing.T) {
	svc, repo := setupService(t)

	repo.seedTemplate("work_order_assigned",
		"Assigned: WO #{{.work_order_id}}",
		"Priority: {{.priority}}",
		"WorkOrder", true)

	err := svc.SendEvent("work_order_assigned", 10, map[string]string{
		"work_order_id": "55",
		"priority":      "Emergency",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify notification was created in mock repo.
	found := false
	for _, n := range repo.notifications {
		if n.RecipientID == 10 && n.Subject == "Assigned: WO #55" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected notification to be created for recipient 10")
	}
}

func TestSendEvent_TemplateNotFound(t *testing.T) {
	svc, _ := setupService(t)

	err := svc.SendEvent("no_such_event", 1, map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing event template")
	}
}

// --- MarkRead tests ---

func TestMarkRead_CreatesReceipt(t *testing.T) {
	svc, repo := setupService(t)

	n, _ := svc.SendDirect(1, "Test", "Body", "System")

	appErr := svc.MarkRead(n.ID, 1)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	receipt, ok := repo.receipts[receiptKey(n.ID, 1)]
	if !ok {
		t.Fatal("receipt not found after marking as read")
	}
	if receipt.ReadAt == nil {
		t.Error("expected ReadAt to be set")
	}
}

func TestMarkRead_Idempotent(t *testing.T) {
	svc, _ := setupService(t)

	n, _ := svc.SendDirect(1, "Test", "Body", "System")

	appErr := svc.MarkRead(n.ID, 1)
	if appErr != nil {
		t.Fatalf("first mark read failed: %v", appErr)
	}

	appErr = svc.MarkRead(n.ID, 1)
	if appErr != nil {
		t.Fatalf("second mark read should be idempotent, got: %v", appErr)
	}
}

func TestMarkRead_WrongUser(t *testing.T) {
	svc, _ := setupService(t)

	n, _ := svc.SendDirect(1, "Test", "Body", "System")

	appErr := svc.MarkRead(n.ID, 2)
	if appErr == nil {
		t.Fatal("expected forbidden error for wrong user")
	}
	if appErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN error code, got %q", appErr.Code)
	}
}

func TestMarkRead_NotFound(t *testing.T) {
	svc, _ := setupService(t)

	appErr := svc.MarkRead(9999, 1)
	if appErr == nil {
		t.Fatal("expected not found error")
	}
	if appErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND error code, got %q", appErr.Code)
	}
}

// --- Unread count tests ---

func TestGetUnreadCount(t *testing.T) {
	svc, _ := setupService(t)

	// Create 3 notifications for user 1.
	for i := 0; i < 3; i++ {
		_, appErr := svc.SendDirect(1, fmt.Sprintf("Notif %d", i), "Body", "System")
		if appErr != nil {
			t.Fatalf("failed to create notification: %v", appErr)
		}
	}

	count, appErr := svc.GetUnreadCount(1)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if count != 3 {
		t.Errorf("expected 3 unread, got %d", count)
	}

	// Mark one as read.
	_ = svc.MarkRead(1, 1)

	count, appErr = svc.GetUnreadCount(1)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if count != 2 {
		t.Errorf("expected 2 unread, got %d", count)
	}
}

func TestGetUnreadCount_NoNotifications(t *testing.T) {
	svc, _ := setupService(t)

	count, appErr := svc.GetUnreadCount(999)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if count != 0 {
		t.Errorf("expected 0 unread for user with no notifications, got %d", count)
	}
}

// --- Thread tests ---

func TestCreateThread(t *testing.T) {
	svc, _ := setupService(t)

	thread, appErr := svc.CreateThread(1, nil, CreateThreadRequest{
		Subject: "Discussion about unit repair",
	})
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if thread.UUID == "" {
		t.Error("expected UUID to be generated")
	}
	if thread.Subject != "Discussion about unit repair" {
		t.Errorf("expected subject, got %q", thread.Subject)
	}
	if thread.CreatedBy != 1 {
		t.Errorf("expected creator ID 1, got %d", thread.CreatedBy)
	}
}

func TestCreateThread_WithWorkOrder(t *testing.T) {
	svc, _ := setupService(t)

	woID := uint64(42)
	thread, appErr := svc.CreateThread(1, nil, CreateThreadRequest{
		WorkOrderID: &woID,
		Subject:     "WO Discussion",
	})
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if thread.WorkOrderID == nil || *thread.WorkOrderID != 42 {
		t.Errorf("expected work order ID 42, got %v", thread.WorkOrderID)
	}
}

func TestCreateThread_EmptySubject(t *testing.T) {
	svc, _ := setupService(t)

	_, appErr := svc.CreateThread(1, nil, CreateThreadRequest{Subject: ""})
	if appErr == nil {
		t.Fatal("expected validation error for empty subject")
	}
}

func TestAddMessage(t *testing.T) {
	svc, _ := setupService(t)

	thread, _ := svc.CreateThread(1, nil, CreateThreadRequest{Subject: "Test Thread"})

	msg, appErr := svc.AddMessage(thread.ID, 1, AddMessageRequest{Body: "Hello, world!"})
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if msg.UUID == "" {
		t.Error("expected UUID to be generated")
	}
	if msg.Body != "Hello, world!" {
		t.Errorf("expected body %q, got %q", "Hello, world!", msg.Body)
	}
	if msg.SenderID != 1 {
		t.Errorf("expected sender ID 1, got %d", msg.SenderID)
	}
	if msg.ThreadID != thread.ID {
		t.Errorf("expected thread ID %d, got %d", thread.ID, msg.ThreadID)
	}
}

func TestAddMessage_EmptyBody(t *testing.T) {
	svc, _ := setupService(t)

	thread, _ := svc.CreateThread(1, nil, CreateThreadRequest{Subject: "Test Thread"})

	_, appErr := svc.AddMessage(thread.ID, 1, AddMessageRequest{Body: ""})
	if appErr == nil {
		t.Fatal("expected validation error for empty body")
	}
}

func TestAddMessage_NonParticipant(t *testing.T) {
	svc, _ := setupService(t)

	thread, _ := svc.CreateThread(1, nil, CreateThreadRequest{Subject: "Private Thread"})

	_, appErr := svc.AddMessage(thread.ID, 2, AddMessageRequest{Body: "Intruder!"})
	if appErr == nil {
		t.Fatal("expected forbidden error for non-participant")
	}
	if appErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN error code, got %q", appErr.Code)
	}
}

func TestAddMessage_ThreadNotFound(t *testing.T) {
	svc, _ := setupService(t)

	_, appErr := svc.AddMessage(9999, 1, AddMessageRequest{Body: "Hello"})
	if appErr == nil {
		t.Fatal("expected not found error")
	}
	if appErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND error code, got %q", appErr.Code)
	}
}

func TestAddMessage_ClosedThread(t *testing.T) {
	svc, repo := setupService(t)

	thread, _ := svc.CreateThread(1, nil, CreateThreadRequest{Subject: "Will be closed"})

	// Close the thread directly in the mock.
	repo.threads[thread.ID].IsClosed = true

	_, appErr := svc.AddMessage(thread.ID, 1, AddMessageRequest{Body: "Too late"})
	if appErr == nil {
		t.Fatal("expected error for closed thread")
	}
}

func TestGetThread_Participant(t *testing.T) {
	svc, _ := setupService(t)

	thread, _ := svc.CreateThread(1, nil, CreateThreadRequest{Subject: "Test"})

	got, appErr := svc.GetThread(thread.ID, 1)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if got.ID != thread.ID {
		t.Errorf("expected thread ID %d, got %d", thread.ID, got.ID)
	}
}

func TestGetThread_NonParticipant(t *testing.T) {
	svc, _ := setupService(t)

	thread, _ := svc.CreateThread(1, nil, CreateThreadRequest{Subject: "Private"})

	_, appErr := svc.GetThread(thread.ID, 2)
	if appErr == nil {
		t.Fatal("expected forbidden error for non-participant")
	}
	if appErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN error code, got %q", appErr.Code)
	}
}

func TestGetThread_NotFound(t *testing.T) {
	svc, _ := setupService(t)

	_, appErr := svc.GetThread(9999, 1)
	if appErr == nil {
		t.Fatal("expected not found error")
	}
}

func TestListThreadMessages(t *testing.T) {
	svc, _ := setupService(t)

	thread, _ := svc.CreateThread(1, nil, CreateThreadRequest{Subject: "Chat"})

	for i := 0; i < 5; i++ {
		_, appErr := svc.AddMessage(thread.ID, 1, AddMessageRequest{Body: fmt.Sprintf("Message %d", i)})
		if appErr != nil {
			t.Fatalf("failed to add message: %v", appErr)
		}
	}

	messages, total, appErr := svc.ListThreadMessages(thread.ID, 1, 1, 20)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if total != 5 {
		t.Errorf("expected 5 messages, got %d", total)
	}
	if len(messages) != 5 {
		t.Errorf("expected 5 messages returned, got %d", len(messages))
	}
}

func TestListThreadMessages_NonParticipant(t *testing.T) {
	svc, _ := setupService(t)

	thread, _ := svc.CreateThread(1, nil, CreateThreadRequest{Subject: "Private"})

	_, _, appErr := svc.ListThreadMessages(thread.ID, 2, 1, 20)
	if appErr == nil {
		t.Fatal("expected forbidden error for non-participant")
	}
}

// --- Object-level auth: user can only read own notifications ---

func TestGetByID_OwnNotification(t *testing.T) {
	svc, _ := setupService(t)

	n, _ := svc.SendDirect(1, "Personal", "This is for user 1", "System")

	got, isRead, appErr := svc.GetByID(n.ID, 1)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if got.ID != n.ID {
		t.Errorf("expected notification ID %d, got %d", n.ID, got.ID)
	}
	if isRead {
		t.Error("expected notification to be unread")
	}
}

func TestGetByID_OtherUserNotification(t *testing.T) {
	svc, _ := setupService(t)

	n, _ := svc.SendDirect(1, "Personal", "This is for user 1", "System")

	_, _, appErr := svc.GetByID(n.ID, 2)
	if appErr == nil {
		t.Fatal("expected forbidden error when accessing another user's notification")
	}
	if appErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN error code, got %q", appErr.Code)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	svc, _ := setupService(t)

	_, _, appErr := svc.GetByID(9999, 1)
	if appErr == nil {
		t.Fatal("expected not found error")
	}
	if appErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND error code, got %q", appErr.Code)
	}
}

// --- Schedule notification tests ---

func TestScheduleNotification(t *testing.T) {
	svc, _ := setupService(t)

	futureTime := time.Now().UTC().Add(24 * time.Hour)
	n, appErr := svc.ScheduleNotification(1, "Scheduled Subject", "Scheduled Body", "Reminder", futureTime)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}

	if n.ScheduledFor == nil {
		t.Fatal("expected scheduled_for to be set")
	}
	if n.Status != common.NotificationStatusPending {
		t.Errorf("expected Pending status, got %s", n.Status)
	}
}

// --- ListNotifications tests ---

func TestListNotifications_Paginated(t *testing.T) {
	svc, _ := setupService(t)

	for i := 0; i < 15; i++ {
		_, appErr := svc.SendDirect(1, fmt.Sprintf("Notification %d", i), "Body", "System")
		if appErr != nil {
			t.Fatalf("failed to create notification: %v", appErr)
		}
	}

	notifications, _, total, appErr := svc.ListNotifications(1, NotificationFilters{}, 1, 10)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if total != 15 {
		t.Errorf("expected total 15, got %d", total)
	}
	if len(notifications) != 10 {
		t.Errorf("expected 10 results on page 1, got %d", len(notifications))
	}

	notifications, _, total, appErr = svc.ListNotifications(1, NotificationFilters{}, 2, 10)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if len(notifications) != 5 {
		t.Errorf("expected 5 results on page 2, got %d", len(notifications))
	}
}

func TestListNotifications_FilterByCategory(t *testing.T) {
	svc, _ := setupService(t)

	categories := []string{"Alert", "System", "Alert", "WorkOrder", "Alert"}
	for i, cat := range categories {
		_, appErr := svc.SendDirect(1, fmt.Sprintf("Test %d", i), "Body", cat)
		if appErr != nil {
			t.Fatalf("failed to create notification: %v", appErr)
		}
	}

	notifications, _, total, appErr := svc.ListNotifications(1, NotificationFilters{Category: "Alert"}, 1, 20)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if total != 3 {
		t.Errorf("expected 3 Alert notifications, got %d", total)
	}
	if len(notifications) != 3 {
		t.Errorf("expected 3 results, got %d", len(notifications))
	}
}
