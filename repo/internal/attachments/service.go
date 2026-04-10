package attachments

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"github.com/google/uuid"
)

// canAccessWorkOrder returns true if the actor may access attachments for the given work order.
//   - SystemAdmin / ComplianceReviewer: unconditional access (cross-property review is intended).
//   - PropertyManager: access only when they actively manage the work order's property.
//   - Technician: access only when the work order is assigned to them.
//   - Tenant: access only to work orders they created.
func (s *Service) canAccessWorkOrder(woID, actorID uint64, roles []string) (bool, error) {
	// Roles with unconditional global access.
	for _, r := range roles {
		if r == "SystemAdmin" || r == "ComplianceReviewer" {
			return true, nil
		}
	}

	// PropertyManager: must manage the property of this work order.
	for _, r := range roles {
		if r == "PropertyManager" {
			propertyID, err := s.woQuerier.GetWorkOrderPropertyID(woID)
			if err != nil {
				return false, err
			}
			return s.woQuerier.IsManagedBy(propertyID, actorID)
		}
	}

	// Technician: must be the assigned technician.
	for _, r := range roles {
		if r == "Technician" {
			assignedTo, err := s.woQuerier.GetWorkOrderAssignedTo(woID)
			if err != nil {
				return false, err
			}
			return assignedTo != nil && *assignedTo == actorID, nil
		}
	}

	// Tenant: must be the work order creator.
	ownerID, err := s.woQuerier.GetWorkOrderOwnerID(woID)
	if err != nil {
		return false, err
	}
	return ownerID == actorID, nil
}

const (
	// MaxFileSize is the maximum allowed file size (5 MB).
	MaxFileSize = 5 * 1024 * 1024
	// MaxAttachmentsPerWorkOrder is the maximum number of attachments per work order.
	MaxAttachmentsPerWorkOrder = 6
	// entityTypeWorkOrder is the entity type for work orders.
	entityTypeWorkOrder = "WorkOrder"
)

// WorkOrderQuerier abstracts the work order lookup needed by the attachment service.
type WorkOrderQuerier interface {
	FindWorkOrderForAttachment(id uint64) (woID uint64, woUUID string, err error)
	// GetWorkOrderOwnerID returns the TenantID (original creator) of the work order.
	GetWorkOrderOwnerID(woID uint64) (tenantID uint64, err error)
	// GetWorkOrderAssignedTo returns the assigned technician's user ID (nil if unassigned).
	GetWorkOrderAssignedTo(woID uint64) (assignedTo *uint64, err error)
	// GetWorkOrderPropertyID returns the property_id of the work order.
	GetWorkOrderPropertyID(woID uint64) (propertyID uint64, err error)
	// IsManagedBy returns true if userID has an active PropertyManager assignment on propertyID.
	IsManagedBy(propertyID, userID uint64) (bool, error)
}

// AuditLogger abstracts the audit logging dependency.
type AuditLogger interface {
	Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string)
}

// Service handles attachment business logic.
type Service struct {
	repo       *Repository
	woQuerier  WorkOrderQuerier
	audit      AuditLogger
	storageRoot string
}

// NewService creates a new attachment Service.
func NewService(repo *Repository, woQuerier WorkOrderQuerier, audit AuditLogger, storageCfg config.StorageConfig) *Service {
	return &Service{
		repo:        repo,
		woQuerier:   woQuerier,
		audit:       audit,
		storageRoot: storageCfg.Root,
	}
}

