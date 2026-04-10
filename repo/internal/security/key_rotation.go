package security

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// GenerateKey generates a new 32-byte random AES key, saves it to the key directory
// under the filename `<version>.key` (e.g. `1.key` for version 1), and caches it in
// the service. The key directory is created if it does not exist.
// This method is the public API for generating keys; internal key generation uses the
// unexported generateKey method in service.go.
func (s *Service) GenerateKey(version int) error {
	if err := os.MkdirAll(s.cfg.KeyDir, 0700); err != nil {
		return fmt.Errorf("GenerateKey: failed to create key directory: %w", err)
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return fmt.Errorf("GenerateKey: failed to generate random key: %w", err)
	}

	keyPath := filepath.Join(s.cfg.KeyDir, fmt.Sprintf("%d.key", version))
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return fmt.Errorf("GenerateKey: failed to write key file %q: %w", keyPath, err)
	}

	s.mu.Lock()
	s.keys[version] = key
	s.mu.Unlock()

	return nil
}
