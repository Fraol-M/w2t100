package attachments

import "time"

// Attachment represents a file attached to a work order or other entity.
type Attachment struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	UUID        string    `gorm:"size:36;uniqueIndex" json:"uuid"`
	EntityType  string    `gorm:"size:50;default:WorkOrder" json:"entity_type"`
	EntityID    uint64    `gorm:"index" json:"entity_id"`
	Filename    string    `gorm:"size:255" json:"filename"`
	MimeType    string    `gorm:"size:100" json:"mime_type"`
	FileSize    uint64    `json:"file_size"`
	SHA256Hash  string    `gorm:"size:64;index" json:"sha256_hash"`
	StoragePath string    `gorm:"size:512" json:"storage_path"`
	UploadedBy  uint64    `json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName overrides the default table name.
func (Attachment) TableName() string {
	return "attachments"
}
