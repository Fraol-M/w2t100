package properties

import (
	"gorm.io/gorm"
)

// Repository defines the data access interface for properties and related entities.
type Repository interface {
	// Properties
	CreateProperty(property *Property) error
	FindPropertyByID(id uint64) (*Property, error)
	UpdateProperty(property *Property) error
	ListProperties(page, perPage int, managerID *uint64, active *bool) ([]Property, int64, error)

	// Units
	CreateUnit(unit *Unit) error
	FindUnitByID(id uint64) (*Unit, error)
	UpdateUnit(unit *Unit) error
	ListUnitsByProperty(propertyID uint64, page, perPage int) ([]Unit, int64, error)

	// Staff Assignments
	AssignStaff(assignment *PropertyStaffAssignment) error
	RemoveStaff(propertyID, userID uint64, role string) error
	ListStaffByProperty(propertyID uint64) ([]PropertyStaffAssignment, error)
	IsStaffMember(propertyID, userID uint64) (bool, error)

	// Skill Tags
	AddSkillTag(tag *TechnicianSkillTag) error
	RemoveSkillTag(userID uint64, tagName string) error
	ListSkillTagsByUser(userID uint64) ([]TechnicianSkillTag, error)

	// Dispatch
	FindTechniciansByPropertyAndSkill(propertyID uint64, skillTag string) ([]PropertyStaffAssignment, error)
	GetDispatchCursor(propertyID uint64, skillTag string) (*DispatchCursor, error)
	UpsertDispatchCursor(cursor *DispatchCursor) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository creates a new property repository.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// --- Property operations ---

func (r *repository) CreateProperty(property *Property) error {
	return r.db.Create(property).Error
}

func (r *repository) FindPropertyByID(id uint64) (*Property, error) {
	var property Property
	err := r.db.First(&property, id).Error
	if err != nil {
		return nil, err
	}
	return &property, nil
}

func (r *repository) UpdateProperty(property *Property) error {
	return r.db.Save(property).Error
}

func (r *repository) ListProperties(page, perPage int, managerID *uint64, active *bool) ([]Property, int64, error) {
	query := r.db.Model(&Property{})

	if managerID != nil {
		query = query.Where("manager_id = ?", *managerID)
	}
	if active != nil {
		query = query.Where("is_active = ?", *active)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var properties []Property
	offset := (page - 1) * perPage
	if err := query.Offset(offset).Limit(perPage).Order("id DESC").Find(&properties).Error; err != nil {
		return nil, 0, err
	}

	return properties, total, nil
}

// --- Unit operations ---

func (r *repository) CreateUnit(unit *Unit) error {
	return r.db.Create(unit).Error
}

func (r *repository) FindUnitByID(id uint64) (*Unit, error) {
	var unit Unit
	err := r.db.First(&unit, id).Error
	if err != nil {
		return nil, err
	}
	return &unit, nil
}

func (r *repository) UpdateUnit(unit *Unit) error {
	return r.db.Save(unit).Error
}

func (r *repository) ListUnitsByProperty(propertyID uint64, page, perPage int) ([]Unit, int64, error) {
	query := r.db.Model(&Unit{}).Where("property_id = ?", propertyID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var units []Unit
	offset := (page - 1) * perPage
	if err := query.Offset(offset).Limit(perPage).Order("unit_number ASC").Find(&units).Error; err != nil {
		return nil, 0, err
	}

	return units, total, nil
}

// --- Staff Assignment operations ---

func (r *repository) AssignStaff(assignment *PropertyStaffAssignment) error {
	return r.db.Create(assignment).Error
}

func (r *repository) RemoveStaff(propertyID, userID uint64, role string) error {
	return r.db.Where("property_id = ? AND user_id = ? AND role = ?", propertyID, userID, role).
		Delete(&PropertyStaffAssignment{}).Error
}

func (r *repository) ListStaffByProperty(propertyID uint64) ([]PropertyStaffAssignment, error) {
	var assignments []PropertyStaffAssignment
	err := r.db.Where("property_id = ?", propertyID).Order("created_at ASC").Find(&assignments).Error
	return assignments, err
}

func (r *repository) IsStaffMember(propertyID, userID uint64) (bool, error) {
	var count int64
	err := r.db.Model(&PropertyStaffAssignment{}).
		Where("property_id = ? AND user_id = ?", propertyID, userID).
		Count(&count).Error
	return count > 0, err
}

// --- Skill Tag operations ---

func (r *repository) AddSkillTag(tag *TechnicianSkillTag) error {
	return r.db.Create(tag).Error
}

func (r *repository) RemoveSkillTag(userID uint64, tagName string) error {
	return r.db.Where("user_id = ? AND tag = ?", userID, tagName).
		Delete(&TechnicianSkillTag{}).Error
}

func (r *repository) ListSkillTagsByUser(userID uint64) ([]TechnicianSkillTag, error) {
	var tags []TechnicianSkillTag
	err := r.db.Where("user_id = ?", userID).Order("tag ASC").Find(&tags).Error
	return tags, err
}

// --- Dispatch operations ---

func (r *repository) FindTechniciansByPropertyAndSkill(propertyID uint64, skillTag string) ([]PropertyStaffAssignment, error) {
	var assignments []PropertyStaffAssignment
	err := r.db.
		Joins("JOIN technician_skill_tags ON technician_skill_tags.user_id = property_staff_assignments.user_id").
		Where("property_staff_assignments.property_id = ? AND technician_skill_tags.tag = ? AND property_staff_assignments.role = ?",
			propertyID, skillTag, "Technician").
		Order("property_staff_assignments.user_id ASC").
		Find(&assignments).Error
	return assignments, err
}

func (r *repository) GetDispatchCursor(propertyID uint64, skillTag string) (*DispatchCursor, error) {
	var cursor DispatchCursor
	err := r.db.Where("property_id = ? AND skill_tag = ?", propertyID, skillTag).First(&cursor).Error
	if err != nil {
		return nil, err
	}
	return &cursor, nil
}

func (r *repository) UpsertDispatchCursor(cursor *DispatchCursor) error {
	return r.db.Save(cursor).Error
}
