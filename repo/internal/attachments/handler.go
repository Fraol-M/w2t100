package attachments

import (
	"path/filepath"
	"strconv"

	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Handler holds HTTP handlers for attachment endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new attachment Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Upload handles POST /work-orders/:id/attachments
func (h *Handler) Upload(c *gin.Context) {
	woID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid work order ID"))
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("file is required"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	attachment, appErr := h.service.Upload(woID, file, userID, roles, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToAttachmentResponse(attachment))
}

// Download handles GET /attachments/:id
func (h *Handler) Download(c *gin.Context) {
	attachID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid attachment ID"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))

	attachment, fullPath, appErr := h.service.Download(attachID, userID, roles)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	c.Header("Content-Disposition", "inline; filename=\""+attachment.Filename+"\"")
	c.Header("Content-Type", attachment.MimeType)
	c.File(filepath.Clean(fullPath))
}

// Delete handles DELETE /attachments/:id
func (h *Handler) Delete(c *gin.Context) {
	attachID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid attachment ID"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	appErr := h.service.Delete(attachID, userID, roles, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.NoContent(c)
}

// ListByWorkOrder handles GET /work-orders/:id/attachments
func (h *Handler) ListByWorkOrder(c *gin.Context) {
	woID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid work order ID"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))

	attachments, appErr := h.service.FindByWorkOrder(woID, userID, roles)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	responses := make([]AttachmentResponse, len(attachments))
	for i := range attachments {
		responses[i] = ToAttachmentResponse(&attachments[i])
	}

	common.Success(c, responses)
}
