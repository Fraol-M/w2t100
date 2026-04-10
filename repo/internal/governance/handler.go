package governance

import (
	"mime/multipart"
	"strconv"

	"propertyops/backend/internal/attachments"
	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// EvidenceUploader is the subset of attachments.Service used for evidence uploads.
type EvidenceUploader interface {
	UploadEvidence(entityType string, entityID uint64, file *multipart.FileHeader, uploaderID uint64, ip, requestID string) (*attachments.Attachment, *common.AppError)
	FindByEntity(entityType string, entityID uint64) ([]attachments.Attachment, *common.AppError)
}

// Handler holds HTTP handlers for governance endpoints.
type Handler struct {
	service  *Service
	evidence EvidenceUploader
}

// NewHandler creates a new governance Handler.
func NewHandler(service *Service, evidence EvidenceUploader) *Handler {
	return &Handler{service: service, evidence: evidence}
}

// --- Report handlers ---

// CreateReport handles POST /governance/reports
func (h *Handler) CreateReport(c *gin.Context) {
	var req CreateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	report, appErr := h.service.CreateReport(req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToReportResponse(report))
}

// ListReports handles GET /governance/reports
func (h *Handler) ListReports(c *gin.Context) {
	page, perPage := common.PaginationFromQuery(c)
	status := c.Query("status")
	targetType := c.Query("target_type")

	var targetID *uint64
	if v := c.Query("target_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			targetID = &id
		}
	}

	reports, total, appErr := h.service.ListReports(status, targetType, targetID, page, perPage)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	responses := make([]ReportResponse, len(reports))
	for i := range reports {
		responses[i] = ToReportResponse(&reports[i])
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, responses, meta)
}

// GetReport handles GET /governance/reports/:id
func (h *Handler) GetReport(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid report ID"))
		return
	}

	report, appErr := h.service.GetReport(id)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, ToReportResponse(report))
}

// ReviewReport handles PATCH /governance/reports/:id/review
func (h *Handler) ReviewReport(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid report ID"))
		return
	}

	var req ReviewReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RoleComplianceReviewer) && !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only compliance reviewers or system admins can review reports"))
		return
	}

	report, appErr := h.service.ReviewReport(id, req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, ToReportResponse(report))
}

// --- Enforcement handlers ---

// CreateEnforcement handles POST /governance/enforcements
func (h *Handler) CreateEnforcement(c *gin.Context) {
	var req CreateEnforcementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RoleComplianceReviewer) && !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only compliance reviewers or system admins can apply enforcement"))
		return
	}

	action, appErr := h.service.ApplyEnforcement(req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToEnforcementResponse(action))
}

// ListEnforcements handles GET /governance/enforcements
func (h *Handler) ListEnforcements(c *gin.Context) {
	page, perPage := common.PaginationFromQuery(c)

	actions, total, appErr := h.service.ListEnforcements(page, perPage)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	responses := make([]EnforcementResponse, len(actions))
	for i := range actions {
		responses[i] = ToEnforcementResponse(&actions[i])
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, responses, meta)
}

// RevokeEnforcement handles POST /governance/enforcements/:id/revoke
func (h *Handler) RevokeEnforcement(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid enforcement ID"))
		return
	}

	var req RevokeEnforcementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RoleComplianceReviewer) && !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only compliance reviewers or system admins can revoke enforcement"))
		return
	}

	action, appErr := h.service.RevokeEnforcement(id, req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, ToEnforcementResponse(action))
}

// --- Keyword handlers ---

// CreateKeyword handles POST /governance/keywords
func (h *Handler) CreateKeyword(c *gin.Context) {
	var req KeywordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only system admins can manage keywords"))
		return
	}

	keyword, appErr := h.service.CreateKeyword(req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToKeywordResponse(keyword))
}

// ListKeywords handles GET /governance/keywords
func (h *Handler) ListKeywords(c *gin.Context) {
	page, perPage := common.PaginationFromQuery(c)

	keywords, total, appErr := h.service.ListKeywords(page, perPage)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	responses := make([]KeywordResponse, len(keywords))
	for i := range keywords {
		responses[i] = ToKeywordResponse(&keywords[i])
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, responses, meta)
}

