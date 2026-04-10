package workorders

import (
	"encoding/json"
	"fmt"
	"time"

	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AuditLogger abstracts the audit logging dependency.
type AuditLogger interface {
	Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string)
}

// NotificationSender abstracts the notification sending dependency.
type NotificationSender interface {
	SendEvent(eventName string, recipientID uint64, data map[string]string) error
}

// allowedTransitions defines the state machine for work order statuses.
// Key = from status, Value = set of allowed to statuses.
var allowedTransitions = map[string]map[string]bool{
	common.WOStatusNew: {
		common.WOStatusAssigned: true,
	},
	common.WOStatusAssigned: {
		common.WOStatusInProgress: true,
	},
	common.WOStatusInProgress: {
		common.WOStatusAwaitingApproval: true,
	},
	common.WOStatusAwaitingApproval: {
		common.WOStatusCompleted:  true,
		common.WOStatusInProgress: true,
	},
	common.WOStatusCompleted: {
		common.WOStatusArchived: true,
	},
}

// Service handles work order business logic.
type Service struct {
	repo         *Repository
	dispatch     *DispatchService
	audit        AuditLogger
	notifier     NotificationSender
	db           *gorm.DB
	cfg          *config.Config
}

// NewService creates a new work order Service.
func NewService(repo *Repository, propQuerier PropertyQuerier, notifier NotificationSender, audit AuditLogger, db *gorm.DB, cfg *config.Config) *Service {
	return &Service{
		repo:     repo,
		dispatch: NewDispatchService(propQuerier),
		audit:    audit,
		notifier: notifier,
		db:       db,
		cfg:      cfg,
	}
}

