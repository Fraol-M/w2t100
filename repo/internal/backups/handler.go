package backups

import (
	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Handler holds Gin handler methods for backup endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new backup Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// CreateBackup handles POST /backups
func (h *Handler) CreateBackup(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	record, err := h.service.CreateBackup(userID, ip, reqID)
	if err != nil {
		common.RespondError(c, common.NewInternalError(err.Error()))
		return
	}

	common.Created(c, record)
}

// ListBackups handles GET /backups
func (h *Handler) ListBackups(c *gin.Context) {
	records, err := h.service.ListBackups()
	if err != nil {
		common.RespondError(c, common.NewInternalError("failed to list backups"))
		return
	}
	common.Success(c, records)
}

// ValidateBackup handles POST /backups/validate
// Request body: {"file_path": "/absolute/path/to/backup.sql.enc"}
func (h *Handler) ValidateBackup(c *gin.Context) {
	var req struct {
		FilePath string `json:"file_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("file_path is required"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	result, err := h.service.ValidateBackup(req.FilePath, userID, ip, reqID)
	if err != nil {
		common.RespondError(c, common.NewInternalError("validation error: "+err.Error()))
		return
	}

	common.Success(c, result)
}

// ApplyRetention handles DELETE /backups/retention
func (h *Handler) ApplyRetention(c *gin.Context) {
	if err := h.service.ApplyRetention(); err != nil {
		common.RespondError(c, common.NewInternalError("retention policy error: "+err.Error()))
		return
	}
	common.Success(c, gin.H{"message": "retention policy applied"})
}
