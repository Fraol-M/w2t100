package backups

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- fake security service for tests ---

type fakeSecurityService struct{}

func (f *fakeSecurityService) Encrypt(plaintext []byte) ([]byte, int, error) {
	// Reversible XOR — not cryptographically secure, used for testing only.
	out := make([]byte, len(plaintext))
	for i, b := range plaintext {
		out[i] = b ^ 0xFF
	}
	return out, 1, nil
}

func (f *fakeSecurityService) Decrypt(ciphertext []byte, _ int) ([]byte, error) {
	out := make([]byte, len(ciphertext))
	for i, b := range ciphertext {
		out[i] = b ^ 0xFF
	}
	return out, nil
}

// --- fake audit logger for tests ---

type fakeAuditLogger struct {
	calls []string
}

func (f *fakeAuditLogger) Log(_ uint64, action, _ string, _ uint64, _, _, _ string) {
	f.calls = append(f.calls, action)
}

// --- testService isolates backup logic from *gorm.DB and *config.Config ---

// testService mirrors the relevant methods of Service without needing a real DB
// or config. All tests that don't require the full service use this type.
type testService struct {
	backupRoot    string
	retentionDays int
	encrypted     bool
	security      SecurityService
	audit         AuditLogger
}

func newTestService(dir string, retentionDays int, encrypted bool) *testService {
	if retentionDays == 0 {
		retentionDays = 30
	}
	return &testService{
		backupRoot:    dir,
		retentionDays: retentionDays,
		encrypted:     encrypted,
		security:      &fakeSecurityService{},
		audit:         &fakeAuditLogger{},
	}
}

// writeBackupFile writes an (optionally encrypted) file and a companion manifest.
func (ts *testService) writeBackupFile(backupPath string, plaintext []byte) error {
	var fileBytes []byte
	var keyVersion int

	if ts.encrypted {
		enc, kv, err := ts.security.Encrypt(plaintext)
		if err != nil {
			return err
		}
		fileBytes = enc
		keyVersion = kv
	} else {
		fileBytes = plaintext
	}

	if err := os.WriteFile(backupPath, fileBytes, 0640); err != nil {
		return err
	}
	return ts.writeManifest(backupPath, fileBytes, ts.encrypted, keyVersion)
}

// writeManifest creates the manifest JSON for a backup file.
func (ts *testService) writeManifest(backupPath string, fileBytes []byte, encrypted bool, keyVersion int) error {
	hash := sha256.Sum256(fileBytes)
	manifest := BackupManifest{
		BackupID:      filepath.Base(backupPath),
		CreatedAt:     time.Now().UTC(),
		DBName:        "testdb",
		FileHash:      hex.EncodeToString(hash[:]),
		FileSizeBytes: int64(len(fileBytes)),
		Encrypted:     encrypted,
		KeyVersion:    keyVersion,
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestPath := strings.TrimSuffix(backupPath, ".sql.enc") + ".manifest.json"
	return os.WriteFile(manifestPath, b, 0640)
}

// validateBackup mirrors Service.ValidateBackup without config/DB dependencies.
func (ts *testService) validateBackup(backupPath string) *ValidationResult {
	result := &ValidationResult{}

	fileBytes, err := os.ReadFile(backupPath)
	if err != nil {
		result.Errors = append(result.Errors, "cannot read backup file")
		return result
	}
	result.Checks = append(result.Checks, "backup_file_readable")

	manifestPath := strings.TrimSuffix(backupPath, ".sql.enc") + ".manifest.json"
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		result.Errors = append(result.Errors, "cannot read manifest")
		return result
	}
	result.Checks = append(result.Checks, "manifest_readable")

	var manifest BackupManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		result.Errors = append(result.Errors, "invalid manifest JSON")
		return result
	}

	// Verify SHA-256 checksum.
	hash := sha256.Sum256(fileBytes)
	hashHex := hex.EncodeToString(hash[:])
	if hashHex != manifest.FileHash {
		result.Errors = append(result.Errors, "checksum mismatch")
	} else {
		result.Checks = append(result.Checks, "checksum_valid")
	}

	// Decrypt if needed and verify dump structure.
	var dumpBytes []byte
	if manifest.Encrypted {
		decrypted, err := ts.security.Decrypt(fileBytes, manifest.KeyVersion)
		if err != nil {
			result.Errors = append(result.Errors, "decryption failed")
		} else {
			dumpBytes = decrypted
			result.Checks = append(result.Checks, "decryption_ok")
		}
	} else {
		dumpBytes = fileBytes
	}

	if len(dumpBytes) > 0 {
		header := string(dumpBytes[:min(60, len(dumpBytes))])
		if strings.Contains(header, "-- MySQL dump") || strings.Contains(header, "-- MariaDB dump") {
			result.Checks = append(result.Checks, "dump_structure_valid")
		} else {
			result.Errors = append(result.Errors, "missing MySQL dump header")
		}
	}

	result.Valid = len(result.Errors) == 0
	return result
}

