package payments

import (
	"time"

	"gorm.io/gorm"
)

// Repository handles all payment database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new payment Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// --- Payment operations ---

// Create inserts a new payment record.
func (r *Repository) Create(payment *Payment) error {
	return r.db.Create(payment).Error
}

// FindByID loads a payment by its primary key.
func (r *Repository) FindByID(id uint64) (*Payment, error) {
	var payment Payment
	err := r.db.Where("id = ?", id).First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// FindByUUID loads a payment by its UUID.
func (r *Repository) FindByUUID(uuid string) (*Payment, error) {
	var payment Payment
	err := r.db.Where("uuid = ?", uuid).First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// Update saves changes to an existing payment.
func (r *Repository) Update(payment *Payment) error {
	return r.db.Save(payment).Error
}

// PaymentFilters holds optional filters for listing payments.
type PaymentFilters struct {
	PropertyID          *uint64
	PropertyIDs         []uint64
	ScopedToPropertyIDs bool // when true, PropertyIDs is an authoritative allow-list (empty = no results)
	Status              string
	Kind                string
	TenantID            *uint64
}

// List retrieves payments matching the given filters with pagination.
func (r *Repository) List(filters PaymentFilters, offset, limit int) ([]Payment, int64, error) {
	query := r.db.Model(&Payment{})

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
	if filters.Kind != "" {
		query = query.Where("kind = ?", filters.Kind)
	}
	if filters.TenantID != nil {
		query = query.Where("tenant_id = ?", *filters.TenantID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var payments []Payment
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&payments).Error
	if err != nil {
		return nil, 0, err
	}

	return payments, total, nil
}

// FindExpiredIntents finds all pending intents whose expires_at has passed.
func (r *Repository) FindExpiredIntents() ([]Payment, error) {
	var payments []Payment
	now := time.Now().UTC()
	err := r.db.Where("kind = ? AND status = ? AND expires_at IS NOT NULL AND expires_at <= ?",
		"Intent", "Pending", now).
		Find(&payments).Error
	return payments, err
}

// FindByDateAndKinds retrieves all payments for a given date and set of kinds.
func (r *Repository) FindByDateAndKinds(date time.Time, kinds []string) ([]Payment, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := startOfDay.Add(24 * time.Hour)

	var payments []Payment
	err := r.db.Where("kind IN ? AND status IN ? AND created_at >= ? AND created_at < ?",
		kinds, []string{"Paid", "Settled"}, startOfDay, endOfDay).
		Find(&payments).Error
	return payments, err
}

// --- Payment approval operations ---

// CreateApproval inserts a new payment approval record.
func (r *Repository) CreateApproval(approval *PaymentApproval) error {
	return r.db.Create(approval).Error
}

// FindApprovalsByPayment retrieves all approvals for a given payment.
func (r *Repository) FindApprovalsByPayment(paymentID uint64) ([]PaymentApproval, error) {
	var approvals []PaymentApproval
	err := r.db.Where("payment_id = ?", paymentID).
		Order("approval_order ASC").
		Find(&approvals).Error
	return approvals, err
}

// CountApprovals returns the number of approvals for a given payment.
func (r *Repository) CountApprovals(paymentID uint64) (int64, error) {
	var count int64
	err := r.db.Model(&PaymentApproval{}).
		Where("payment_id = ?", paymentID).
		Count(&count).Error
	return count, err
}

// --- Reconciliation operations ---

// CreateReconciliation inserts a new reconciliation run record.
func (r *Repository) CreateReconciliation(run *ReconciliationRun) error {
	return r.db.Create(run).Error
}

// FindReconciliation loads a reconciliation run by its primary key.
func (r *Repository) FindReconciliation(id uint64) (*ReconciliationRun, error) {
	var run ReconciliationRun
	err := r.db.Where("id = ?", id).First(&run).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// ListReconciliations retrieves reconciliation runs with pagination.
func (r *Repository) ListReconciliations(offset, limit int) ([]ReconciliationRun, int64, error) {
	query := r.db.Model(&ReconciliationRun{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var runs []ReconciliationRun
	err := query.Order("run_date DESC").Offset(offset).Limit(limit).Find(&runs).Error
	if err != nil {
		return nil, 0, err
	}

	return runs, total, nil
}

// UpdateReconciliation saves changes to an existing reconciliation run.
func (r *Repository) UpdateReconciliation(run *ReconciliationRun) error {
	return r.db.Save(run).Error
}
