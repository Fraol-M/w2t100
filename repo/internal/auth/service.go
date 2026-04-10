package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuditLogger is the interface used by the auth service to log audit events.
// This is defined here to avoid circular imports with the audit package.
type AuditLogger interface {
	Log(actorID uint64, action, resourceType string, resourceID uint64, description string, ipAddress string, requestID string)
}

// SessionContext holds the authenticated user's information extracted from a session.
type SessionContext struct {
	UserID    uint64
	Username  string
	Roles     []string
	SessionID uint64
}

// sessionStore defines the session operations needed by the service.
type sessionStore interface {
	CreateSession(session *Session) error
	FindByTokenHash(hash string) (*Session, error)
	UpdateLastActive(id uint64, now time.Time, idleExpiresAt time.Time) error
	RevokeSession(id uint64) error
}

// userStore defines the user lookup operations needed by the service.
type userStore interface {
	FindByUsername(username string) (*User, error)
	FindByID(id uint64) (*User, error)
}

// Service provides authentication operations: login, logout, session validation.
type Service struct {
	repo        *Repository
	authCfg     config.AuthConfig
	auditLogger AuditLogger
	// Legacy fields for backward compatibility with tests using separate repos
	legacySessions *SessionRepository
	legacyUsers    *UserRepository
}

// NewService creates a new auth Service using the unified Repository.
func NewService(repo *Repository, authCfg config.AuthConfig, auditLogger AuditLogger) *Service {
	return &Service{
		repo:        repo,
		authCfg:     authCfg,
		auditLogger: auditLogger,
	}
}

// NewServiceWithRepos creates a new auth Service using separate session and user repositories.
// This is kept for backward compatibility with existing tests.
func NewServiceWithRepos(sessions *SessionRepository, users *UserRepository, authCfg config.AuthConfig, auditLogger AuditLogger) *Service {
	return &Service{
		legacySessions: sessions,
		legacyUsers:    users,
		authCfg:        authCfg,
		auditLogger:    auditLogger,
	}
}

// findByUsername resolves the user lookup across unified or legacy repos.
func (s *Service) findByUsername(username string) (*User, error) {
	if s.repo != nil {
		return s.repo.FindByUsername(username)
	}
	return s.legacyUsers.FindByUsername(username)
}

// findByID resolves the user lookup across unified or legacy repos.
func (s *Service) findByID(id uint64) (*User, error) {
	if s.repo != nil {
		return s.repo.FindByID(id)
	}
	return s.legacyUsers.FindByID(id)
}

// createSession creates a session via unified or legacy repo.
func (s *Service) createSession(session *Session) error {
	if s.repo != nil {
		return s.repo.CreateSession(session)
	}
	return s.legacySessions.CreateSession(session)
}

// findSessionByTokenHash looks up a session via unified or legacy repo.
func (s *Service) findSessionByTokenHash(hash string) (*Session, error) {
	if s.repo != nil {
		return s.repo.FindByTokenHash(hash)
	}
	return s.legacySessions.FindByTokenHash(hash)
}

// updateLastActive updates session activity via unified or legacy repo.
func (s *Service) updateLastActive(id uint64, now, idleExpires time.Time) error {
	if s.repo != nil {
		return s.repo.UpdateLastActive(id, now, idleExpires)
	}
	return s.legacySessions.UpdateLastActive(id, now, idleExpires)
}

// revokeSession revokes a session via unified or legacy repo.
func (s *Service) revokeSessionByHash(tokenHash string) error {
	if s.repo != nil {
		// Find the session by hash to get the ID
		session, err := s.repo.FindByTokenHash(tokenHash)
		if err != nil {
			return err
		}
		return s.repo.RevokeSession(session.ID)
	}
	return s.legacySessions.RevokeSession(tokenHash, time.Now().UTC())
}

