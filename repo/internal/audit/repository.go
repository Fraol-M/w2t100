package audit

import (
	"time"

	"propertyops/backend/internal/common"

	"gorm.io/gorm"
)

// Repository handles database operations for audit logs.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new audit Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ListFilters contains all filter options for listing audit logs.
type ListFilters struct {
	From      time.Time
	To        time.Time
	Level     string
	Category  string // maps to Action field
	ActorID   *uint64
	RequestID string
	Page      int
	PerPage   int
}

// Create inserts a new audit log record into the database.
func (r *Repository) Create(log *AuditLog) error {
	return r.db.Create(log).Error
}

// GetByID returns a single audit log by its primary key, or a not-found error.
func (r *Repository) GetByID(id uint64) (*AuditLog, *common.AppError) {
	var entry AuditLog
	if err := r.db.First(&entry, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, common.NewNotFoundError("Audit log")
		}
		return nil, common.NewInternalError("")
	}
	return &entry, nil
}

// List returns paginated audit logs matching the provided filters along with the total count.
func (r *Repository) List(filters ListFilters) ([]AuditLog, int64, error) {
	q := r.db.Model(&AuditLog{})

	if !filters.From.IsZero() {
		q = q.Where("created_at >= ?", filters.From)
	}
	if !filters.To.IsZero() {
		q = q.Where("created_at <= ?", filters.To)
	}
	if filters.Category != "" {
		q = q.Where("action = ?", filters.Category)
	}
	if filters.ActorID != nil {
		q = q.Where("actor_id = ?", *filters.ActorID)
	}
	if filters.RequestID != "" {
		q = q.Where("request_id = ?", filters.RequestID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filters.Page
	if page < 1 {
		page = 1
	}
	perPage := filters.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	offset := (page - 1) * perPage

	var logs []AuditLog
	if err := q.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
