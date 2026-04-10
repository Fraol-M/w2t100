package auth

import "time"

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is returned on successful authentication.
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

// UserInfo holds non-sensitive user information returned by the API.
type UserInfo struct {
	ID       uint64   `json:"id"`
	UUID     string   `json:"uuid"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
}

// MeResponse is the response for GET /auth/me.
type MeResponse = UserInfo

// UserToInfo converts a User model to a UserInfo DTO.
func UserToInfo(u *User) UserInfo {
	return UserInfo{
		ID:       u.ID,
		UUID:     u.UUID,
		Username: u.Username,
		Email:    u.Email,
		Roles:    u.RoleNames(),
	}
}
