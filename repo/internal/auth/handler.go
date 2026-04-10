package auth

import (
	"net/http"

	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Handler holds Gin handler methods for authentication endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new auth Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Login handles POST /auth/login.
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body"))
		return
	}

	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqIDStr, _ := requestID.(string)

	resp, appErr := h.service.Login(req.Username, req.Password, ip, userAgent, reqIDStr)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{
		Success: true,
		Data:    resp,
	})
}

// Logout handles POST /auth/logout.
func (h *Handler) Logout(c *gin.Context) {
	sessionTokenHash, exists := c.Get(string(common.CtxKeySessionID))
	if !exists {
		common.RespondError(c, common.NewUnauthorizedError(""))
		return
	}

	tokenHash, ok := sessionTokenHash.(string)
	if !ok {
		common.RespondError(c, common.NewUnauthorizedError(""))
		return
	}

	userID, _ := c.Get(string(common.CtxKeyUserID))
	uid, _ := userID.(uint64)
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqIDStr, _ := requestID.(string)

	if appErr := h.service.Logout(tokenHash, uid, ip, reqIDStr); appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.NoContent(c)
}

// Me handles GET /auth/me.
func (h *Handler) Me(c *gin.Context) {
	userIDVal, exists := c.Get(string(common.CtxKeyUserID))
	if !exists {
		common.RespondError(c, common.NewUnauthorizedError(""))
		return
	}

	userID, ok := userIDVal.(uint64)
	if !ok {
		common.RespondError(c, common.NewUnauthorizedError(""))
		return
	}

	user, appErr := h.service.GetUserByID(userID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, UserToInfo(user))
}
