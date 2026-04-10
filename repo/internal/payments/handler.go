package payments

import (
	"strconv"
	"time"

	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// Handler holds HTTP handlers for payment endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new payment Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// CreateIntent handles POST /payments/intents
func (h *Handler) CreateIntent(c *gin.Context) {
	var req CreateIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RolePropertyManager) && !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only property managers or system admins can create payment intents"))
		return
	}

	if appErr := h.enforcePMScope(c, req.PropertyID); appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	payment, appErr := h.service.CreateIntent(req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToPaymentResponse(payment))
}

// ListPayments handles GET /payments
func (h *Handler) ListPayments(c *gin.Context) {
	var req ListPaymentsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid query parameters"))
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.PerPage == 0 {
		req.PerPage = 20
	}

	// Scope PropertyManager to their managed properties.
	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	if hasRole(roles, common.RolePropertyManager) && !hasRole(roles, common.RoleSystemAdmin) {
		managedIDs, err := h.service.GetManagedPropertyIDs(userID)
		if err != nil {
			common.RespondError(c, common.NewInternalError("failed to determine managed properties"))
			return
		}
		if req.PropertyID != nil {
			// Client supplied a specific property_id — verify it is within the managed set.
			// Without this check a PM can bypass their scope by passing any arbitrary property_id.
			allowed := false
			for _, id := range managedIDs {
				if id == *req.PropertyID {
					allowed = true
					break
				}
			}
			if !allowed {
				common.RespondError(c, common.NewForbiddenError("you do not manage the requested property"))
				return
			}
			// Validated: keep PropertyID as the precise filter; managed-scope list not needed.
		} else {
			req.PropertyIDs = managedIDs
			req.ScopedToPropertyIDs = true // empty list must return zero results, not all records
		}
	}

	payments, total, appErr := h.service.ListPayments(req)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	responses := make([]PaymentResponse, len(payments))
	for i := range payments {
		responses[i] = ToPaymentResponse(&payments[i])
	}

	meta := common.BuildMeta(req.Page, req.PerPage, total)
	common.SuccessWithMeta(c, responses, meta)
}

// GetPayment handles GET /payments/:id
func (h *Handler) GetPayment(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid payment ID"))
		return
	}

	payment, appErr := h.service.GetPayment(id)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	// PropertyManager may only view payments for properties they manage.
	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	if hasRole(roles, common.RolePropertyManager) && !hasRole(roles, common.RoleSystemAdmin) {
		managed, err := h.service.IsManagedBy(payment.PropertyID, userID)
		if err != nil || !managed {
			common.RespondError(c, common.NewForbiddenError("you do not manage the property for this payment"))
			return
		}
	}

	common.Success(c, ToPaymentResponse(payment))
}

// MarkPaid handles POST /payments/:id/mark-paid
func (h *Handler) MarkPaid(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid payment ID"))
		return
	}

	var req MarkPaidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// MarkPaidRequest has optional fields; allow empty body.
		req = MarkPaidRequest{}
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RolePropertyManager) && !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only property managers or system admins can mark payments as paid"))
		return
	}

	existing, appErr := h.service.GetPayment(id)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}
	if scopeErr := h.enforcePMScope(c, existing.PropertyID); scopeErr != nil {
		common.RespondError(c, scopeErr)
		return
	}

	payment, appErr := h.service.MarkPaid(id, req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, ToPaymentResponse(payment))
}

// ApprovePayment handles POST /payments/:id/approve
func (h *Handler) ApprovePayment(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid payment ID"))
		return
	}

	var req ApprovePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = ApprovePaymentRequest{}
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RolePropertyManager) && !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only property managers or system admins can approve payments"))
		return
	}

	existing, appErr := h.service.GetPayment(id)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}
	if scopeErr := h.enforcePMScope(c, existing.PropertyID); scopeErr != nil {
		common.RespondError(c, scopeErr)
		return
	}

	approval, appErr := h.service.ApprovePayment(id, req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToPaymentApprovalResponse(approval))
}