// Create validates the input and creates a new work order.
// It calculates the SLA deadline based on priority and attempts auto-dispatch.
func (s *Service) Create(req CreateWorkOrderRequest, tenantID uint64, ip, requestID string) (*WorkOrder, *common.AppError) {
	// Validate required fields.
	var fieldErrors []*common.FieldError

	if fe := common.ValidateDescription(req.Description); fe != nil {
		fieldErrors = append(fieldErrors, fe)
	}
	if fe := common.ValidatePriority(req.Priority); fe != nil {
		fieldErrors = append(fieldErrors, fe)
	}

	// Validate optional date/time fields.
	var preferredDate *time.Time
	if req.PreferredAccessDate != "" {
		if fe := common.ValidateDateFormat("preferred_access_date", req.PreferredAccessDate); fe != nil {
			fieldErrors = append(fieldErrors, fe)
		} else {
			parsed, err := common.ParseDateMMDDYYYY(req.PreferredAccessDate)
			if err != nil {
				fieldErrors = append(fieldErrors, &common.FieldError{Field: "preferred_access_date", Message: "invalid date"})
			} else {
				preferredDate = &parsed
			}
		}
	}
	if req.PreferredAccessStartTime != "" {
		if fe := common.ValidateTimeFormat("preferred_access_start_time", req.PreferredAccessStartTime); fe != nil {
			fieldErrors = append(fieldErrors, fe)
		}
	}
	if req.PreferredAccessEndTime != "" {
		if fe := common.ValidateTimeFormat("preferred_access_end_time", req.PreferredAccessEndTime); fe != nil {
			fieldErrors = append(fieldErrors, fe)
		}
	}

	if appErr := common.CollectFieldErrors(fieldErrors...); appErr != nil {
		return nil, appErr
	}

	// Tenant isolation: if the tenant has an assigned unit, verify its property matches
	// the requested property_id. This prevents tenants from filing work orders against
	// properties they do not belong to.
	tenantPropertyID, err := s.repo.GetTenantPropertyID(tenantID)
	if err != nil {
		return nil, common.NewInternalError("failed to validate tenant property access")
	}
	if tenantPropertyID != 0 && tenantPropertyID != req.PropertyID {
		return nil, common.NewForbiddenError("you do not have access to the requested property")
	}

	// If a specific unit_id is provided, verify it belongs to the requested property.
	if req.UnitID != nil {
		unitValid, err := s.repo.ValidateUnitBelongsToProperty(*req.UnitID, req.PropertyID)
		if err != nil {
			return nil, common.NewInternalError("failed to validate unit")
		}
		if !unitValid {
			return nil, common.NewValidationError("the specified unit does not belong to this property")
		}
	}

	now := time.Now().UTC()
	slaDue := CalculateSLA(req.Priority, now)

	wo := &WorkOrder{
		UUID:                     uuid.New().String(),
		PropertyID:               req.PropertyID,
		UnitID:                   req.UnitID,
		TenantID:                 tenantID,
		Description:              req.Description,
		Priority:                 req.Priority,
		Status:                   common.WOStatusNew,
		IssueType:                req.IssueType,
		SkillTag:                 req.SkillTag,
		Tags:                     datatypes.JSON(req.Tags),
		PreferredAccessDate:      preferredDate,
		PreferredAccessStartTime: req.PreferredAccessStartTime,
		PreferredAccessEndTime:   req.PreferredAccessEndTime,
		SLADueAt:                 &slaDue,
	}

	if err := s.repo.Create(wo); err != nil {
		return nil, common.NewInternalError("failed to create work order")
	}

	// Create the initial event.
	s.createEvent(wo.ID, &tenantID, "created", "", common.WOStatusNew, "Work order created", nil)

	// Attempt auto-dispatch if skill tag is provided.
	if req.SkillTag != "" {
		result, err := s.dispatch.Dispatch(req.PropertyID, req.SkillTag)
		if err == nil && result.Assigned {
			wo.AssignedTo = &result.TechnicianID
			wo.Status = common.WOStatusAssigned
			if err := s.repo.Update(wo); err == nil {
				s.createEvent(wo.ID, &tenantID, "auto_dispatched", common.WOStatusNew, common.WOStatusAssigned,
					fmt.Sprintf("Auto-dispatched to technician %d", result.TechnicianID), nil)
				// Notify the assigned technician.
				if s.notifier != nil {
					_ = s.notifier.SendEvent("work_order_assigned", result.TechnicianID, map[string]string{
						"work_order_id": fmt.Sprintf("%d", wo.ID),
						"priority":      wo.Priority,
					})
				}
			}
		} else if err == nil && !result.Assigned {
			s.createEvent(wo.ID, &tenantID, "dispatch_failed", "", "", result.Message, nil)
		}
	}

	// Audit log.
	s.audit.Log(tenantID, common.AuditActionCreate, "WorkOrder", wo.ID, "Work order created", ip, requestID)

	return wo, nil
}

// Delete hard-deletes a work order by ID. Used only to roll back a failed multipart create.
func (s *Service) Delete(woID uint64) error {
	return s.db.Delete(&WorkOrder{}, woID).Error
}

// IsManagedBy checks whether managerID is an active staff member on the given property.
func (s *Service) IsManagedBy(propertyID, managerID uint64) (bool, error) {
	return s.repo.IsManagedBy(propertyID, managerID)
}

// GetManagedPropertyIDs returns all property IDs the user is assigned to manage.
func (s *Service) GetManagedPropertyIDs(userID uint64) ([]uint64, error) {
	return s.repo.GetManagedPropertyIDs(userID)
}

// GetByID retrieves a work order by its ID.
func (s *Service) GetByID(id uint64) (*WorkOrder, *common.AppError) {
	wo, err := s.repo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, common.NewNotFoundError("Work order")
		}
		return nil, common.NewInternalError("failed to retrieve work order")
	}
	return wo, nil
}

// GetByUUID retrieves a work order by its UUID.
func (s *Service) GetByUUID(uuid string) (*WorkOrder, *common.AppError) {
	wo, err := s.repo.FindByUUID(uuid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, common.NewNotFoundError("Work order")
		}
		return nil, common.NewInternalError("failed to retrieve work order")
	}
	return wo, nil
}

