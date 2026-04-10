package security

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"propertyops/backend/internal/config"
)

// newTestService creates a Service backed by a temporary key directory.
// It directly injects a pre-generated key into the service's internal key map
// so that tests do not depend on on-disk key file naming conventions.
func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	cfg := config.EncryptionConfig{
		KeyDir:       dir,
		ActiveKeyID:  1,
		RotationDays: 180,
	}
	svc := &Service{
		keys:     make(map[int][]byte),
		activeID: cfg.ActiveKeyID,
		cfg:      cfg,
	}
	// Inject a known 32-byte key for version 1.
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	svc.keys[1] = key
	return svc
}


// --- Encrypt / Decrypt round-trip ---

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	svc := newTestService(t)

	plaintext := []byte("hello, PropertyOps!")
	ciphertext, version, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if version != 1 {
		t.Errorf("expected key version 1, got %d", version)
	}

	decrypted, err := svc.Decrypt(ciphertext, version)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("round-trip mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	svc := newTestService(t)

	ciphertext, version, err := svc.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("Encrypt empty plaintext failed: %v", err)
	}
	decrypted, err := svc.Decrypt(ciphertext, version)
	if err != nil {
		t.Fatalf("Decrypt empty plaintext failed: %v", err)
	}
	if len(decrypted) != 0 {
		t.Errorf("expected empty decrypted, got len=%d", len(decrypted))
	}
}

func TestEncryptDecrypt_NonDeterministic(t *testing.T) {
	svc := newTestService(t)
	plaintext := []byte("same input")

	ct1, _, _ := svc.Encrypt(plaintext)
	ct2, _, _ := svc.Encrypt(plaintext)
	if bytes.Equal(ct1, ct2) {
		t.Error("expected different ciphertexts for the same plaintext (nonce randomness)")
	}
}

// --- Wrong key version ---

func TestDecrypt_WrongKeyVersion_Fails(t *testing.T) {
	svc := newTestService(t)

	// Inject a second key for version 2.
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte(i + 50)
	}
	svc.mu.Lock()
	svc.keys[2] = key2
	svc.mu.Unlock()

	plaintext := []byte("secret data")
	ciphertext, version, err := svc.Encrypt(plaintext) // encrypted with key 1
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected version 1, got %d", version)
	}

	// Try to decrypt claiming key version 2 — the embedded version is 1 so it must fail.
	_, err = svc.Decrypt(ciphertext, 2)
	if err == nil {
		t.Error("expected decryption with wrong key version to fail")
	}
}

// --- Tampered ciphertext ---

func TestDecrypt_TamperedCiphertext_Fails(t *testing.T) {
	svc := newTestService(t)

	ciphertext, version, err := svc.Encrypt([]byte("important data"))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Flip a byte in the ciphertext payload (past the 16-byte header).
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xFF

	_, err = svc.Decrypt(tampered, version)
	if err == nil {
		t.Error("expected decryption of tampered ciphertext to fail")
	}
}

func TestDecrypt_TooShort_Fails(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Decrypt([]byte{0x00, 0x00}, 1)
	if err == nil {
		t.Error("expected error for too-short ciphertext")
	}
}

// --- MaskString ---

func TestMaskString_LongString(t *testing.T) {
	result := MaskString("abcdefghij")
	if result != "******ghij" {
		t.Errorf("expected '******ghij', got %q", result)
	}
}

func TestMaskString_ExactlyFourChars(t *testing.T) {
	result := MaskString("abcd")
	if result != "****" {
		t.Errorf("expected '****', got %q", result)
	}
}

func TestMaskString_ShortString(t *testing.T) {
	result := MaskString("ab")
	if result != "****" {
		t.Errorf("expected '****', got %q", result)
	}
}

func TestMaskString_Empty(t *testing.T) {
	result := MaskString("")
	if result != "****" {
		t.Errorf("expected '****', got %q", result)
	}
}

// --- MaskPhone ---

func TestMaskPhone_Standard(t *testing.T) {
	result := MaskPhone("+1 (555) 867-5309")
	if result != "****-****-5309" {
		t.Errorf("expected '****-****-5309', got %q", result)
	}
}

