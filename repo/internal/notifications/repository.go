package notifications

import (
	"time"

	"gorm.io/gorm"
)

// Repository handles all notification and thread database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new notification Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// --- Notification operations ---

// CreateNotification inserts a new notification record.
func (r *Repository) CreateNotification(n *Notification) error {
	return r.db.Create(n).Error
}

// FindByID loads a notification by its primary key.
func (r *Repository) FindByID(id uint64) (*Notification, error) {
	var n Notification
	err := r.db.Where("id = ?", id).First(&n).Error
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// NotificationFilters holds optional filters for listing notifications.
type NotificationFilters struct {
	Status   string
	Category string
	ReadFlag string // "true", "false", or "" (all)
	UserID   uint64
}

// FindByRecipient retrieves notifications for a user with filtering and pagination.
func (r *Repository) FindByRecipient(userID uint64, filters NotificationFilters, offset, limit int) ([]Notification, int64, error) {
	query := r.db.Model(&Notification{}).Where("recipient_id = ?", userID)

	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.Category != "" {
		query = query.Where("category = ?", filters.Category)
	}
	if filters.ReadFlag == "true" {
		query = query.Where("id IN (SELECT notification_id FROM notification_receipts WHERE user_id = ?)", userID)
	} else if filters.ReadFlag == "false" {
		query = query.Where("id NOT IN (SELECT notification_id FROM notification_receipts WHERE user_id = ?)", userID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var notifications []Notification
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&notifications).Error
	if err != nil {
		return nil, 0, err
	}

	return notifications, total, nil
}

// CountUnread returns the number of unread notifications for a user.
func (r *Repository) CountUnread(userID uint64) (int64, error) {
	var count int64
	err := r.db.Model(&Notification{}).
		Where("recipient_id = ?", userID).
		Where("id NOT IN (SELECT notification_id FROM notification_receipts WHERE user_id = ?)", userID).
		Count(&count).Error
	return count, err
}

// UpdateNotificationStatus updates the status of a notification and optionally sets SentAt.
func (r *Repository) UpdateNotificationStatus(id uint64, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if status == "Sent" {
		now := time.Now().UTC()
		updates["sent_at"] = &now
	}
	return r.db.Model(&Notification{}).Where("id = ?", id).Updates(updates).Error
}

// FindPendingNotifications retrieves notifications that are pending and due to be sent.
func (r *Repository) FindPendingNotifications(limit int) ([]Notification, error) {
	var notifications []Notification
	now := time.Now().UTC()
	err := r.db.Where("status = ? AND (scheduled_for IS NULL OR scheduled_for <= ?)", "Pending", now).
		Order("created_at ASC").
		Limit(limit).
		Find(&notifications).Error
	return notifications, err
}

// IncrementRetry increments the retry count and sets the last retry timestamp.
func (r *Repository) IncrementRetry(id uint64) error {
	now := time.Now().UTC()
	return r.db.Model(&Notification{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"retry_count":  gorm.Expr("retry_count + 1"),
			"last_retry_at": &now,
		}).Error
}

// FindUserIDsByRole returns IDs of all active users with the given role name.
func (r *Repository) FindUserIDsByRole(role string) ([]uint64, error) {
	var ids []uint64
	err := r.db.Raw(`SELECT u.id FROM users u
		JOIN user_roles ur ON ur.user_id = u.id
		JOIN roles ro ON ro.id = ur.role_id
		WHERE ro.name = ? AND u.is_active = ?`, role, true).Scan(&ids).Error
	return ids, err
}

// --- Receipt operations ---

// CreateReceipt inserts a new notification receipt.
func (r *Repository) CreateReceipt(receipt *NotificationReceipt) error {
	return r.db.Create(receipt).Error
}

// FindReceipt looks up a receipt for a specific notification and user.
func (r *Repository) FindReceipt(notificationID, userID uint64) (*NotificationReceipt, error) {
	var receipt NotificationReceipt
	err := r.db.Where("notification_id = ? AND user_id = ?", notificationID, userID).First(&receipt).Error
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

// --- Template operations ---

// FindTemplateByName loads a notification template by its unique name.
func (r *Repository) FindTemplateByName(name string) (*NotificationTemplate, error) {
	var tmpl NotificationTemplate
	err := r.db.Where("name = ? AND is_active = ?", name, true).First(&tmpl).Error
	if err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// --- Thread operations ---

// CreateThread inserts a new message thread.
func (r *Repository) CreateThread(thread *MessageThread) error {
	return r.db.Create(thread).Error
}

// FindThreadByID loads a message thread by its primary key.
func (r *Repository) FindThreadByID(id uint64) (*MessageThread, error) {
	var thread MessageThread
	err := r.db.Where("id = ?", id).First(&thread).Error
	if err != nil {
		return nil, err
	}
	return &thread, nil
}

// ListThreadsByWorkOrder retrieves all threads linked to a specific work order.
func (r *Repository) ListThreadsByWorkOrder(workOrderID uint64) ([]MessageThread, error) {
	var threads []MessageThread
	err := r.db.Where("work_order_id = ?", workOrderID).
		Order("created_at DESC").
		Find(&threads).Error
	return threads, err
}

// ListThreadsByUser retrieves all threads that a user participates in, with pagination.
func (r *Repository) ListThreadsByUser(userID uint64, offset, limit int) ([]MessageThread, int64, error) {
	query := r.db.Model(&MessageThread{}).
		Where("id IN (SELECT thread_id FROM thread_participants WHERE user_id = ?)", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var threads []MessageThread
	err := query.Order("updated_at DESC").Offset(offset).Limit(limit).Find(&threads).Error
	if err != nil {
		return nil, 0, err
	}

	return threads, total, nil
}

// --- Thread participant operations ---

// AddParticipant adds a user as a participant in a thread.
func (r *Repository) AddParticipant(participant *ThreadParticipant) error {
	return r.db.Create(participant).Error
}

// IsParticipant checks whether a user is a participant in a thread.
func (r *Repository) IsParticipant(threadID, userID uint64) (bool, error) {
	var count int64
	err := r.db.Model(&ThreadParticipant{}).
		Where("thread_id = ? AND user_id = ?", threadID, userID).
		Count(&count).Error
	return count > 0, err
}

// ListParticipants retrieves all participants for a thread.
func (r *Repository) ListParticipants(threadID uint64) ([]ThreadParticipant, error) {
	var participants []ThreadParticipant
	err := r.db.Where("thread_id = ?", threadID).Order("joined_at ASC").Find(&participants).Error
	return participants, err
}

// --- Thread message operations ---

// CreateThreadMessage inserts a new message in a thread.
func (r *Repository) CreateThreadMessage(msg *ThreadMessage) error {
	return r.db.Create(msg).Error
}

// ListThreadMessages retrieves messages in a thread with pagination.
func (r *Repository) ListThreadMessages(threadID uint64, offset, limit int) ([]ThreadMessage, int64, error) {
	query := r.db.Model(&ThreadMessage{}).Where("thread_id = ?", threadID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var messages []ThreadMessage
	err := query.Order("created_at ASC").Offset(offset).Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

// UpdateThreadTimestamp updates the updated_at timestamp on a thread (e.g., when a new message is added).
func (r *Repository) UpdateThreadTimestamp(threadID uint64) error {
	return r.db.Model(&MessageThread{}).Where("id = ?", threadID).
		Update("updated_at", time.Now().UTC()).Error
}
