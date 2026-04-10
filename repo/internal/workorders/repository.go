package workorders

import (
	"time"

	"gorm.io/gorm"
)

// Repository handles all work order database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new work order Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new work order record.
func (r *Repository) Create(wo *WorkOrder) error {
	return r.db.Create(wo).Error
}

// FindByID loads a work order by its primary key.
func (r *Repository) FindByID(id uint64) (*WorkOrder, error) {
	var wo WorkOrder
	err := r.db.Where("id = ?", id).First(&wo).Error
	if err != nil {
		return nil, err
	}
	return &wo, nil
}

// FindByUUID loads a work order by its UUID.
func (r *Repository) FindByUUID(uuid string) (*WorkOrder, error) {
	var wo WorkOrder
	err := r.db.Where("uuid = ?", uuid).First(&wo).Error
	if err != nil {
		return nil, err
	}
	return &wo, nil
}

// Update saves changes to an existing work order.
func (r *Repository) Update(wo *WorkOrder) error {
	return r.db.Save(wo).Error
}

// ListFilters holds the optional filters for listing work orders.
type ListFilters struct {
	PropertyID          *uint64
	PropertyIDs         []uint64 // for multi-property scoping (PM access control)
	ScopedToPropertyIDs bool     // when true, PropertyIDs is an authoritative allow-list (empty = no results)
	Status              string
	AssignedTo          *uint64
	TenantID            *uint64
	Priority            string
	DateFrom            *time.Time
	DateTo              *time.Time
}