func TestMaskPhone_ShortNumber(t *testing.T) {
	result := MaskPhone("123")
	if result != "****" {
		t.Errorf("expected '****', got %q", result)
	}
}

func TestMaskPhone_ExactlyFourDigits(t *testing.T) {
	result := MaskPhone("1234")
	if result != "****" {
		t.Errorf("expected '****', got %q", result)
	}
}

// --- Key rotation due ---

func TestKeyRotationDue_Fresh(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EncryptionConfig{
		KeyDir:       dir,
		ActiveKeyID:  1,
		RotationDays: 180,
	}
	// Write a fresh key file using service.go's naming convention.
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	keyPath := filepath.Join(dir, "1.key")
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		t.Fatal(err)
	}

	svc := &Service{
		keys:     map[int][]byte{1: key},
		activeID: 1,
		cfg:      cfg,
	}
	// Key was just created — rotation should not be due.
	if svc.KeyRotationDue() {
		t.Error("expected rotation not due for a freshly created key")
	}
}

func TestKeyRotationDue_OldKey(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EncryptionConfig{
		KeyDir:       dir,
		ActiveKeyID:  1,
		RotationDays: 1, // 1-day threshold
	}
	key := make([]byte, 32)
	keyPath := filepath.Join(dir, "1.key")
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		t.Fatal(err)
	}
	// Back-date the file modification time to 2 days ago so rotation is clearly overdue.
	pastTime := time.Now().AddDate(0, 0, -2)
	if err := os.Chtimes(keyPath, pastTime, pastTime); err != nil {
		t.Fatalf("failed to back-date key file: %v", err)
	}

	svc := &Service{
		keys:     map[int][]byte{1: key},
		activeID: 1,
		cfg:      cfg,
	}
	if !svc.KeyRotationDue() {
		t.Error("expected rotation to be due for a 2-day-old key with 1-day threshold")
	}
}

func TestKeyRotationDue_MissingFile(t *testing.T) {
	cfg := config.EncryptionConfig{
		KeyDir:       t.TempDir(),
		ActiveKeyID:  99,
		RotationDays: 180,
	}
	svc := &Service{
		keys:     make(map[int][]byte),
		activeID: 99,
		cfg:      cfg,
	}
	// File 99.key doesn't exist — KeyRotationDue returns true (assumes rotation needed).
	if !svc.KeyRotationDue() {
		t.Error("expected true when key file does not exist (service.go behaviour)")
	}
}

// --- ActiveKeyID ---

func TestActiveKeyID(t *testing.T) {
	cfg := config.EncryptionConfig{
		KeyDir:       t.TempDir(),
		ActiveKeyID:  3,
		RotationDays: 90,
	}
	svc := &Service{
		keys:     make(map[int][]byte),
		activeID: 3,
		cfg:      cfg,
	}
	if svc.ActiveKeyID() != 3 {
		t.Errorf("expected ActiveKeyID=3, got %d", svc.ActiveKeyID())
	}
}

// --- GenerateKey (key_rotation.go) ---

func TestGenerateKey_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EncryptionConfig{
		KeyDir:       dir,
		ActiveKeyID:  1,
		RotationDays: 180,
	}
	svc := &Service{
		keys:     make(map[int][]byte),
		activeID: 1,
		cfg:      cfg,
	}
	if err := svc.GenerateKey(5); err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	keyPath := filepath.Join(dir, "5.key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("expected 5.key to be created: %v", err)
	}
	if len(data) != 32 {
		t.Errorf("expected 32-byte key, got %d bytes", len(data))
	}
}

func TestGenerateKey_CachesKey(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EncryptionConfig{
		KeyDir:       dir,
		ActiveKeyID:  1,
		RotationDays: 180,
	}
	svc := &Service{
		keys:     make(map[int][]byte),
		activeID: 1,
		cfg:      cfg,
	}
	if err := svc.GenerateKey(7); err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	svc.mu.RLock()
	_, ok := svc.keys[7]
	svc.mu.RUnlock()
	if !ok {
		t.Error("expected key version 7 to be cached after GenerateKey")
	}
}

