package users

import (
	"strconv"

	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Handler holds dependencies for user HTTP handlers.
type Handler struct {
	service *Service
}

// NewHandler creates a new user handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// CreateUser handles POST /users (admin only).
func (h *Handler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	resp, appErr := h.service.Create(req, actorID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, resp)
}

// GetUser handles GET /users/:id.
// Access is restricted to the requesting user viewing their own profile, or a SystemAdmin.
func (h *Handler) GetUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid user ID"))
		return
	}

	requesterID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))

	// Only self-access or SystemAdmin is permitted.
	isAdmin := false
	for _, r := range roles {
		if r == common.RoleSystemAdmin {
			isAdmin = true
			break
		}
	}
	if requesterID != userID && !isAdmin {
		common.RespondError(c, common.NewForbiddenError("you may only view your own profile"))
		return
	}

	resp, appErr := h.service.GetByID(userID, requesterID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, resp)
}

// UpdateUser handles PUT /users/:id.
func (h *Handler) UpdateUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid user ID"))
		return
	}

	// Object-level auth: users can only update their own profile unless admin
	requesterID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	if requesterID != userID && !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("you can only update your own profile"))
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	resp, appErr := h.service.Update(userID, req, requesterID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, resp)
}

// ListUsers handles GET /users.
func (h *Handler) ListUsers(c *gin.Context) {
	page, perPage := common.PaginationFromQuery(c)

	req := ListUsersRequest{
		Page:    page,
		PerPage: perPage,
		Role:    c.Query("role"),
		Search:  c.Query("search"),
	}

	if activeStr := c.Query("active"); activeStr != "" {
		active := activeStr == "true"
		req.Active = &active
	}

	users, total, appErr := h.service.List(req)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, users, meta)
}

// ToggleActive handles PATCH /users/:id/active (admin only).
func (h *Handler) ToggleActive(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid user ID"))
		return
	}

	var req ToggleActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	resp, appErr := h.service.ToggleActive(userID, req.IsActive, actorID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, resp)
}

// AssignRole handles POST /users/:id/roles (admin only).
func (h *Handler) AssignRole(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid user ID"))
		return
	}

	var req AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	appErr := h.service.AssignRole(userID, req.RoleName, actorID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, gin.H{"message": "role assigned successfully"})
}

// RemoveRole handles DELETE /users/:id/roles/:role (admin only).
func (h *Handler) RemoveRole(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid user ID"))
		return
	}

	roleName := c.Param("role")
	if roleName == "" {
		common.RespondError(c, common.NewBadRequestError("role name is required"))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	appErr := h.service.RemoveRole(userID, roleName, actorID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, gin.H{"message": "role removed successfully"})
}

// hasRole checks if a slice of role names contains the specified role.
func hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
