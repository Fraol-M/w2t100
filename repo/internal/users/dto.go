package users

import (
	"time"
)

// CreateUserRequest is the payload to create a new user.
type CreateUserRequest struct {
	Username  string   `json:"username" binding:"required"`
	Email     string   `json:"email" binding:"required,email"`
	Password  string   `json:"password" binding:"required,min=8"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Phone     string   `json:"phone"`
	RoleNames []string `json:"role_names"`
}

// UpdateUserRequest is the payload to update an existing user.
type UpdateUserRequest struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Email     *string `json:"email" binding:"omitempty,email"`
	Phone     *string `json:"phone"`
}

// UserResponse is the public representation of a user.
type UserResponse struct {
	ID        uint64    `json:"id"`
	UUID      string    `json:"uuid"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Phone     string    `json:"phone,omitempty"`
	IsActive  bool      `json:"is_active"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListUsersRequest holds pagination and filter parameters for listing users.
type ListUsersRequest struct {
	Page    int    `form:"page"`
	PerPage int    `form:"per_page"`
	Role    string `form:"role"`
	Search  string `form:"search"`
	Active  *bool  `form:"active"`
}

// AssignRoleRequest is the payload to assign a role to a user.
type AssignRoleRequest struct {
	RoleName string `json:"role_name" binding:"required"`
}

// ToggleActiveRequest is the payload to activate or deactivate a user.
type ToggleActiveRequest struct {
	IsActive bool `json:"is_active"`
}

// ToUserResponse converts a User model to a UserResponse DTO.
// Phone is masked by default unless revealPhone is true.
func ToUserResponse(u *User, decryptedPhone string, revealPhone bool) UserResponse {
	roles := make([]string, 0, len(u.Roles))
	for _, r := range u.Roles {
		roles = append(roles, r.Name)
	}

	phone := ""
	if decryptedPhone != "" {
		if revealPhone {
			phone = decryptedPhone
		} else {
			phone = maskPhone(decryptedPhone)
		}
	}

	return UserResponse{
		ID:        u.ID,
		UUID:      u.UUID,
		Username:  u.Username,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Phone:     phone,
		IsActive:  u.IsActive,
		Roles:     roles,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// maskPhone masks all but the last 4 digits of a phone number.
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return phone
	}
	masked := make([]byte, len(phone))
	for i := range masked {
		if i < len(phone)-4 {
			masked[i] = '*'
		} else {
			masked[i] = phone[i]
		}
	}
	return string(masked)
}
