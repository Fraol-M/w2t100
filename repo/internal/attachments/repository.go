package attachments

import "gorm.io/gorm"

// Repository handles all attachment database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new attachment Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new attachment record.
func (r *Repository) Create(a *Attachment) error {
	return r.db.Create(a).Error
}

// FindByID loads an attachment by its primary key.
func (r *Repository) FindByID(id uint64) (*Attachment, error) {
	var a Attachment
	err := r.db.Where("id = ?", id).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// FindByEntity retrieves all attachments for a given entity type and ID.
func (r *Repository) FindByEntity(entityType string, entityID uint64) ([]Attachment, error) {
	var attachments []Attachment
	err := r.db.Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at ASC").
		Find(&attachments).Error
	return attachments, err
}

// CountByEntity counts the number of attachments for a given entity type and ID.
func (r *Repository) CountByEntity(entityType string, entityID uint64) (int, error) {
	var count int64
	err := r.db.Model(&Attachment{}).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Count(&count).Error
	return int(count), err
}

// Delete removes an attachment record by its primary key.
func (r *Repository) Delete(id uint64) error {
	return r.db.Delete(&Attachment{}, id).Error
}
