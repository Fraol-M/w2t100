package properties

import (
	"strconv"

	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Handler holds dependencies for property HTTP handlers.
type Handler struct {
	service *Service
}

// NewHandler creates a new property handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// --- Property handlers ---

// CreateProperty handles POST /properties.
func (h *Handler) CreateProperty(c *gin.Context) {
	var req CreatePropertyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	resp, appErr := h.service.CreateProperty(req, actorID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, resp)
}

// requirePMManagesProperty is a helper that enforces property ownership for PMs.
// Returns true when access should proceed; false when a 403 has already been sent.
func (h *Handler) requirePMManagesProperty(c *gin.Context, propertyID uint64) bool {
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	if hasRole(roles, common.RoleSystemAdmin) {
		return true
	}
	actorID := c.GetUint64(string(common.CtxKeyUserID))
	ok, appErr := h.service.CanManageProperty(actorID, roles, propertyID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return false
	}
	if !ok {
		common.RespondError(c, common.NewForbiddenError("you do not manage this property"))
		return false
	}
	return true
}

// GetProperty handles GET /properties/:id.
func (h *Handler) GetProperty(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid property ID"))
		return
	}

	if !h.requirePMManagesProperty(c, propertyID) {
		return
	}

	resp, appErr := h.service.GetPropertyByID(propertyID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, resp)
}

// UpdateProperty handles PUT /properties/:id.
func (h *Handler) UpdateProperty(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid property ID"))
		return
	}

	var req UpdatePropertyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	resp, appErr := h.service.UpdateProperty(propertyID, req, actorID, roles, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, resp)
}

// ListProperties handles GET /properties.
func (h *Handler) ListProperties(c *gin.Context) {
	page, perPage := common.PaginationFromQuery(c)

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))

	var managerID *uint64
	if v := c.Query("manager_id"); v != "" {
		if mid, err := strconv.ParseUint(v, 10, 64); err == nil {
			managerID = &mid
		}
	}

	// PropertyManagers may only list properties they manage — ignore any
	// manager_id query param and force the scope to the caller's own ID.
	if hasRole(roles, common.RolePropertyManager) && !hasRole(roles, common.RoleSystemAdmin) {
		managerID = &actorID
	}

	var active *bool
	if v := c.Query("active"); v != "" {
		a := v == "true"
		active = &a
	}

	properties, total, appErr := h.service.ListProperties(page, perPage, managerID, active)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, properties, meta)
}

// --- Unit handlers ---

// CreateUnit handles POST /properties/:id/units.
func (h *Handler) CreateUnit(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid property ID"))
		return
	}

	var req CreateUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	resp, appErr := h.service.CreateUnit(propertyID, req, actorID, roles, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, resp)
}

// GetUnit handles GET /properties/:id/units/:unit_id.
func (h *Handler) GetUnit(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid property ID"))
		return
	}
	unitID, err := strconv.ParseUint(c.Param("unit_id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid unit ID"))
		return
	}

	if !h.requirePMManagesProperty(c, propertyID) {
		return
	}

	resp, appErr := h.service.GetUnitByID(unitID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, resp)
}

// UpdateUnit handles PUT /properties/:id/units/:unit_id.
func (h *Handler) UpdateUnit(c *gin.Context) {
	unitID, err := strconv.ParseUint(c.Param("unit_id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid unit ID"))
		return
	}

	var req UpdateUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	resp, appErr := h.service.UpdateUnit(unitID, req, actorID, roles, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, resp)
}

// ListUnits handles GET /properties/:id/units.
func (h *Handler) ListUnits(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid property ID"))
		return
	}

	if !h.requirePMManagesProperty(c, propertyID) {
		return
	}

	page, perPage := common.PaginationFromQuery(c)

	units, total, appErr := h.service.ListUnitsByProperty(propertyID, page, perPage)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, units, meta)
}

// --- Staff Assignment handlers ---

// AssignStaff handles POST /properties/:id/staff.
func (h *Handler) AssignStaff(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid property ID"))
		return
	}

	var req AssignStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	resp, appErr := h.service.AssignStaff(propertyID, req, actorID, roles, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, resp)
}

// RemoveStaff handles DELETE /properties/:id/staff/:user_id/:role.
func (h *Handler) RemoveStaff(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid property ID"))
		return
	}

	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid user ID"))
		return
	}

	role := c.Param("role")
	if role == "" {
		common.RespondError(c, common.NewBadRequestError("role is required"))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	appErr := h.service.RemoveStaff(propertyID, userID, role, actorID, roles, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, gin.H{"message": "staff removed successfully"})
}

// ListStaff handles GET /properties/:id/staff.
func (h *Handler) ListStaff(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid property ID"))
		return
	}

	if !h.requirePMManagesProperty(c, propertyID) {
		return
	}

	assignments, appErr := h.service.ListStaffByProperty(propertyID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, assignments)
}

// --- Skill Tag handlers ---

// AddSkillTag handles POST /properties/technicians/:user_id/skills.
func (h *Handler) AddSkillTag(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid user ID"))
		return
	}

	var req AddSkillTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("Invalid request body: "+err.Error()))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	resp, appErr := h.service.AddSkillTag(userID, req, actorID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, resp)
}

// RemoveSkillTag handles DELETE /properties/technicians/:user_id/skills/:tag.
func (h *Handler) RemoveSkillTag(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid user ID"))
		return
	}

	tag := c.Param("tag")
	if tag == "" {
		common.RespondError(c, common.NewBadRequestError("tag name is required"))
		return
	}

	actorID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	appErr := h.service.RemoveSkillTag(userID, tag, actorID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, gin.H{"message": "skill tag removed successfully"})
}

// ListSkillTags handles GET /properties/technicians/:user_id/skills.
func (h *Handler) ListSkillTags(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid user ID"))
		return
	}

	tags, appErr := h.service.ListSkillTagsByUser(userID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, tags)
}