// List retrieves a paginated list of work orders matching the given filters.
func (s *Service) List(req WorkOrderListRequest) ([]WorkOrder, int64, *common.AppError) {
	page, perPage := req.Page, req.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	filters := ListFilters{
		PropertyID:          req.PropertyID,
		PropertyIDs:         req.PropertyIDs,
		ScopedToPropertyIDs: req.ScopedToPropertyIDs,
		TenantID:            req.TenantID,
		Status:              req.Status,
		AssignedTo:          req.AssignedTo,
		Priority:            req.Priority,
	}

	if req.DateFrom != "" {
		if t, err := common.ParseDateMMDDYYYY(req.DateFrom); err == nil {
			filters.DateFrom = &t
		}
	}
	if req.DateTo != "" {
		if t, err := common.ParseDateMMDDYYYY(req.DateTo); err == nil {
			endOfDay := t.Add(24*time.Hour - time.Second)
			filters.DateTo = &endOfDay
		}
	}

	offset := (page - 1) * perPage
	orders, total, err := s.repo.List(filters, offset, perPage)
	if err != nil {
		return nil, 0, common.NewInternalError("failed to list work orders")
	}

	return orders, total, nil
}

// Transition validates and applies a status transition on a work order.
func (s *Service) Transition(woID uint64, req TransitionRequest, actorID uint64, ip, requestID string) (*WorkOrder, *common.AppError) {
	wo, appErr := s.GetByID(woID)
	if appErr != nil {
		return nil, appErr
	}

	if !IsValidTransition(wo.Status, req.ToStatus) {
		return nil, common.NewValidationError(
			fmt.Sprintf("invalid transition from %s to %s", wo.Status, req.ToStatus),
		)
	}

	fromStatus := wo.Status
	wo.Status = req.ToStatus
	now := time.Now().UTC()

	// Set timestamps for terminal states.
	if req.ToStatus == common.WOStatusCompleted {
		wo.CompletedAt = &now
	}
	if req.ToStatus == common.WOStatusArchived {
		wo.ArchivedAt = &now
	}

	if err := s.repo.Update(wo); err != nil {
		return nil, common.NewInternalError("failed to update work order status")
	}

	description := fmt.Sprintf("Status changed from %s to %s", fromStatus, req.ToStatus)
	if req.Notes != "" {
		description += ": " + req.Notes
	}
	s.createEvent(wo.ID, &actorID, "status_change", fromStatus, req.ToStatus, description, nil)

	s.audit.Log(actorID, common.AuditActionStatusChange, "WorkOrder", wo.ID, description, ip, requestID)

	return wo, nil
}

// Dispatch explicitly assigns a New work order to a technician.
// This endpoint exists for PropertyManagers and SystemAdmins to handle work orders
// that were not auto-dispatched (e.g. created without a skill_tag).
// The work order must be in New status; use Reassign for Assigned/InProgress orders.
func (s *Service) Dispatch(woID uint64, req DispatchRequest, actorID uint64, ip, requestID string) (*WorkOrder, *common.AppError) {
	wo, appErr := s.GetByID(woID)
	if appErr != nil {
		return nil, appErr
	}

	if wo.Status != common.WOStatusNew {
		return nil, common.NewValidationError("work order must be in New status to dispatch; use reassign for Assigned or InProgress orders")
	}

	wo.AssignedTo = &req.TechnicianID
	wo.Status = common.WOStatusAssigned

	if err := s.repo.Update(wo); err != nil {
		return nil, common.NewInternalError("failed to dispatch work order")
	}

	notes := req.Reason
	if notes == "" {
		notes = fmt.Sprintf("Manually dispatched to technician %d", req.TechnicianID)
	}
	s.createEvent(wo.ID, &actorID, "dispatched", common.WOStatusNew, common.WOStatusAssigned, notes, nil)

	if s.notifier != nil {
		_ = s.notifier.SendEvent("work_order_assigned", req.TechnicianID, map[string]string{
			"work_order_id": fmt.Sprintf("%d", wo.ID),
			"priority":      wo.Priority,
		})
	}

	s.audit.Log(actorID, common.AuditActionStatusChange, "WorkOrder", wo.ID,
		fmt.Sprintf("Dispatched to technician %d", req.TechnicianID), ip, requestID)

	return wo, nil
}

