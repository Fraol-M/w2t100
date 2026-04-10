package tenants

import (
	"strconv"

	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Handler holds dependencies for tenant HTTP handlers.
type Handler struct {
	service *Service
}

// NewHandler creates a new tenant handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// CreateProfile handles POST /tenants.
func (h *Handler) CreateProfile(c *gin.Context) {
	var req CreateTenantProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	resp, appErr := h.service.CreateProfile(req, actorID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, resp)
}

// GetProfile handles GET /tenants/:id.
func (h *Handler) GetProfile(c *gin.Context) {
	profileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid tenant profile ID"))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))

	resp, appErr := h.service.GetByID(profileID, actorID, roles)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, resp)
}

// GetProfileByUser handles GET /tenants/by-user/:user_id.
func (h *Handler) GetProfileByUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid user ID"))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))

	resp, appErr := h.service.GetByUserID(userID, actorID, roles)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, resp)
}

// UpdateProfile handles PUT /tenants/:id.
func (h *Handler) UpdateProfile(c *gin.Context) {
	profileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid tenant profile ID"))
		return
	}

	var req UpdateTenantProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	resp, appErr := h.service.UpdateProfile(profileID, req, actorID, roles, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, resp)
}

// ListByProperty handles GET /tenants/by-property/:property_id.
func (h *Handler) ListByProperty(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("property_id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid property ID"))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))

	// SystemAdmin can list any property; PM must manage the property.
	if !hasRole(roles, common.RoleSystemAdmin) {
		if !h.service.IsPMForProperty(propertyID, actorID) {
			common.RespondError(c, common.NewForbiddenError("you do not manage this property"))
			return
		}
	}

	page, perPage := common.PaginationFromQuery(c)

	profiles, total, appErr := h.service.ListByProperty(propertyID, page, perPage)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, profiles, meta)
}
