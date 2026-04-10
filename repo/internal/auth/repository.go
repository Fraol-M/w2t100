package auth

import (
	"time"

	"gorm.io/gorm"
)

// Repository combines session and user data access for authentication.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new auth Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// SessionRepository handles all session-related database operations.
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository creates a new SessionRepository.
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// --- Session operations on Repository ---

// CreateSession inserts a new session record.
func (r *Repository) CreateSession(session *Session) error {
	return r.db.Create(session).Error
}

// FindByTokenHash looks up a non-revoked session by its SHA-256 token hash.
func (r *Repository) FindByTokenHash(hash string) (*Session, error) {
	var session Session
	err := r.db.Where("token_hash = ? AND revoked_at IS NULL", hash).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// UpdateLastActive bumps the last_active_at and idle_expires_at timestamps.
func (r *Repository) UpdateLastActive(id uint64, now time.Time, idleExpiresAt time.Time) error {
	return r.db.Model(&Session{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_active_at":  now,
			"idle_expires_at": idleExpiresAt,
		}).Error
}

// RevokeSession sets the revoked_at timestamp on a single session.
func (r *Repository) RevokeSession(id uint64) error {
	return r.db.Model(&Session{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", time.Now().UTC()).Error
}

// RevokeAllUserSessions revokes all active sessions for a given user.
func (r *Repository) RevokeAllUserSessions(userID uint64) error {
	return r.db.Model(&Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", time.Now().UTC()).Error
}

// DeleteExpiredSessions removes sessions that have exceeded their absolute expiry.
func (r *Repository) DeleteExpiredSessions() (int64, error) {
	result := r.db.Where("absolute_expires_at < ?", time.Now().UTC()).Delete(&Session{})
	return result.RowsAffected, result.Error
}

// FindUserWithRoles loads a user (with roles) by ID.
func (r *Repository) FindUserWithRoles(userID uint64) (*User, error) {
	var user User
	err := r.db.Preload("Roles").
		Where("id = ? AND deleted_at IS NULL", userID).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByUsername loads a user (with roles) by username.
func (r *Repository) FindByUsername(username string) (*User, error) {
	var user User
	err := r.db.Preload("Roles").
		Where("username = ? AND deleted_at IS NULL", username).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID loads a user (with roles) by ID.
func (r *Repository) FindByID(id uint64) (*User, error) {
	var user User
	err := r.db.Preload("Roles").
		Where("id = ? AND deleted_at IS NULL", id).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// --- Legacy SessionRepository methods (for backward compatibility) ---

// CreateSession inserts a new session record.
func (r *SessionRepository) CreateSession(session *Session) error {
	return r.db.Create(session).Error
}

// FindByTokenHash looks up a session by its SHA-256 token hash.
func (r *SessionRepository) FindByTokenHash(tokenHash string) (*Session, error) {
	var session Session
	err := r.db.Where("token_hash = ?", tokenHash).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// UpdateLastActive bumps the last_active_at and idle_expires_at timestamps.
func (r *SessionRepository) UpdateLastActive(sessionID uint64, lastActive, idleExpires time.Time) error {
	return r.db.Model(&Session{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"last_active_at":  lastActive,
			"idle_expires_at": idleExpires,
		}).Error
}

// RevokeSession sets the revoked_at timestamp on a single session.
func (r *SessionRepository) RevokeSession(tokenHash string, revokedAt time.Time) error {
	return r.db.Model(&Session{}).
		Where("token_hash = ? AND revoked_at IS NULL", tokenHash).
		Update("revoked_at", revokedAt).Error
}

// RevokeAllForUser revokes all active sessions for a given user.
func (r *SessionRepository) RevokeAllForUser(userID uint64, revokedAt time.Time) error {
	return r.db.Model(&Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", revokedAt).Error
}

// DeleteExpired removes sessions that have exceeded their absolute expiry.
func (r *SessionRepository) DeleteExpired(before time.Time) (int64, error) {
	result := r.db.Where("absolute_expires_at < ?", before).Delete(&Session{})
	return result.RowsAffected, result.Error
}

// UserRepository handles user lookups for authentication.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByUsername loads a user (with roles) by username.
func (r *UserRepository) FindByUsername(username string) (*User, error) {
	var user User
	err := r.db.Preload("Roles").
		Where("username = ? AND deleted_at IS NULL", username).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID loads a user (with roles) by ID.
func (r *UserRepository) FindByID(id uint64) (*User, error) {
	var user User
	err := r.db.Preload("Roles").
		Where("id = ? AND deleted_at IS NULL", id).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