// Login authenticates a user by username and password, then creates a session.
// Returns the raw bearer token (hex-encoded) and login response.
func (s *Service) Login(username, password, ip, userAgent, requestID string) (*LoginResponse, *common.AppError) {
	// Look up user
	user, err := s.findByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewUnauthorizedError("Invalid username or password")
		}
		log.Printf("ERROR auth.Login: failed to find user %q: %v", username, err)
		return nil, common.NewInternalError("")
	}

	// Check user is active
	if !user.IsActive {
		return nil, common.NewUnauthorizedError("Account is deactivated")
	}

	// Verify password against bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, common.NewUnauthorizedError("Invalid username or password")
	}

	// Generate cryptographically random token (32 bytes -> 64 hex chars)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Printf("ERROR auth.Login: failed to generate token: %v", err)
		return nil, common.NewInternalError("")
	}
	rawToken := hex.EncodeToString(tokenBytes)

	// Hash the token for storage (SHA-256 -> hex)
	tokenHash := hashToken(rawToken)

	now := time.Now().UTC()
	idleExpires := now.Add(s.authCfg.SessionIdleTimeout)
	absoluteExpires := now.Add(s.authCfg.SessionMaxLifetime)

	session := &Session{
		TokenHash:         tokenHash,
		UserID:            user.ID,
		IPAddress:         ip,
		UserAgent:         truncate(userAgent, 512),
		CreatedAt:         now,
		LastActiveAt:      now,
		IdleExpiresAt:     idleExpires,
		AbsoluteExpiresAt: absoluteExpires,
	}

	if err := s.createSession(session); err != nil {
		log.Printf("ERROR auth.Login: failed to create session: %v", err)
		return nil, common.NewInternalError("")
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.Log(user.ID, common.AuditActionLogin, "Session", session.ID, fmt.Sprintf("User %s logged in", username), ip, requestID)
	}

	return &LoginResponse{
		Token:     rawToken,
		ExpiresAt: absoluteExpires,
		User:      UserToInfo(user),
	}, nil
}

// Logout revokes the session identified by its token hash.
func (s *Service) Logout(tokenHash string, actorID uint64, ip, requestID string) *common.AppError {
	if err := s.revokeSessionByHash(tokenHash); err != nil {
		log.Printf("ERROR auth.Logout: failed to revoke session: %v", err)
		return common.NewInternalError("")
	}

	if s.auditLogger != nil {
		s.auditLogger.Log(actorID, common.AuditActionLogout, "Session", 0, "User logged out", ip, requestID)
	}

	return nil
}

// ValidateSession checks a token hash against the session store.
// Returns the session context if valid, or an AppError if expired/revoked.
func (s *Service) ValidateSession(tokenHash string) (*Session, *common.AppError) {
	session, err := s.findSessionByTokenHash(tokenHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewUnauthorizedError("Invalid or expired session")
		}
		log.Printf("ERROR auth.ValidateSession: %v", err)
		return nil, common.NewInternalError("")
	}

	// Check revocation
	if session.RevokedAt != nil {
		return nil, common.NewUnauthorizedError("Session has been revoked")
	}

	now := time.Now().UTC()

	// Check absolute expiry
	if now.After(session.AbsoluteExpiresAt) {
		return nil, common.NewUnauthorizedError("Session has expired")
	}

	// Check idle expiry
	if now.After(session.IdleExpiresAt) {
		return nil, common.NewUnauthorizedError("Session has expired due to inactivity")
	}

	// Refresh idle timeout — never extend beyond absolute expiry
	newIdleExpires := now.Add(s.authCfg.SessionIdleTimeout)
	if newIdleExpires.After(session.AbsoluteExpiresAt) {
		newIdleExpires = session.AbsoluteExpiresAt
	}

	if err := s.updateLastActive(session.ID, now, newIdleExpires); err != nil {
		log.Printf("WARN auth.ValidateSession: failed to update last_active_at: %v", err)
		// Non-fatal: session is still valid
	}

	session.LastActiveAt = now
	session.IdleExpiresAt = newIdleExpires

	return session, nil
}

// GetSessionContext validates a session and returns a SessionContext.
func (s *Service) GetSessionContext(tokenHash string) (*SessionContext, *common.AppError) {
	session, appErr := s.ValidateSession(tokenHash)
	if appErr != nil {
		return nil, appErr
	}

	user, userErr := s.findByID(session.UserID)
	if userErr != nil {
		if errors.Is(userErr, gorm.ErrRecordNotFound) {
			return nil, common.NewUnauthorizedError("User not found")
		}
		log.Printf("ERROR auth.GetSessionContext: %v", userErr)
		return nil, common.NewInternalError("")
	}

	return &SessionContext{
		UserID:    user.ID,
		Username:  user.Username,
		Roles:     user.RoleNames(),
		SessionID: session.ID,
	}, nil
}

// GetUserByID retrieves a user by their database ID.
func (s *Service) GetUserByID(id uint64) (*User, *common.AppError) {
	user, err := s.findByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewNotFoundError("User")
		}
		log.Printf("ERROR auth.GetUserByID: %v", err)
		return nil, common.NewInternalError("")
	}
	return user, nil
}

// GetCurrentUser retrieves the authenticated user's info by ID.
func (s *Service) GetCurrentUser(userID uint64) (*UserInfo, *common.AppError) {
	user, appErr := s.GetUserByID(userID)
	if appErr != nil {
		return nil, appErr
	}
	info := UserToInfo(user)
	return &info, nil
}

// HashToken computes SHA-256 of a raw token and returns the hex string.
// Exported for use by middleware.
func HashToken(raw string) string {
	return hashToken(raw)
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
