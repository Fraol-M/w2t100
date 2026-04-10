package audit

import (
	"strconv"
	"time"

	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Handler holds Gin handler methods for audit log endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new audit Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /admin/audit-logs
// Supports query params: from, to, action, actor_id, request_id, page, per_page
// Restricted to SystemAdmin.
func (h *Handler) List(c *gin.Context) {
	filters := ListFilters{}

	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filters.From = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filters.To = t
		}
	}
	if v := c.Query("action"); v != "" {
		filters.Category = v
	}
	if v := c.Query("actor_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			filters.ActorID = &id
		}
	}
	if v := c.Query("request_id"); v != "" {
		filters.RequestID = v
	}

	filters.Page, filters.PerPage = common.PaginationFromQuery(c)

	logs, total, err := h.service.List(filters)
	if err != nil {
		common.RespondError(c, common.NewInternalError("Failed to retrieve audit logs"))
		return
	}

	common.SuccessWithMeta(c, logs, common.BuildMeta(filters.Page, filters.PerPage, total))
}

// Get handles GET /admin/audit-logs/:id
// Returns a single audit log by its numeric primary-key ID.
// Restricted to SystemAdmin.
func (h *Handler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid audit log ID"))
		return
	}

	entry, appErr := h.service.GetByID(id)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, entry)
}
