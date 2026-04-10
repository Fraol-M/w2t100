package users

import (
	"gorm.io/gorm"
)

// Repository defines the data access interface for users.
type Repository interface {
	Create(user *User) error
	FindByID(id uint64) (*User, error)
	FindByUsername(username string) (*User, error)
	FindByEmail(email string) (*User, error)
	Update(user *User) error
	List(page, perPage int, role, search string, active *bool) ([]User, int64, error)
	AssignRole(userID, roleID uint64) error
	RemoveRole(userID, roleID uint64) error
	FindRoleByName(name string) (*Role, error)
	CountByRole(roleName string) (int64, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository creates a new user repository backed by the given database.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(user *User) error {
	return r.db.Create(user).Error
}

func (r *repository) FindByID(id uint64) (*User, error) {
	var user User
	err := r.db.Preload("Roles").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *repository) FindByUsername(username string) (*User, error) {
	var user User
	err := r.db.Preload("Roles").Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *repository) FindByEmail(email string) (*User, error) {
	var user User
	err := r.db.Preload("Roles").Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *repository) Update(user *User) error {
	return r.db.Save(user).Error
}

func (r *repository) List(page, perPage int, role, search string, active *bool) ([]User, int64, error) {
	query := r.db.Model(&User{}).Preload("Roles")

	if role != "" {
		query = query.Joins("JOIN user_roles ON user_roles.user_id = users.id").
			Joins("JOIN roles ON roles.id = user_roles.role_id").
			Where("roles.name = ?", role)
	}

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("(username LIKE ? OR email LIKE ? OR first_name LIKE ? OR last_name LIKE ?)",
			like, like, like, like)
	}

	if active != nil {
		query = query.Where("users.is_active = ?", *active)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []User
	offset := (page - 1) * perPage
	if err := query.Offset(offset).Limit(perPage).Order("users.id DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *repository) AssignRole(userID, roleID uint64) error {
	ur := UserRole{
		UserID: userID,
		RoleID: roleID,
	}
	return r.db.Create(&ur).Error
}

func (r *repository) RemoveRole(userID, roleID uint64) error {
	return r.db.Where("user_id = ? AND role_id = ?", userID, roleID).Delete(&UserRole{}).Error
}

func (r *repository) FindRoleByName(name string) (*Role, error) {
	var role Role
	err := r.db.Where("name = ?", name).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *repository) CountByRole(roleName string) (int64, error) {
	var count int64
	err := r.db.Model(&UserRole{}).
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("roles.name = ?", roleName).
		Count(&count).Error
	return count, err
}