// DeleteKeyword handles DELETE /governance/keywords/:id
func (h *Handler) DeleteKeyword(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid keyword ID"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only system admins can manage keywords"))
		return
	}

	if appErr := h.service.DeleteKeyword(id, userID, ip, reqID); appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.NoContent(c)
}

// --- Risk rule handlers ---

// CreateRiskRule handles POST /governance/risk-rules
func (h *Handler) CreateRiskRule(c *gin.Context) {
	var req RiskRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only system admins can manage risk rules"))
		return
	}

	rule, appErr := h.service.CreateRiskRule(req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToRiskRuleResponse(rule))
}

// GetRiskRule handles GET /governance/risk-rules/:id
func (h *Handler) GetRiskRule(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid risk rule ID"))
		return
	}

	rule, appErr := h.service.GetRiskRule(id)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, ToRiskRuleResponse(rule))
}

// ListRiskRules handles GET /governance/risk-rules
func (h *Handler) ListRiskRules(c *gin.Context) {
	page, perPage := common.PaginationFromQuery(c)

	rules, total, appErr := h.service.ListRiskRules(page, perPage)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	responses := make([]RiskRuleResponse, len(rules))
	for i := range rules {
		responses[i] = ToRiskRuleResponse(&rules[i])
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, responses, meta)
}

// UpdateRiskRule handles PUT /governance/risk-rules/:id
func (h *Handler) UpdateRiskRule(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid risk rule ID"))
		return
	}

	var req RiskRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only system admins can manage risk rules"))
		return
	}

	rule, appErr := h.service.UpdateRiskRule(id, req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, ToRiskRuleResponse(rule))
}

// DeleteRiskRule handles DELETE /governance/risk-rules/:id
func (h *Handler) DeleteRiskRule(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid risk rule ID"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only system admins can manage risk rules"))
		return
	}

	if appErr := h.service.DeleteRiskRule(id, userID, ip, reqID); appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.NoContent(c)
}

// --- Evidence attachment handlers ---

// canAccessReportEvidence returns true if the actor is the original reporter,
// a ComplianceReviewer, or a SystemAdmin.
func (h *Handler) canAccessReportEvidence(c *gin.Context, reportID uint64) bool {
	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))

	for _, r := range roles {
		if r == common.RoleComplianceReviewer || r == common.RoleSystemAdmin {
			return true
		}
	}

	// Check if the caller is the reporter who filed this report.
	report, appErr := h.service.GetReport(reportID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return false
	}
	if report.ReporterID != userID {
		common.RespondError(c, common.NewForbiddenError("only the reporter, compliance reviewers, or admins may access this report's evidence"))
		return false
	}
	return true
}

// UploadEvidence handles POST /governance/reports/:id/evidence.
// The original reporter, ComplianceReviewers, and SystemAdmins may attach evidence.
func (h *Handler) UploadEvidence(c *gin.Context) {
	reportID, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid report ID"))
		return
	}

	if !h.canAccessReportEvidence(c, reportID) {
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	file, fileErr := c.FormFile("file")
	if fileErr != nil {
		common.RespondError(c, common.NewBadRequestError("file is required"))
		return
	}

	if h.evidence == nil {
		common.RespondError(c, common.NewInternalError("evidence upload not configured"))
		return
	}

	attachment, appErr := h.evidence.UploadEvidence("Report", reportID, file, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, attachment)
}

// ListEvidence handles GET /governance/reports/:id/evidence.
// The original reporter, ComplianceReviewers, and SystemAdmins may list evidence.
func (h *Handler) ListEvidence(c *gin.Context) {
	reportID, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid report ID"))
		return
	}

	if !h.canAccessReportEvidence(c, reportID) {
		return
	}

	if h.evidence == nil {
		common.Success(c, []interface{}{})
		return
	}

	attachments, appErr := h.evidence.FindByEntity("Report", reportID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, attachments)
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