// Reassign reassigns a work order to a different technician with a required reason.
func (s *Service) Reassign(woID uint64, req ReassignRequest, actorID uint64, ip, requestID string) (*WorkOrder, *common.AppError) {
	// Validate reason length.
	if fe := common.ValidateReassignReason(req.Reason); fe != nil {
		return nil, common.NewValidationError("Validation failed", *fe)
	}

	wo, appErr := s.GetByID(woID)
	if appErr != nil {
		return nil, appErr
	}

	// Work order must be in Assigned or InProgress to reassign.
	if wo.Status != common.WOStatusAssigned && wo.Status != common.WOStatusInProgress {
		return nil, common.NewValidationError("work order must be in Assigned or InProgress status to reassign")
	}

	previousAssignee := wo.AssignedTo
	wo.AssignedTo = &req.TechnicianID
	wo.Status = common.WOStatusAssigned

	if err := s.repo.Update(wo); err != nil {
		return nil, common.NewInternalError("failed to reassign work order")
	}

	metadata := map[string]interface{}{
		"previous_assignee": previousAssignee,
		"new_assignee":      req.TechnicianID,
		"reason":            req.Reason,
	}
	metaJSON, _ := json.Marshal(metadata)

	s.createEvent(wo.ID, &actorID, "reassigned", "", common.WOStatusAssigned,
		fmt.Sprintf("Reassigned to technician %d: %s", req.TechnicianID, req.Reason),
		metaJSON)

	s.audit.Log(actorID, common.AuditActionUpdate, "WorkOrder", wo.ID,
		fmt.Sprintf("Reassigned to technician %d: %s", req.TechnicianID, req.Reason), ip, requestID)

	// Notify new assignee.
	if s.notifier != nil {
		_ = s.notifier.SendEvent("work_order_assigned", req.TechnicianID, map[string]string{
			"work_order_id": fmt.Sprintf("%d", wo.ID),
			"priority":      wo.Priority,
		})
	}

	return wo, nil
}

// AddCostItem validates and adds a cost item to a work order.
func (s *Service) AddCostItem(woID uint64, req AddCostItemRequest, actorID uint64, ip, requestID string) (*CostItem, *common.AppError) {
	// Validate fields.
	var fieldErrors []*common.FieldError
	if fe := common.ValidateCostType(req.CostType); fe != nil {
		fieldErrors = append(fieldErrors, fe)
	}
	if fe := common.ValidateResponsibility(req.Responsibility); fe != nil {
		fieldErrors = append(fieldErrors, fe)
	}
	if fe := common.ValidateMoneyAmount("amount", req.Amount); fe != nil {
		fieldErrors = append(fieldErrors, fe)
	}
	if req.Description == "" {
		fieldErrors = append(fieldErrors, &common.FieldError{Field: "description", Message: "is required"})
	}
	if appErr := common.CollectFieldErrors(fieldErrors...); appErr != nil {
		return nil, appErr
	}

	// Verify work order exists.
	if _, appErr := s.GetByID(woID); appErr != nil {
		return nil, appErr
	}

	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}

	item := &CostItem{
		UUID:           uuid.New().String(),
		WorkOrderID:    woID,
		CostType:       req.CostType,
		Description:    req.Description,
		Amount:         req.Amount,
		Responsibility: req.Responsibility,
		Notes:          notes,
		CreatedBy:      actorID,
	}

	if err := s.repo.CreateCostItem(item); err != nil {
		return nil, common.NewInternalError("failed to add cost item")
	}

	s.createEvent(woID, &actorID, "cost_item_added", "", "",
		fmt.Sprintf("Cost item added: %s $%.2f", req.CostType, req.Amount), nil)

	s.audit.Log(actorID, common.AuditActionCreate, "CostItem", item.ID,
		fmt.Sprintf("Cost item added to work order %d", woID), ip, requestID)

	return item, nil
}