// List retrieves work orders matching the given filters with pagination.
func (r *Repository) List(filters ListFilters, offset, limit int) ([]WorkOrder, int64, error) {
	query := r.db.Model(&WorkOrder{})

	if filters.PropertyID != nil {
		query = query.Where("property_id = ?", *filters.PropertyID)
	} else if filters.ScopedToPropertyIDs {
		if len(filters.PropertyIDs) == 0 {
			// PM has no managed properties — return zero results.
			return nil, 0, nil
		}
		query = query.Where("property_id IN ?", filters.PropertyIDs)
	}
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.AssignedTo != nil {
		query = query.Where("assigned_to = ?", *filters.AssignedTo)
	}
	if filters.TenantID != nil {
		query = query.Where("tenant_id = ?", *filters.TenantID)
	}
	if filters.Priority != "" {
		query = query.Where("priority = ?", filters.Priority)
	}
	if filters.DateFrom != nil {
		query = query.Where("created_at >= ?", *filters.DateFrom)
	}
	if filters.DateTo != nil {
		query = query.Where("created_at <= ?", *filters.DateTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var orders []WorkOrder
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&orders).Error
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

// FindByTenant retrieves all work orders for a specific tenant with pagination.
func (r *Repository) FindByTenant(tenantID uint64, offset, limit int) ([]WorkOrder, int64, error) {
	return r.List(ListFilters{TenantID: &tenantID}, offset, limit)
}

// FindByTechnician retrieves all work orders assigned to a specific technician with pagination.
func (r *Repository) FindByTechnician(technicianID uint64, offset, limit int) ([]WorkOrder, int64, error) {
	return r.List(ListFilters{AssignedTo: &technicianID}, offset, limit)
}

// FindOverdueSLA retrieves work orders that have breached their SLA but are not yet completed or archived.
func (r *Repository) FindOverdueSLA() ([]WorkOrder, error) {
	var orders []WorkOrder
	err := r.db.Where("sla_due_at IS NOT NULL AND sla_due_at <= ? AND sla_breached_at IS NULL AND status NOT IN (?, ?)",
		time.Now().UTC(), "Completed", "Archived").
		Find(&orders).Error
	return orders, err
}

// CreateEvent inserts a new work order event (append-only).
func (r *Repository) CreateEvent(event *WorkOrderEvent) error {
	return r.db.Create(event).Error
}

// ListEvents retrieves all events for a work order, ordered by creation time.
func (r *Repository) ListEvents(workOrderID uint64) ([]WorkOrderEvent, error) {
	var events []WorkOrderEvent
	err := r.db.Where("work_order_id = ?", workOrderID).
		Order("created_at ASC").
		Find(&events).Error
	return events, err
}

// CreateCostItem inserts a new cost item.
func (r *Repository) CreateCostItem(item *CostItem) error {
	return r.db.Create(item).Error
}

// ListCostItems retrieves all cost items for a work order.
func (r *Repository) ListCostItems(workOrderID uint64) ([]CostItem, error) {
	var items []CostItem
	err := r.db.Where("work_order_id = ?", workOrderID).
		Order("created_at ASC").
		Find(&items).Error
	return items, err
}

// FindCostItemByID loads a cost item by its primary key.
func (r *Repository) FindCostItemByID(id uint64) (*CostItem, error) {
	var item CostItem
	err := r.db.Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FindWorkOrderForAttachment returns the ID and UUID of a work order for use by the attachment service.
// This method satisfies the attachments.WorkOrderQuerier interface.
func (r *Repository) FindWorkOrderForAttachment(id uint64) (uint64, string, error) {
	wo, err := r.FindByID(id)
	if err != nil {
		return 0, "", err
	}
	return wo.ID, wo.UUID, nil
}

// IsManagedBy checks whether userID has an active PropertyManager assignment on the given property.
func (r *Repository) IsManagedBy(propertyID, userID uint64) (bool, error) {
	var count int64
	err := r.db.Table("property_staff_assignments").
		Where("property_id = ? AND user_id = ? AND role = 'PropertyManager' AND is_active = ?", propertyID, userID, true).
		Count(&count).Error
	return count > 0, err
}

// GetManagedPropertyIDs returns all property IDs where userID has an active PropertyManager assignment.
func (r *Repository) GetManagedPropertyIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.Table("property_staff_assignments").
		Where("user_id = ? AND role = 'PropertyManager' AND is_active = ?", userID, true).
		Pluck("property_id", &ids).Error
	return ids, err
}

// GetWorkOrderPropertyID returns the property_id of the given work order.
// Used by the attachment and notification services for PM scope enforcement.
func (r *Repository) GetWorkOrderPropertyID(woID uint64) (uint64, error) {
	var wo WorkOrder
	if err := r.db.Select("property_id").Where("id = ?", woID).First(&wo).Error; err != nil {
		return 0, err
	}
	return wo.PropertyID, nil
}

// GetWorkOrderOwnerID returns the TenantID (original creator) of the given work order.
// This method satisfies the attachments.WorkOrderQuerier interface for ownership checks.
func (r *Repository) GetWorkOrderOwnerID(woID uint64) (uint64, error) {
	var wo WorkOrder
	if err := r.db.Select("tenant_id").Where("id = ?", woID).First(&wo).Error; err != nil {
		return 0, err
	}
	return wo.TenantID, nil
}

// GetWorkOrderAssignedTo returns the assigned technician's user ID for the given work order.
// Returns nil if the work order is unassigned.
// This method satisfies the attachments.WorkOrderQuerier interface.
func (r *Repository) GetWorkOrderAssignedTo(woID uint64) (*uint64, error) {
	var wo WorkOrder
	if err := r.db.Select("assigned_to").Where("id = ?", woID).First(&wo).Error; err != nil {
		return nil, err
	}
	return wo.AssignedTo, nil
}

// GetTenantPropertyID returns the property_id associated with the tenant's assigned unit.
// Returns 0 if the tenant has no profile or no unit assigned.
func (r *Repository) GetTenantPropertyID(tenantID uint64) (uint64, error) {
	var propertyID uint64
	err := r.db.Raw(`
		SELECT u.property_id FROM tenant_profiles tp
		JOIN units u ON u.id = tp.unit_id
		WHERE tp.user_id = ?`, tenantID).
		Scan(&propertyID).Error
	return propertyID, err
}

// ValidateUnitBelongsToProperty returns true if the given unit is part of the given property.
func (r *Repository) ValidateUnitBelongsToProperty(unitID, propertyID uint64) (bool, error) {
	var count int64
	err := r.db.Table("units").
		Where("id = ? AND property_id = ?", unitID, propertyID).
		Count(&count).Error
	return count > 0, err
}