// Upload validates and stores an attachment for a work order.
// uploaderRoles is used to enforce object-level authorization: Tenant users may only
// upload to work orders they created; all other roles may upload to any work order.
func (s *Service) Upload(workOrderID uint64, file *multipart.FileHeader, uploaderID uint64, uploaderRoles []string, ip, requestID string) (*Attachment, *common.AppError) {
	// Object-level authorization: verify the uploader has access to this work order.
	ok, err := s.canAccessWorkOrder(workOrderID, uploaderID, uploaderRoles)
	if err != nil {
		return nil, common.NewNotFoundError("Work order")
	}
	if !ok {
		return nil, common.NewForbiddenError("you do not have access to this work order")
	}

	// Validate file size.
	if file.Size > MaxFileSize {
		return nil, common.NewPayloadTooLargeError("file must be at most 5 MB")
	}

	// Check attachment count limit.
	currentCount, err := s.repo.CountByEntity(entityTypeWorkOrder, workOrderID)
	if err != nil {
		return nil, common.NewInternalError("failed to check attachment count")
	}
	if fe := common.ValidateAttachmentCount(currentCount, 1, MaxAttachmentsPerWorkOrder); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	// Look up the work order to get its UUID for the storage path.
	_, woUUID, err := s.woQuerier.FindWorkOrderForAttachment(workOrderID)
	if err != nil {
		return nil, common.NewNotFoundError("Work order")
	}

	// Open the uploaded file.
	src, err := file.Open()
	if err != nil {
		return nil, common.NewInternalError("failed to open uploaded file")
	}
	defer src.Close()

	// Read the file header for magic byte validation.
	header := make([]byte, 512)
	n, err := src.Read(header)
	if err != nil && err != io.EOF {
		return nil, common.NewInternalError("failed to read file header")
	}
	header = header[:n]

	// Validate magic bytes.
	detectedMIME, fe := common.ValidateFileSignature(header)
	if fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	// Validate declared MIME against detected.
	declaredMIME := strings.ToLower(file.Header.Get("Content-Type"))
	if fe := common.ValidateImageMIME(declaredMIME); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}
	if declaredMIME != detectedMIME {
		return nil, common.NewValidationError("declared MIME type does not match file content")
	}

	// Reset reader to beginning for hashing and storage.
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, common.NewInternalError("failed to reset file reader")
	}

	// Compute SHA-256 hash.
	hasher := sha256.New()
	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return nil, common.NewInternalError("failed to read file")
	}
	hasher.Write(fileBytes)
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Determine file extension from detected MIME.
	ext := ".jpg"
	if detectedMIME == "image/png" {
		ext = ".png"
	}

	// Generate storage path.
	attachUUID := uuid.New().String()
	storageDirRel := filepath.Join("attachments", woUUID)
	storageDir := filepath.Join(s.storageRoot, storageDirRel)
	if err := os.MkdirAll(storageDir, 0750); err != nil {
		return nil, common.NewInternalError("failed to create storage directory")
	}

	filename := attachUUID + ext
	storagePath := filepath.Join(storageDirRel, filename)
	fullPath := filepath.Join(s.storageRoot, storagePath)

	// Write file to disk.
	if err := os.WriteFile(fullPath, fileBytes, 0640); err != nil {
		return nil, common.NewInternalError("failed to write file to storage")
	}

	// Create the database record.
	attachment := &Attachment{
		UUID:        attachUUID,
		EntityType:  entityTypeWorkOrder,
		EntityID:    workOrderID,
		Filename:    file.Filename,
		MimeType:    detectedMIME,
		FileSize:    uint64(file.Size),
		SHA256Hash:  hash,
		StoragePath: storagePath,
		UploadedBy:  uploaderID,
	}

	if err := s.repo.Create(attachment); err != nil {
		// Clean up the file on DB failure.
		_ = os.Remove(fullPath)
		return nil, common.NewInternalError("failed to save attachment record")
	}

	s.audit.Log(uploaderID, common.AuditActionCreate, "Attachment", attachment.ID,
		fmt.Sprintf("Uploaded attachment for work order %d", workOrderID), ip, requestID)

	return attachment, nil
}

// Download retrieves the file path and metadata for an attachment.
// actorID and roles are used to enforce object-level authorization.
func (s *Service) Download(attachmentID uint64, actorID uint64, roles []string) (*Attachment, string, *common.AppError) {
	attachment, err := s.repo.FindByID(attachmentID)
	if err != nil {
		return nil, "", common.NewNotFoundError("Attachment")
	}

	// Enforce access to the parent work order.
	if attachment.EntityType == entityTypeWorkOrder {
		ok, err := s.canAccessWorkOrder(attachment.EntityID, actorID, roles)
		if err != nil || !ok {
			return nil, "", common.NewForbiddenError("you do not have access to this attachment")
		}
	}

	fullPath := filepath.Join(s.storageRoot, attachment.StoragePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, "", common.NewNotFoundError("Attachment file")
	}

	return attachment, fullPath, nil
}

// Delete removes an attachment's file and database record.
// Authorization uses the same property-scoped rules as Upload/Download:
//   - SystemAdmin: unconditional.
//   - Work-order attachments: delegates to canAccessWorkOrder (PM must manage the property).
//   - Non-work-order attachments: only the uploader or SystemAdmin may delete.
func (s *Service) Delete(attachmentID uint64, actorID uint64, roles []string, ip, requestID string) *common.AppError {
	attachment, err := s.repo.FindByID(attachmentID)
	if err != nil {
		return common.NewNotFoundError("Attachment")
	}

	// SystemAdmin has unconditional access.
	isAdmin := false
	for _, r := range roles {
		if r == "SystemAdmin" {
			isAdmin = true
			break
		}
	}

	if !isAdmin {
		if attachment.EntityType == entityTypeWorkOrder {
			// Enforce property-scope for PM and all other roles.
			ok, err := s.canAccessWorkOrder(attachment.EntityID, actorID, roles)
			if err != nil {
				return common.NewNotFoundError("Work order")
			}
			if !ok {
				return common.NewForbiddenError("you do not have access to this attachment")
			}
		} else {
			// Non-WO attachments: only the uploader may delete.
			if attachment.UploadedBy != actorID {
				return common.NewForbiddenError("only the uploader or a system admin may delete this attachment")
			}
		}
	}

	// Remove the file from storage.
	fullPath := filepath.Join(s.storageRoot, attachment.StoragePath)
	_ = os.Remove(fullPath)

	// Delete the database record.
	if err := s.repo.Delete(attachmentID); err != nil {
		return common.NewInternalError("failed to delete attachment record")
	}

	s.audit.Log(actorID, common.AuditActionDelete, "Attachment", attachment.ID,
		fmt.Sprintf("Deleted attachment %s from entity %s/%d", attachment.UUID, attachment.EntityType, attachment.EntityID), ip, requestID)

	return nil
}

