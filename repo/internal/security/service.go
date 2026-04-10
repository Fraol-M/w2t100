package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"propertyops/backend/internal/config"
)

// Service provides field-level encryption and key management.
type Service struct {
	mu       sync.RWMutex
	keys     map[int][]byte // keyVersion -> 32-byte AES-256 key
	activeID int
	cfg      config.EncryptionConfig
}

// NewService initialises the security service by loading keys from cfg.KeyDir.
func NewService(cfg config.EncryptionConfig) *Service {
	s := &Service{
		keys:     make(map[int][]byte),
		activeID: cfg.ActiveKeyID,
		cfg:      cfg,
	}
	if err := s.loadKeys(); err != nil {
		log.Fatalf("security: failed to load keys from %s: %v", cfg.KeyDir, err)
	}
	// Derive active key from the highest version found on disk so that a rotation
	// persists across restarts even if the environment variable is not updated.
	s.mu.Lock()
	for v := range s.keys {
		if v > s.activeID {
			s.activeID = v
		}
	}
	s.mu.Unlock()
	return s
}

// loadKeys reads key files named <version>.key from the key directory.
// Each file must contain exactly 32 bytes (AES-256).
func (s *Service) loadKeys() error {
	entries, err := os.ReadDir(s.cfg.KeyDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Auto-generate a key if directory doesn't exist yet.
			_ = os.MkdirAll(s.cfg.KeyDir, 0700)
			return s.generateKey(s.cfg.ActiveKeyID)
		}
		return err
	}

	loaded := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".key") {
			continue
		}
		versionStr := strings.TrimSuffix(name, ".key")
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			continue
		}
		keyBytes, err := os.ReadFile(filepath.Join(s.cfg.KeyDir, name))
		if err != nil {
			continue
		}
		if len(keyBytes) != 32 {
			continue
		}
		s.keys[version] = keyBytes
		loaded++
	}

	if loaded == 0 {
		return s.generateKey(s.cfg.ActiveKeyID)
	}
	return nil
}

// generateKey creates a new random 32-byte key and writes it to disk.
func (s *Service) generateKey(version int) error {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return err
	}
	path := filepath.Join(s.cfg.KeyDir, fmt.Sprintf("%d.key", version))
	if err := os.WriteFile(path, key, 0600); err != nil {
		return err
	}
	s.keys[version] = key
	return nil
}

// Encrypt encrypts plaintext using AES-256-GCM with the active key.
// The returned ciphertext is: [4-byte big-endian keyVersion][12-byte nonce][GCM ciphertext+tag].
func (s *Service) Encrypt(plaintext []byte) ([]byte, int, error) {
	s.mu.RLock()
	key, ok := s.keys[s.activeID]
	keyVersion := s.activeID
	s.mu.RUnlock()

	if !ok {
		return nil, 0, fmt.Errorf("security: active key %d not loaded", s.activeID)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, 0, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, 0, err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Prepend 4-byte key version and nonce so Decrypt can unwrap.
	out := make([]byte, 4+len(nonce)+len(ciphertext))
	binary.BigEndian.PutUint32(out[0:4], uint32(keyVersion))
	copy(out[4:4+len(nonce)], nonce)
	copy(out[4+len(nonce):], ciphertext)

	return out, keyVersion, nil
}

// Decrypt decrypts a ciphertext blob previously produced by Encrypt.
// keyVersion is used as a cross-check against the embedded version.
func (s *Service) Decrypt(ciphertext []byte, keyVersion int) ([]byte, error) {
	if len(ciphertext) < 4 {
		return nil, fmt.Errorf("security: ciphertext too short")
	}

	embeddedVersion := int(binary.BigEndian.Uint32(ciphertext[0:4]))
	if embeddedVersion != keyVersion {
		return nil, fmt.Errorf("security: key version mismatch: got %d, want %d", embeddedVersion, keyVersion)
	}

	s.mu.RLock()
	key, ok := s.keys[keyVersion]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("security: key version %d not found", keyVersion)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < 4+nonceSize {
		return nil, fmt.Errorf("security: ciphertext too short for nonce")
	}

	nonce := ciphertext[4 : 4+nonceSize]
	ct := ciphertext[4+nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("security: decryption failed: %w", err)
	}
	return plaintext, nil
}

// ActiveKeyID returns the ID of the currently active encryption key.
func (s *Service) ActiveKeyID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeID
}

// KeyRotationDue reports whether a new key should be generated based on RotationDays.
// It checks the modification time of the active key file.
func (s *Service) KeyRotationDue() bool {
	path := filepath.Join(s.cfg.KeyDir, fmt.Sprintf("%d.key", s.activeID))
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > time.Duration(s.cfg.RotationDays)*24*time.Hour
}

// ListKeyVersions returns the sorted list of loaded key versions.
func (s *Service) ListKeyVersions() ([]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions := make([]int, 0, len(s.keys))
	for v := range s.keys {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	return versions, nil
}

// RotateKey generates a new key with version = current max + 1, sets it as active,
// persists it to disk, and returns the new version.
func (s *Service) RotateKey() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newVersion := s.activeID + 1
	for v := range s.keys {
		if v >= newVersion {
			newVersion = v + 1
		}
	}

	if err := s.generateKey(newVersion); err != nil {
		return 0, fmt.Errorf("security: failed to generate new key: %w", err)
	}
	s.activeID = newVersion
	return newVersion, nil
}
