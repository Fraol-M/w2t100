package admin

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SecurityService is the encryption/key-management interface required by admin.
type SecurityService interface {
	ActiveKeyID() int
	KeyRotationDue() bool
	ListKeyVersions() ([]int, error)
	RotateKey() (int, error)
}

// AuditLogger is the audit interface required by admin.
type AuditLogger interface {
	Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string)
}

// UserService is the minimal user-query interface required by the admin package.
// The method signature is intentionally broad (any, *AppError) so that concrete
// service types from other packages satisfy this interface without the admin
// package importing them directly.
type UserService interface {
	// GetByID is satisfied by *users.Service.GetByID via the userServiceWrapper adapter
	// defined in routes.go-compatible wiring.
}

// systemSetting represents a key/value configuration row in the DB.
// Columns: id, setting_key, setting_value, description, created_at, updated_at
type systemSetting struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"                json:"id"`
	SettingKey   string    `gorm:"column:setting_key;uniqueIndex;size:255"  json:"key"`
	SettingValue string    `gorm:"column:setting_value;type:text"           json:"value"`
	CreatedAt    time.Time `                                                json:"created_at"`
	UpdatedAt    time.Time `                                                json:"updated_at"`
}

func (systemSetting) TableName() string { return "system_settings" }

// Handler holds dependencies for all admin HTTP handlers.
type Handler struct {
	db       *gorm.DB
	cfg      *config.Config
	security SecurityService
	audit    AuditLogger
}

// NewHandler creates a new admin Handler.
// The UserService parameter is accepted for interface compatibility with the
// app/routes.go wiring but is not used directly inside handlers.
func NewHandler(db *gorm.DB, cfg *config.Config, security SecurityService, audit AuditLogger, _ UserService) *Handler {
	return &Handler{
		db:       db,
		cfg:      cfg,
		security: security,
		audit:    audit,
	}
}

// ListSettings handles GET /admin/settings
// Returns all key/value pairs from the system_settings table.
func (h *Handler) ListSettings(c *gin.Context) {
	var settings []systemSetting
	if err := h.db.Find(&settings).Error; err != nil {
		common.RespondError(c, common.NewInternalError("failed to retrieve settings"))
		return
	}
	common.Success(c, settings)
}