// CreateSettlement handles POST /payments/settlements
func (h *Handler) CreateSettlement(c *gin.Context) {
	var req CreateSettlementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RolePropertyManager) && !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only property managers or system admins can create settlements"))
		return
	}

	linkedPayment, appErr := h.service.GetPayment(req.PaymentID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}
	if scopeErr := h.enforcePMScope(c, linkedPayment.PropertyID); scopeErr != nil {
		common.RespondError(c, scopeErr)
		return
	}

	settlement, appErr := h.service.CreateSettlement(req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToPaymentResponse(settlement))
}

// CreateReversal handles POST /payments/:id/reverse
func (h *Handler) CreateReversal(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid payment ID"))
		return
	}

	var req CreateReversalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RolePropertyManager) && !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only property managers or system admins can reverse payments"))
		return
	}

	existing, appErr := h.service.GetPayment(id)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}
	if scopeErr := h.enforcePMScope(c, existing.PropertyID); scopeErr != nil {
		common.RespondError(c, scopeErr)
		return
	}

	reversal, appErr := h.service.CreateReversal(id, req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToPaymentResponse(reversal))
}

// CreateMakeup handles POST /payments/:id/makeup
func (h *Handler) CreateMakeup(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid payment ID"))
		return
	}

	var req CreateMakeupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid request body"))
		return
	}

	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	ip := c.ClientIP()
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqID, _ := requestID.(string)

	if !hasRole(roles, common.RolePropertyManager) && !hasRole(roles, common.RoleSystemAdmin) {
		common.RespondError(c, common.NewForbiddenError("only property managers or system admins can create makeup postings"))
		return
	}

	existing, appErr := h.service.GetPayment(id)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}
	if scopeErr := h.enforcePMScope(c, existing.PropertyID); scopeErr != nil {
		common.RespondError(c, scopeErr)
		return
	}

	makeup, appErr := h.service.CreateMakeup(id, req, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToPaymentResponse(makeup))
}

// RunReconciliation handles POST /payments/reconciliation/run
func (h *Handler) RunReconciliation(c *gin.Context) {
	var req RunReconciliationRequest
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
		common.RespondError(c, common.NewForbiddenError("only system admins can run reconciliation"))
		return
	}

	runDate, err := time.Parse("2006-01-02", req.RunDate)
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("run_date must be in YYYY-MM-DD format"))
		return
	}

	run, appErr := h.service.RunReconciliation(runDate, userID, ip, reqID)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Created(c, ToReconciliationResponse(run))
}

// ListReconciliations handles GET /payments/reconciliation
func (h *Handler) ListReconciliations(c *gin.Context) {
	page, perPage := common.PaginationFromQuery(c)

	runs, total, appErr := h.service.ListReconciliations(page, perPage)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	responses := make([]ReconciliationResponse, len(runs))
	for i := range runs {
		responses[i] = ToReconciliationResponse(&runs[i])
	}

	meta := common.BuildMeta(page, perPage, total)
	common.SuccessWithMeta(c, responses, meta)
}

// GetReconciliation handles GET /payments/reconciliation/:id
func (h *Handler) GetReconciliation(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		common.RespondError(c, common.NewBadRequestError("invalid reconciliation ID"))
		return
	}

	run, appErr := h.service.GetReconciliation(id)
	if appErr != nil {
		common.RespondError(c, appErr)
		return
	}

	common.Success(c, ToReconciliationResponse(run))
}

// --- Utility helpers ---

// enforcePMScope returns a forbidden error if the caller is a PropertyManager
// who does not manage the given propertyID. SystemAdmins always pass.
func (h *Handler) enforcePMScope(c *gin.Context, propertyID uint64) *common.AppError {
	userID := c.GetUint64(string(common.CtxKeyUserID))
	roles := c.GetStringSlice(string(common.CtxKeyRoles))
	if hasRole(roles, common.RolePropertyManager) && !hasRole(roles, common.RoleSystemAdmin) {
		managed, err := h.service.IsManagedBy(propertyID, userID)
		if err != nil || !managed {
			return common.NewForbiddenError("you do not manage the property for this payment")
		}
	}
	return nil
}

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
