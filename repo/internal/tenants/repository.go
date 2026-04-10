package tenants

import (
	"gorm.io/gorm"
)

// Repository defines the data access interface for tenant profiles.
type Repository interface {
	Create(profile *TenantProfile) error
	FindByID(id uint64) (*TenantProfile, error)
	FindByUserID(userID uint64) (*TenantProfile, error)
	Update(profile *TenantProfile) error
	List(page, perPage int) ([]TenantProfile, int64, error)
	FindByProperty(propertyID uint64, page, perPage int) ([]TenantProfile, int64, error)
	// IsPMForUnit returns true if pmID is an active staff member on the property
	// that contains unitID. Used to scope PropertyManager profile access.
	IsPMForUnit(unitID, pmID uint64) bool
	// IsPMForProperty returns true if pmID is an active staff member on propertyID.
	IsPMForProperty(propertyID, pmID uint64) bool
}

type repository struct {
	db *gorm.DB
}

// NewRepository creates a new tenant profile repository.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(profile *TenantProfile) error {
	return r.db.Create(profile).Error
}

func (r *repository) FindByID(id uint64) (*TenantProfile, error) {
	var profile TenantProfile
	err := r.db.First(&profile, id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *repository) FindByUserID(userID uint64) (*TenantProfile, error) {
	var profile TenantProfile
	err := r.db.Where("user_id = ?", userID).First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *repository) Update(profile *TenantProfile) error {
	return r.db.Save(profile).Error
}

func (r *repository) List(page, perPage int) ([]TenantProfile, int64, error) {
	var total int64
	if err := r.db.Model(&TenantProfile{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var profiles []TenantProfile
	offset := (page - 1) * perPage
	if err := r.db.Offset(offset).Limit(perPage).Order("id DESC").Find(&profiles).Error; err != nil {
		return nil, 0, err
	}

	return profiles, total, nil
}

func (r *repository) IsPMForUnit(unitID, pmID uint64) bool {
	var count int64
	r.db.Table("property_staff_assignments psa").
		Joins("JOIN units u ON u.property_id = psa.property_id").
		Where("u.id = ? AND psa.user_id = ? AND psa.role = 'PropertyManager' AND psa.is_active = ?", unitID, pmID, true).
		Count(&count)
	return count > 0
}

func (r *repository) IsPMForProperty(propertyID, pmID uint64) bool {
	var count int64
	r.db.Table("property_staff_assignments").
		Where("property_id = ? AND user_id = ? AND role = 'PropertyManager' AND is_active = ?", propertyID, pmID, true).
		Count(&count)
	return count > 0
}

func (r *repository) FindByProperty(propertyID uint64, page, perPage int) ([]TenantProfile, int64, error) {
	query := r.db.Model(&TenantProfile{}).
		Joins("JOIN units ON units.id = tenant_profiles.unit_id").
		Where("units.property_id = ?", propertyID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var profiles []TenantProfile
	offset := (page - 1) * perPage
	if err := query.Offset(offset).Limit(perPage).Order("tenant_profiles.id DESC").Find(&profiles).Error; err != nil {
		return nil, 0, err
	}

	return profiles, total, nil
}