// Rate allows a tenant to rate a completed work order.
func (s *Service) Rate(woID uint64, req RateWorkOrderRequest, tenantID uint64, ip, requestID string) (*WorkOrder, *common.AppError) {
	// Validate rating.
	var fieldErrors []*common.FieldError
	if fe := common.ValidateRating(req.Rating); fe != nil {
		fieldErrors = append(fieldErrors, fe)
	}
	if req.Feedback != "" {
		if fe := common.ValidateFeedback(req.Feedback); fe != nil {
			fieldErrors = append(fieldErrors, fe)
		}
	}
	if appErr := common.CollectFieldErrors(fieldErrors...); appErr != nil {
		return nil, appErr
	}

	wo, appErr := s.GetByID(woID)
	if appErr != nil {
		return nil, appErr
	}

	// Only the tenant who created the work order can rate it.
	if wo.TenantID != tenantID {
		return nil, common.NewForbiddenError("only the work order tenant can rate it")
	}

	// Only completed work orders can be rated.
	if wo.Status != common.WOStatusCompleted {
		return nil, common.NewValidationError("only completed work orders can be rated")
	}

	// Prevent re-rating.
	if wo.Rating != nil {
		return nil, common.NewConflictError("work order has already been rated")
	}

	rating := uint8(req.Rating)
	wo.Rating = &rating
	if req.Feedback != "" {
		wo.Feedback = &req.Feedback
	}

	if err := s.repo.Update(wo); err != nil {
		return nil, common.NewInternalError("failed to rate work order")
	}

	s.createEvent(wo.ID, &tenantID, "rated", "", "",
		fmt.Sprintf("Rated %d/5", req.Rating), nil)

	s.audit.Log(tenantID, common.AuditActionUpdate, "WorkOrder", wo.ID,
		fmt.Sprintf("Work order rated %d/5", req.Rating), ip, requestID)

	return wo, nil
}

// ListEvents retrieves all events for a work order.
func (s *Service) ListEvents(woID uint64) ([]WorkOrderEvent, *common.AppError) {
	// Verify work order exists.
	if _, appErr := s.GetByID(woID); appErr != nil {
		return nil, appErr
	}
	events, err := s.repo.ListEvents(woID)
	if err != nil {
		return nil, common.NewInternalError("failed to list events")
	}
	return events, nil
}

// ListCostItems retrieves all cost items for a work order.
func (s *Service) ListCostItems(woID uint64) ([]CostItem, *common.AppError) {
	// Verify work order exists.
	if _, appErr := s.GetByID(woID); appErr != nil {
		return nil, appErr
	}
	items, err := s.repo.ListCostItems(woID)
	if err != nil {
		return nil, common.NewInternalError("failed to list cost items")
	}
	return items, nil
}

// createEvent is a helper that creates a work order event record.
func (s *Service) createEvent(woID uint64, actorID *uint64, eventType, from, to, description string, metadata []byte) {
	event := &WorkOrderEvent{
		WorkOrderID: woID,
		ActorID:     actorID,
		EventType:   eventType,
		FromStatus:  from,
		ToStatus:    to,
		Description: description,
		Metadata:    datatypes.JSON(metadata),
	}
	_ = s.repo.CreateEvent(event)
}

// IsValidTransition checks whether a status transition is allowed by the state machine.
func IsValidTransition(from, to string) bool {
	targets, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	return targets[to]
}

// CalculateSLA computes the SLA due time based on priority and the given start time.
func CalculateSLA(priority string, from time.Time) time.Time {
	switch priority {
	case common.PriorityEmergency:
		return from.Add(4 * time.Hour)
	case common.PriorityHigh:
		return from.Add(24 * time.Hour)
	case common.PriorityNormal:
		return from.Add(72 * time.Hour)
	case common.PriorityLow:
		return addBusinessDays(from, 5)
	default:
		return from.Add(72 * time.Hour)
	}
}

// addBusinessDays adds the specified number of business days (Mon-Fri) to the given time.
func addBusinessDays(from time.Time, days int) time.Time {
	added := 0
	current := from
	for added < days {
		current = current.Add(24 * time.Hour)
		weekday := current.Weekday()
		if weekday != time.Saturday && weekday != time.Sunday {
			added++
		}
	}
	return current
}
