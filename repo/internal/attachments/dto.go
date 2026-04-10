package attachments

import "time"

// AttachmentResponse is the API response DTO for an attachment.
type AttachmentResponse struct {
	ID         uint64    `json:"id"`
	UUID       string    `json:"uuid"`
	EntityType string    `json:"entity_type"`
	EntityID   uint64    `json:"entity_id"`
	Filename   string    `json:"filename"`
	MimeType   string    `json:"mime_type"`
	FileSize   uint64    `json:"file_size"`
	SHA256Hash string    `json:"sha256_hash"`
	UploadedBy uint64    `json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
}

// ToAttachmentResponse converts an Attachment model to its API response DTO.
func ToAttachmentResponse(a *Attachment) AttachmentResponse {
	return AttachmentResponse{
		ID:         a.ID,
		UUID:       a.UUID,
		EntityType: a.EntityType,
		EntityID:   a.EntityID,
		Filename:   a.Filename,
		MimeType:   a.MimeType,
		FileSize:   a.FileSize,
		SHA256Hash: a.SHA256Hash,
		UploadedBy: a.UploadedBy,
		CreatedAt:  a.CreatedAt,
	}
}
