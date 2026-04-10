package logs

import (
	"time"

	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Handler holds Gin handler methods for log query endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new logs Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /admin/logs
// Supports query params: from, to, level, category, request_id, actor_id, page, per_page
// Restricted to SystemAdmin.
func (h *Handler) List(c *gin.Context) {
	filters := QueryFilters{}

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
	if v := c.Query("level"); v != "" {
		filters.Level = v
	}
	if v := c.Query("category"); v != "" {
		filters.Category = v
	}
	if v := c.Query("request_id"); v != "" {
		filters.RequestID = v
	}
	if v := c.Query("actor_id"); v != "" {
		filters.ActorID = v
	}

	filters.Page, filters.PerPage = common.PaginationFromQuery(c)

	entries, total, err := h.service.Query(filters)
	if err != nil {
		common.RespondError(c, common.NewInternalError("Failed to query logs"))
		return
	}

	common.SuccessWithMeta(c, entries, common.BuildMeta(filters.Page, filters.PerPage, total))
}
