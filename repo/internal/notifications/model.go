package notifications

import "time"

// NotificationTemplate stores reusable notification templates with Go text/template syntax.
type NotificationTemplate struct {
	ID              uint64    `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"size:100;uniqueIndex" json:"name"`
	SubjectTemplate string    `gorm:"size:500" json:"subject_template"`
	BodyTemplate    string    `gorm:"type:text" json:"body_template"`
	Category        string    `gorm:"size:50" json:"category"`
	IsActive        bool      `gorm:"default:true" json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName overrides the default table name.
func (NotificationTemplate) TableName() string {
	return "notification_templates"
}

// Notification represents a single notification sent to a user.
type Notification struct {
	ID           uint64     `gorm:"primaryKey" json:"id"`
	UUID         string     `gorm:"size:36;uniqueIndex" json:"uuid"`
	RecipientID  uint64     `gorm:"index" json:"recipient_id"`
	TemplateID   *uint64    `json:"template_id,omitempty"`
	Subject      string     `gorm:"size:500" json:"subject"`
	Body         string     `gorm:"type:text" json:"body"`
	Category     string     `gorm:"size:50;default:System" json:"category"`
	Status       string     `gorm:"size:20;default:Pending" json:"status"`
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	SentAt       *time.Time `json:"sent_at,omitempty"`
	RetryCount   int        `gorm:"default:0" json:"retry_count"`
	LastRetryAt  *time.Time `json:"last_retry_at,omitempty"`
	EntityType   string     `gorm:"size:50" json:"entity_type,omitempty"`
	EntityID     *uint64    `json:"entity_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TableName overrides the default table name.
func (Notification) TableName() string {
	return "notifications"
}

// NotificationReceipt tracks when a user has read a notification.
// There is a unique constraint on (notification_id, user_id).
type NotificationReceipt struct {
	ID             uint64     `gorm:"primaryKey" json:"id"`
	NotificationID uint64     `gorm:"uniqueIndex:idx_receipt_notif_user" json:"notification_id"`
	UserID         uint64     `gorm:"uniqueIndex:idx_receipt_notif_user" json:"user_id"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// TableName overrides the default table name.
func (NotificationReceipt) TableName() string {
	return "notification_receipts"
}

// MessageThread represents a conversation thread, optionally linked to a work order.
type MessageThread struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	UUID        string    `gorm:"size:36;uniqueIndex" json:"uuid"`
	WorkOrderID *uint64   `json:"work_order_id,omitempty"`
	Subject     string    `gorm:"size:255" json:"subject"`
	CreatedBy   uint64    `json:"created_by"`
	IsClosed    bool      `gorm:"default:false" json:"is_closed"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName overrides the default table name.
func (MessageThread) TableName() string {
	return "message_threads"
}

// ThreadParticipant tracks which users are participants in a message thread.
type ThreadParticipant struct {
	ID       uint64    `gorm:"primaryKey" json:"id"`
	ThreadID uint64    `gorm:"uniqueIndex:idx_thread_participant" json:"thread_id"`
	UserID   uint64    `gorm:"uniqueIndex:idx_thread_participant" json:"user_id"`
	JoinedAt time.Time `json:"joined_at"`
}

// TableName overrides the default table name.
func (ThreadParticipant) TableName() string {
	return "thread_participants"
}

// ThreadMessage represents a single message within a thread.
type ThreadMessage struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	UUID      string    `gorm:"size:36;uniqueIndex" json:"uuid"`
	ThreadID  uint64    `gorm:"index" json:"thread_id"`
	SenderID  uint64    `json:"sender_id"`
	Body      string    `gorm:"type:text" json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName overrides the default table name.
func (ThreadMessage) TableName() string {
	return "thread_messages"
}
