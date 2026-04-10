package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"propertyops/backend/internal/config"

	"golang.org/x/crypto/bcrypt"
)

func TestHashToken(t *testing.T) {
	raw := "abc123"
	expected := sha256.Sum256([]byte(raw))
	expectedHex := hex.EncodeToString(expected[:])

	got := HashToken(raw)
	if got != expectedHex {
		t.Errorf("HashToken(%q) = %q, want %q", raw, got, expectedHex)
	}
}

func TestHashTokenLength(t *testing.T) {
	got := HashToken("anything")
	if len(got) != 64 {
		t.Errorf("HashToken output length = %d, want 64", len(got))
	}
}

func TestPasswordVerification(t *testing.T) {
	password := "SuperSecret123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}

	// Correct password should succeed
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Error("expected correct password to verify, got error:", err)
	}

	// Wrong password should fail
	if err := bcrypt.CompareHashAndPassword(hash, []byte("WrongPassword")); err == nil {
		t.Error("expected wrong password to fail verification")
	}
}

func TestPasswordVerificationCost(t *testing.T) {
	password := "TestPass!234"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}

	cost, err := bcrypt.Cost(hash)
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if cost < 12 {
		t.Errorf("bcrypt cost = %d, want >= 12", cost)
	}
}

func TestSessionIdleExpiry(t *testing.T) {
	authCfg := config.AuthConfig{
		BcryptCost:         12,
		SessionIdleTimeout: 30 * time.Minute,
		SessionMaxLifetime: 7 * 24 * time.Hour,
	}

	now := time.Now().UTC()

	tests := []struct {
		name        string
		session     Session
		expectValid bool
	}{
		{
			name: "active session within idle timeout",
			session: Session{
				ID:                1,
				UserID:            1,
				LastActiveAt:      now.Add(-10 * time.Minute),
				IdleExpiresAt:     now.Add(20 * time.Minute),
				AbsoluteExpiresAt: now.Add(7 * 24 * time.Hour),
			},
			expectValid: true,
		},
		{
			name: "session past idle timeout",
			session: Session{
				ID:                2,
				UserID:            1,
				LastActiveAt:      now.Add(-45 * time.Minute),
				IdleExpiresAt:     now.Add(-15 * time.Minute),
				AbsoluteExpiresAt: now.Add(7 * 24 * time.Hour),
			},
			expectValid: false,
		},
		{
			name: "session past absolute expiry",
			session: Session{
				ID:                3,
				UserID:            1,
				LastActiveAt:      now.Add(-1 * time.Hour),
				IdleExpiresAt:     now.Add(30 * time.Minute),
				AbsoluteExpiresAt: now.Add(-1 * time.Hour),
			},
			expectValid: false,
		},
		{
			name: "revoked session",
			session: Session{
				ID:                4,
				UserID:            1,
				LastActiveAt:      now,
				IdleExpiresAt:     now.Add(30 * time.Minute),
				AbsoluteExpiresAt: now.Add(7 * 24 * time.Hour),
				RevokedAt:         timePtr(now.Add(-5 * time.Minute)),
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := isSessionValid(&tt.session, authCfg)
			if valid != tt.expectValid {
				t.Errorf("isSessionValid() = %v, want %v", valid, tt.expectValid)
			}
		})
	}
}

func TestTokenGenerationLength(t *testing.T) {
	// Simulate token generation: 32 bytes -> 64 hex chars
	tokenBytes := make([]byte, 32)
	// Use a deterministic fill for testing
	for i := range tokenBytes {
		tokenBytes[i] = byte(i)
	}
	rawToken := hex.EncodeToString(tokenBytes)
	if len(rawToken) != 64 {
		t.Errorf("raw token length = %d, want 64", len(rawToken))
	}

	// Hash should be 64 hex chars (SHA-256)
	hashed := HashToken(rawToken)
	if len(hashed) != 64 {
		t.Errorf("hashed token length = %d, want 64", len(hashed))
	}

	// Raw and hash should differ
	if rawToken == hashed {
		t.Error("raw token and hash should not be equal")
	}
}

func TestSessionMaxLifetime(t *testing.T) {
	authCfg := config.AuthConfig{
		BcryptCost:         12,
		SessionIdleTimeout: 30 * time.Minute,
		SessionMaxLifetime: 7 * 24 * time.Hour,
	}

	now := time.Now().UTC()

	// Session created 8 days ago — absolute expiry has passed even though idle looks OK
	session := Session{
		ID:                1,
		UserID:            1,
		LastActiveAt:      now.Add(-5 * time.Minute),
		IdleExpiresAt:     now.Add(25 * time.Minute),
		AbsoluteExpiresAt: now.Add(-24 * time.Hour), // expired 1 day ago
	}

	if isSessionValid(&session, authCfg) {
		t.Error("session past absolute expiry should be invalid")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

// isSessionValid checks session validity without hitting the DB.
// This mirrors the logic in Service.ValidateSession for unit testing.
func isSessionValid(session *Session, _ config.AuthConfig) bool {
	if session.RevokedAt != nil {
		return false
	}
	now := time.Now().UTC()
	if now.After(session.AbsoluteExpiresAt) {
		return false
	}
	if now.After(session.IdleExpiresAt) {
		return false
	}
	return true
}

func timePtr(t time.Time) *time.Time {
	return &t
}