// applyRetention deletes backup files older than retentionDays.
func (ts *testService) applyRetention() error {
	cutoff := time.Now().UTC().AddDate(0, 0, -ts.retentionDays)

	entries, err := os.ReadDir(ts.backupRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql.enc") {
			continue
		}
		fullPath := filepath.Join(ts.backupRoot, e.Name())
		manifestPath := strings.TrimSuffix(fullPath, ".sql.enc") + ".manifest.json"

		var createdAt time.Time
		if b, readErr := os.ReadFile(manifestPath); readErr == nil {
			var m BackupManifest
			if jsonErr := json.Unmarshal(b, &m); jsonErr == nil {
				createdAt = m.CreatedAt
			}
		}
		if createdAt.IsZero() {
			if info, infoErr := e.Info(); infoErr == nil {
				createdAt = info.ModTime()
			}
		}

		if createdAt.Before(cutoff) {
			_ = os.Remove(fullPath)
			_ = os.Remove(manifestPath)
		}
	}
	return nil
}

// listBackups scans the backup directory and returns BackupRecord entries.
func (ts *testService) listBackups() ([]BackupRecord, error) {
	entries, err := os.ReadDir(ts.backupRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var records []BackupRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql.enc") {
			continue
		}
		fullPath := filepath.Join(ts.backupRoot, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		rec := BackupRecord{
			Filename:  e.Name(),
			FilePath:  fullPath,
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime(),
			Encrypted: true,
		}
		manifestPath := strings.TrimSuffix(fullPath, ".sql.enc") + ".manifest.json"
		if b, readErr := os.ReadFile(manifestPath); readErr == nil {
			var m BackupManifest
			if jsonErr := json.Unmarshal(b, &m); jsonErr == nil {
				rec.CreatedAt = m.CreatedAt
				rec.Encrypted = m.Encrypted
				rec.ManifestPath = manifestPath
			}
		}
		records = append(records, rec)
	}
	return records, nil
}

// --- Tests ---

// TestManifestCreation verifies that a manifest is written with the correct checksum.
func TestManifestCreation(t *testing.T) {
	dir := t.TempDir()
	ts := newTestService(dir, 30, true)

	dumpContent := []byte("-- MySQL dump test content\n")
	backupPath := filepath.Join(dir, "2026-01-01-test.sql.enc")

	if err := ts.writeBackupFile(backupPath, dumpContent); err != nil {
		t.Fatalf("writeBackupFile: %v", err)
	}

	// Read back and verify manifest.
	manifestPath := strings.TrimSuffix(backupPath, ".sql.enc") + ".manifest.json"
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if manifest.BackupID == "" {
		t.Error("manifest BackupID should not be empty")
	}
	if manifest.FileHash == "" {
		t.Error("manifest FileHash should not be empty")
	}
	if manifest.FileSizeBytes == 0 {
		t.Error("manifest FileSizeBytes should be > 0")
	}
	if !manifest.Encrypted {
		t.Error("manifest.Encrypted should be true")
	}

	// Verify the checksum matches.
	fileBytes, _ := os.ReadFile(backupPath)
	hash := sha256.Sum256(fileBytes)
	wantHash := hex.EncodeToString(hash[:])
	if manifest.FileHash != wantHash {
		t.Errorf("FileHash mismatch: want %s, got %s", wantHash, manifest.FileHash)
	}
}