// FindByWorkOrder retrieves all attachments for a work order, enforcing object-level access.
func (s *Service) FindByWorkOrder(woID uint64, actorID uint64, roles []string) ([]Attachment, *common.AppError) {
	ok, err := s.canAccessWorkOrder(woID, actorID, roles)
	if err != nil {
		return nil, common.NewNotFoundError("Work order")
	}
	if !ok {
		return nil, common.NewForbiddenError("you do not have access to this work order")
	}
	attachments, err := s.repo.FindByEntity(entityTypeWorkOrder, woID)
	if err != nil {
		return nil, common.NewInternalError("failed to retrieve attachments")
	}
	return attachments, nil
}

// UploadEvidence stores an attachment for a non-work-order entity (e.g., governance reports).
// Validation rules are the same as Upload (JPEG/PNG, max 5 MB, max 6 per entity).
// Access control must be enforced by the caller before invoking this method.
func (s *Service) UploadEvidence(entityType string, entityID uint64, file *multipart.FileHeader, uploaderID uint64, ip, requestID string) (*Attachment, *common.AppError) {
	if file.Size > MaxFileSize {
		return nil, common.NewPayloadTooLargeError("file must be at most 5 MB")
	}

	currentCount, err := s.repo.CountByEntity(entityType, entityID)
	if err != nil {
		return nil, common.NewInternalError("failed to check attachment count")
	}
	if fe := common.ValidateAttachmentCount(currentCount, 1, MaxAttachmentsPerWorkOrder); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	src, err := file.Open()
	if err != nil {
		return nil, common.NewInternalError("failed to open uploaded file")
	}
	defer src.Close()

	header := make([]byte, 512)
	n, err := src.Read(header)
	if err != nil && err != io.EOF {
		return nil, common.NewInternalError("failed to read file header")
	}
	header = header[:n]

	detectedMIME, fe := common.ValidateFileSignature(header)
	if fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}
	declaredMIME := strings.ToLower(file.Header.Get("Content-Type"))
	if fe := common.ValidateImageMIME(declaredMIME); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}
	if declaredMIME != detectedMIME {
		return nil, common.NewValidationError("declared MIME type does not match file content")
	}

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, common.NewInternalError("failed to reset file reader")
	}

	hasher := sha256.New()
	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return nil, common.NewInternalError("failed to read file")
	}
	hasher.Write(fileBytes)

	fileUUID := uuid.New().String()
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		switch detectedMIME {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		}
	}
	storagePath := filepath.Join("evidence", fmt.Sprintf("%s%s", fileUUID, ext))
	fullPath := filepath.Join(s.storageRoot, storagePath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0750); err != nil {
		return nil, common.NewInternalError("failed to create storage directory")
	}
	if err := os.WriteFile(fullPath, fileBytes, 0640); err != nil {
		return nil, common.NewInternalError("failed to save file")
	}

	attachment := &Attachment{
		UUID:        fileUUID,
		EntityType:  entityType,
		EntityID:    entityID,
		Filename:    file.Filename,
		StoragePath: storagePath,
		MimeType:    detectedMIME,
		FileSize:    uint64(file.Size),
		SHA256Hash:  hex.EncodeToString(hasher.Sum(nil)),
		UploadedBy:  uploaderID,
	}

	if err := s.repo.Create(attachment); err != nil {
		_ = os.Remove(fullPath)
		return nil, common.NewInternalError("failed to save attachment record")
	}

	s.audit.Log(uploaderID, common.AuditActionCreate, "Attachment", attachment.ID,
		fmt.Sprintf("Uploaded evidence %s for %s/%d", attachment.UUID, entityType, entityID), ip, requestID)

	return attachment, nil
}

// FindByEntity retrieves all attachments for a given entity (no access check — internal use only).
func (s *Service) FindByEntity(entityType string, entityID uint64) ([]Attachment, *common.AppError) {
	attachments, err := s.repo.FindByEntity(entityType, entityID)
	if err != nil {
		return nil, common.NewInternalError("failed to retrieve attachments")
	}
	return attachments, nil
}

// DeleteForWorkOrder removes all attachment files and DB records for a work order.
// This is an internal-only cleanup method used during rollback; it bypasses access-control checks.
func (s *Service) DeleteForWorkOrder(workOrderID uint64) {
	attachments, err := s.repo.FindByEntity(entityTypeWorkOrder, workOrderID)
	if err != nil {
		return
	}
	for _, a := range attachments {
		fullPath := filepath.Join(s.storageRoot, a.StoragePath)
		_ = os.Remove(fullPath)
		_ = s.repo.Delete(a.ID)
	}
}

// ComputeSHA256 computes the SHA-256 hash of the given data.
// Exported for testing purposes.
func ComputeSHA256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
