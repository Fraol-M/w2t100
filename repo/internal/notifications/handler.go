package notifications

import (
	"strconv"
	"time"

	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Handler holds HTTP handlers for notification and thread endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new notification Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListNotifications handles GET /notifications
func (h *Handler) ListNotifications(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))

	var req NotificationListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid query parameters"))
		return
	}

	page, perPage := common.PaginationFromQuery(c)

	filters := NotificationFilters{
		Status:   req.Status,
		Category: req.Category,
		ReadFlag: req.ReadFlag,
		UserID:   userID,
	}

	notifications, readStatuses, total, appErr := h.service.ListNotifications(userID, filters, page, perPage)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	responses := make([]NotificationResponse, len(notifications))
	for i := range notifications {
		responses[i] = ToNotificationResponse(&notifications[i], readStatuses[i])
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, responses, meta)
}

// GetNotification handles GET /notifications/:id
func (h *Handler) GetNotification(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))

	notifID, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid notification ID"))
		return
	}

	n, isRead, appErr := h.service.GetByID(notifID, userID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, ToNotificationResponse(n, isRead))
}

// MarkRead handles PATCH /notifications/:id/read
func (h *Handler) MarkRead(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))

	notifID, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid notification ID"))
		return
	}

	appErr := h.service.MarkRead(notifID, userID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.NoContent(c)
}

// GetUnreadCount handles GET /notifications/unread-count
func (h *Handler) GetUnreadCount(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))

	count, appErr := h.service.GetUnreadCount(userID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, UnreadCountResponse{Count: count})
}

// SendNotification handles POST /notifications/send
func (h *Handler) SendNotification(c *gin.Context) {
	roles := c.GetStringSlice(string(common.CtxKeyRoles))

	// Only admins and property managers can send direct notifications.
	if !hasRole(roles, common.RoleSystemAdmin) && !hasRole(roles, common.RolePropertyManager) {
		common.RespondError(c, common.NewForbiddenError("only admins and managers can send notifications"))
		return
	}

	var req SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	category := req.Category
	if category == "" {
		category = "System"
	}

	// Check if this is a scheduled notification.
	if req.ScheduledFor != nil && *req.ScheduledFor != "" {
		scheduledTime, parseErr := time.Parse(time.RFC3339, *req.ScheduledFor)
		if parseErr != nil {
			common.RespondError(c, common.NewBadRequestError("scheduled_for must be in RFC3339 format"))
			return
		}

		n, appErr := h.service.ScheduleNotification(req.RecipientID, req.Subject, req.Body, category, scheduledTime)
		if appErr != nil {
			common.RespondError(c, appErr)
			return
		}

		common.Created(c, ToNotificationResponse(n, false))
		return
	}

	n, appErr := h.service.SendDirect(req.RecipientID, req.Subject, req.Body, category)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToNotificationResponse(n, false))
}

// --- Thread handlers ---

// CreateThread handles POST /notifications/threads
func (h *Handler) CreateThread(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))

	var req CreateThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	// Access enforcement is delegated to the service (which holds the WorkOrderChecker).
	thread, appErr := h.service.CreateThread(userID, roles, req)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToThreadResponse(thread))
}

// ListThreads handles GET /notifications/threads
func (h *Handler) ListThreads(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))
	page, perPage := common.PaginationFromQuery(c)

	threads, total, appErr := h.service.ListThreads(userID, page, perPage)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	responses := make([]ThreadResponse, len(threads))
	for i := range threads {
		responses[i] = ToThreadResponse(&threads[i])
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, responses, meta)
}

// GetThread handles GET /notifications/threads/:id
func (h *Handler) GetThread(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))

	threadID, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid thread ID"))
		return
	}

	thread, appErr := h.service.GetThread(threadID, userID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, ToThreadResponse(thread))
}

// AddParticipant handles POST /notifications/threads/:id/participants
func (h *Handler) AddParticipant(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))

	threadID, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid thread ID"))
		return
	}

	var req AddParticipantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("user_id is required"))
		return
	}

	appErr := h.service.AddThreadParticipant(threadID, userID, req.UserID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ParticipantResponse{
		ThreadID: threadID,
		UserID:   req.UserID,
	})
}

// ListParticipants handles GET /notifications/threads/:id/participants
func (h *Handler) ListParticipants(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))

	threadID, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid thread ID"))
		return
	}

	participants, appErr := h.service.GetParticipants(threadID, userID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	responses := make([]ParticipantResponse, len(participants))
	for i, p := range participants {
		responses[i] = ParticipantResponse{
			ThreadID: p.ThreadID,
			UserID:   p.UserID,
			JoinedAt: p.JoinedAt,
		}
	}
	common.Success(c, responses)
}

// AddMessage handles POST /notifications/threads/:id/messages
func (h *Handler) AddMessage(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))

	threadID, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid thread ID"))
		return
	}

	var req AddMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	msg, appErr := h.service.AddMessage(threadID, userID, req)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToThreadMessageResponse(msg))
}

// ListMessages handles GET /notifications/threads/:id/messages
func (h *Handler) ListMessages(c *gin.Context) {
	userID := c.GetUint64(string(common.CtxKeyUserID))

	threadID, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid thread ID"))
		return
	}

	page, perPage := common.PaginationFromQuery(c)

	messages, total, appErr := h.service.ListThreadMessages(threadID, userID, page, perPage)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	responses := make([]ThreadMessageResponse, len(messages))
	for i := range messages {
		responses[i] = ToThreadMessageResponse(&messages[i])
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, responses, meta)
}

// --- Utility helpers ---

func parseIDParam(c *gin.Context, param string) (uint64, error) {
	return strconv.ParseUint(c.Param(param), 10, 64)
}

func hasRole(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}