// TestChecksumValidation verifies that the checksum check detects file tampering.
func TestChecksumValidation(t *testing.T) {
	dir := t.TempDir()
	ts := newTestService(dir, 30, true)

	dump := []byte("-- MySQL dump\nSELECT 1;\n")
	backupPath := filepath.Join(dir, "2026-01-01-valid.sql.enc")
	if err := ts.writeBackupFile(backupPath, dump); err != nil {
		t.Fatalf("writeBackupFile: %v", err)
	}

	// Valid backup should pass all checks.
	result := ts.validateBackup(backupPath)
	if !result.Valid {
		t.Errorf("expected valid=true, got errors: %v", result.Errors)
	}
	if !sliceContains(result.Checks, "checksum_valid") {
		t.Errorf("expected checksum_valid in checks: %v", result.Checks)
	}
	if !sliceContains(result.Checks, "dump_structure_valid") {
		t.Errorf("expected dump_structure_valid in checks: %v", result.Checks)
	}

	// Tamper with the backup file (flip a byte).
	fileBytes, _ := os.ReadFile(backupPath)
	tampered := make([]byte, len(fileBytes))
	copy(tampered, fileBytes)
	if len(tampered) > 0 {
		tampered[0] ^= 0x01
	}
	tamperedPath := filepath.Join(dir, "2026-01-01-tampered.sql.enc")
	_ = os.WriteFile(tamperedPath, tampered, 0640)

	// Copy the original manifest to the tampered backup path.
	origManifestPath := strings.TrimSuffix(backupPath, ".sql.enc") + ".manifest.json"
	tamperedManifestPath := strings.TrimSuffix(tamperedPath, ".sql.enc") + ".manifest.json"
	origManifest, _ := os.ReadFile(origManifestPath)
	_ = os.WriteFile(tamperedManifestPath, origManifest, 0640)

	result2 := ts.validateBackup(tamperedPath)
	if result2.Valid {
		t.Error("expected invalid=true for tampered backup")
	}
	if !sliceContains(result2.Errors, "checksum mismatch") {
		t.Errorf("expected 'checksum mismatch' error, got: %v", result2.Errors)
	}
}

// TestRetentionPolicy verifies that files older than RetentionDays are deleted
// while newer files are preserved.
func TestRetentionPolicy(t *testing.T) {
	dir := t.TempDir()
	ts := newTestService(dir, 7, false) // 7-day retention

	// Old backup — created 10 days ago.
	oldFile := filepath.Join(dir, "2020-01-01-old.sql.enc")
	oldManifest := filepath.Join(dir, "2020-01-01-old.manifest.json")
	_ = os.WriteFile(oldFile, []byte("-- MySQL dump old\n"), 0640)
	oldManifestData, _ := json.Marshal(BackupManifest{
		BackupID:  "old",
		CreatedAt: time.Now().UTC().AddDate(0, 0, -10),
		Encrypted: false,
	})
	_ = os.WriteFile(oldManifest, oldManifestData, 0640)

	// New backup — created now.
	newFile := filepath.Join(dir, "2026-04-09-new.sql.enc")
	newManifest := filepath.Join(dir, "2026-04-09-new.manifest.json")
	_ = os.WriteFile(newFile, []byte("-- MySQL dump new\n"), 0640)
	newManifestData, _ := json.Marshal(BackupManifest{
		BackupID:  "new",
		CreatedAt: time.Now().UTC(),
		Encrypted: false,
	})
	_ = os.WriteFile(newManifest, newManifestData, 0640)

	if err := ts.applyRetention(); err != nil {
		t.Fatalf("applyRetention: %v", err)
	}

	// Old files should be deleted.
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old backup to be deleted")
	}
	if _, err := os.Stat(oldManifest); !os.IsNotExist(err) {
		t.Errorf("expected old manifest to be deleted")
	}

	// New files should still exist.
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Errorf("expected new backup to still exist")
	}
	if _, err := os.Stat(newManifest); os.IsNotExist(err) {
		t.Errorf("expected new manifest to still exist")
	}
}

// TestBackupRecordListing verifies that the directory scan returns the correct BackupRecord entries.
func TestBackupRecordListing(t *testing.T) {
	dir := t.TempDir()
	ts := newTestService(dir, 30, true)

	names := []string{"2026-04-08-backup1", "2026-04-09-backup2"}
	for _, name := range names {
		backupPath := filepath.Join(dir, name+".sql.enc")
		if err := ts.writeBackupFile(backupPath, []byte("-- MySQL dump\ntest\n")); err != nil {
			t.Fatalf("writeBackupFile %s: %v", name, err)
		}
	}

	records, err := ts.listBackups()
	if err != nil {
		t.Fatalf("listBackups: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 backup records, got %d", len(records))
	}
	for _, rec := range records {
		if rec.ManifestPath == "" {
			t.Errorf("expected manifest path to be set for %s", rec.Filename)
		}
		if !rec.Encrypted {
			t.Errorf("expected Encrypted=true for %s", rec.Filename)
		}
		if rec.SizeBytes == 0 {
			t.Errorf("expected SizeBytes > 0 for %s", rec.Filename)
		}
	}
}

// --- helpers ---

func sliceContains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