// UpdateSetting handles PUT /admin/settings/:key
// Updates a single system setting.
func (h *Handler) UpdateSetting(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		common.RespondError(c, common.NewBadRequestError("setting key is required"))
		return
	}

	var body struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.RespondError(c, common.NewBadRequestError("value is required"))
		return
	}

	// Load existing record (needed to preserve ID for Save to issue UPDATE, not INSERT).
	var setting systemSetting
	if err := h.db.Where("setting_key = ?", key).First(&setting).Error; err != nil {
		common.RespondError(c, common.NewNotFoundError("setting"))
		return
	}
	setting.SettingValue = body.Value

	if err := h.db.Save(&setting).Error; err != nil {
		common.RespondError(c, common.NewInternalError("failed to update setting"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	h.audit.Log(userID, common.AuditActionUpdate, "SystemSetting", 0,
		fmt.Sprintf("Updated system setting %q", key), ip, reqID)

	common.Success(c, setting)
}

// ListKeys handles GET /admin/keys
// Returns active key ID, rotation status, and all loaded key versions.
func (h *Handler) ListKeys(c *gin.Context) {
	versions, err := h.security.ListKeyVersions()
	if err != nil {
		common.RespondError(c, common.NewInternalError("failed to list key versions"))
		return
	}

	common.Success(c, gin.H{
		"active_key_id":    h.security.ActiveKeyID(),
		"rotation_due":     h.security.KeyRotationDue(),
		"loaded_versions":  versions,
	})
}

// RotateKey handles POST /admin/keys/rotate
// Generates a new encryption key, sets it as active, and records the rotation
// in the encryption_key_versions table.
func (h *Handler) RotateKey(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	newVersion, err := h.security.RotateKey()
	if err != nil {
		common.RespondError(c, common.NewInternalError("key rotation failed: "+err.Error()))
		return
	}

	// Record the rotation event in the encryption_key_versions table.
	// The table tracks key version metadata; actual key material lives in key files.
	now := time.Now().UTC()
	h.db.Exec(`INSERT INTO encryption_key_versions
		(key_version, algorithm, is_active, activated_at, created_at)
		VALUES (?, 'AES-256-GCM', TRUE, ?, ?)
		ON DUPLICATE KEY UPDATE is_active = TRUE, activated_at = ?`,
		newVersion, now, now, now)
	// Mark previous versions inactive.
	h.db.Exec(`UPDATE encryption_key_versions SET is_active = FALSE WHERE key_version != ?`, newVersion)

	h.audit.Log(userID, common.AuditActionKeyRotation, "EncryptionKey", uint64(newVersion),
		fmt.Sprintf("Rotated encryption key to version %d", newVersion), ip, reqID)

	common.Created(c, gin.H{
		"new_key_version": newVersion,
		"message":         "key rotation completed successfully",
	})
}

// ExportPII handles POST /admin/pii-export
// Requires an explicit purpose (min 10 chars), exports a CSV, and audit-logs the action.
func (h *Handler) ExportPII(c *gin.Context) {
	var req struct {
		Type    string `json:"type"    binding:"required"`
		Purpose string `json:"purpose" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("type and purpose are required"))
		return
	}

	if len(strings.TrimSpace(req.Purpose)) < 10 {
		common.RespondError(c, common.NewValidationError("purpose must be at least 10 characters",
			common.FieldError{Field: "purpose", Message: "must be at least 10 characters"}))
		return
	}

	allowedTypes := map[string]bool{
		"users":    true,
		"tenants":  true,
		"payments": true,
	}
	if !allowedTypes[req.Type] {
		common.RespondError(c, common.NewBadRequestError(fmt.Sprintf("unsupported export type: %s", req.Type)))
		return
	}

	exportDir := filepath.Join(h.cfg.Storage.Root, "exports")
	if err := os.MkdirAll(exportDir, 0750); err != nil {
		common.RespondError(c, common.NewInternalError("failed to create export directory"))
		return
	}

	exportID := uuid.New().String()
	filename := fmt.Sprintf("%s-%s.csv", req.Type, exportID)
	fullPath := filepath.Join(exportDir, filename)

	// Write export file with CSV header.
	f, err := os.Create(fullPath)
	if err != nil {
		common.RespondError(c, common.NewInternalError("failed to create export file"))
		return
	}
	defer f.Close()

	if err := h.writeExportCSV(f, req.Type); err != nil {
		_ = os.Remove(fullPath)
		common.RespondError(c, common.NewInternalError("failed to write export data"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	h.audit.Log(userID, common.AuditActionExport, "PII", 0,
		fmt.Sprintf("PII export type=%s purpose=%q file=%s", req.Type, req.Purpose, filename),
		ip, reqID)

	common.Created(c, gin.H{
		"file_path": fullPath,
		"filename":  filename,
		"type":      req.Type,
	})
}

// EnforceDataRetention handles DELETE /admin/data-retention
// Permanently deletes database records that have exceeded their minimum retention period:
//   - thread_messages older than MESSAGE_RETENTION_YEARS years
//   - payments older than FINANCIAL_RETENTION_YEARS years (rarely triggered; 7-year default)
//
// This is a destructive, irreversible operation and requires SystemAdmin authorization.
func (h *Handler) EnforceDataRetention(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	msgCutoff := time.Now().UTC().AddDate(-h.cfg.Retention.MessageYears, 0, 0)
	finCutoff := time.Now().UTC().AddDate(-h.cfg.Retention.FinancialYears, 0, 0)

	var msgDeleted, finDeleted int64

	// Purge thread_messages older than the message retention window.
	msgResult := h.db.Exec("DELETE FROM thread_messages WHERE created_at < ?", msgCutoff)
	if msgResult.Error != nil {
		common.RespondError(c, common.NewInternalError("failed to purge old messages: "+msgResult.Error.Error()))
		return
	}
	msgDeleted = msgResult.RowsAffected

	// Purge payments older than the financial retention window.
	// Only terminal states (Settled, Reversed, Expired) are eligible for deletion.
	finResult := h.db.Exec(
		"DELETE FROM payments WHERE created_at < ? AND status IN ('Settled','Reversed','Expired')",
		finCutoff,
	)
	if finResult.Error != nil {
		common.RespondError(c, common.NewInternalError("failed to purge old payments: "+finResult.Error.Error()))
		return
	}
	finDeleted = finResult.RowsAffected

	h.audit.Log(userID, "DataRetentionEnforced", "System", 0,
		fmt.Sprintf("data retention: deleted %d messages (cutoff %s), %d payments (cutoff %s)",
			msgDeleted, msgCutoff.Format(time.RFC3339), finDeleted, finCutoff.Format(time.RFC3339)),
		ip, reqID)

	common.Success(c, gin.H{
		"messages_deleted":    msgDeleted,
		"payments_deleted":    finDeleted,
		"message_cutoff":      msgCutoff.Format(time.RFC3339),
		"financial_cutoff":    finCutoff.Format(time.RFC3339),
	})
}

// IngestAnomaly handles POST /admin/anomaly
// Stores an anomaly record in audit_logs. Protected by AnomalyAllowlist middleware.
// No external dependencies or credentials are required.
func (h *Handler) IngestAnomaly(c *gin.Context) {
	var body struct {
		Source      string                 `json:"source"`
		Severity    string                 `json:"severity"`
		Description string                 `json:"description"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid anomaly payload"))
		return
	}

	if body.Description == "" {
		common.RespondError(c, common.NewBadRequestError("description is required"))
		return
	}

	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	desc := fmt.Sprintf("[source=%s severity=%s] %s", body.Source, body.Severity, body.Description)

	// Log the anomaly as an audit entry with a dedicated action name.
	h.audit.Log(0, "AnomalyIngestion", "System", 0, desc, ip, reqID)

	common.Created(c, gin.H{
		"status":  "ingested",
		"message": "anomaly recorded successfully",
	})
}

// --- internal helpers ---

// writeExportCSV writes a simple CSV for the given type using raw SQL.
func (h *Handler) writeExportCSV(f *os.File, exportType string) error {
	switch exportType {
	case "users":
		return h.exportTable(f, "users",
			[]string{"id", "uuid", "username", "email", "first_name", "last_name", "is_active", "created_at"})
	case "tenants":
		return h.exportTable(f, "tenant_profiles",
			[]string{"id", "uuid", "user_id", "unit_id", "lease_start", "lease_end", "move_in_date", "created_at"})
	case "payments":
		return h.exportTable(f, "payments",
			[]string{"id", "uuid", "property_id", "tenant_id", "kind", "amount", "currency", "status", "created_at"})
	default:
		return fmt.Errorf("unsupported type: %s", exportType)
	}
}

// formulaSafeCell prefixes a cell value with a single quote when its first character
// would be interpreted as a formula by spreadsheet applications (=, +, -, @, tab, CR).
// This prevents CSV injection (formula injection) attacks.
func formulaSafeCell(v string) string {
	if len(v) > 0 {
		switch v[0] {
		case '=', '+', '-', '@', '\t', '\r':
			return "'" + v
		}
	}
	return v
}

func (h *Handler) exportTable(f *os.File, table string, columns []string) error {
	cols := make([]string, len(columns))
	for i, c := range columns {
		cols[i] = "`" + c + "`"
	}
	query := fmt.Sprintf("SELECT %s FROM `%s`", strings.Join(cols, ", "), table)

	sqlDB, err := h.db.DB()
	if err != nil {
		return err
	}
	rows, err := sqlDB.Query(query)
	if err != nil {
		return fmt.Errorf("export query failed for table %q: %w", table, err)
	}
	defer rows.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Write header row.
	if err := w.Write(columns); err != nil {
		return err
	}

	for rows.Next() {
		ptrs := make([]interface{}, len(columns))
		vals := make([]interface{}, len(columns))
		for i := range ptrs {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		record := make([]string, len(columns))
		for i, v := range vals {
			var s string
			if v == nil {
				s = ""
			} else {
				s = fmt.Sprintf("%v", v)
			}
			record[i] = formulaSafeCell(s)
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return w.Error()
}
