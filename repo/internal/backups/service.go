package backups

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"propertyops/backend/internal/config"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SecurityService is the encryption interface required by this package.
type SecurityService interface {
	Encrypt(plaintext []byte) ([]byte, int, error)
	Decrypt(ciphertext []byte, keyVersion int) ([]byte, error)
}

// AuditLogger is the audit interface required by this package.
type AuditLogger interface {
	Log(actorID uint64, action, resourceType string, resourceID uint64, description, ip, requestID string)
}

// BackupRecord describes a backup file on disk.
type BackupRecord struct {
	Filename     string    `json:"filename"`
	FilePath     string    `json:"file_path"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
	Encrypted    bool      `json:"encrypted"`
	ManifestPath string    `json:"manifest_path,omitempty"`
}

// ValidationResult holds the outcome of a backup integrity check.
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Checks []string `json:"checks"`
	Errors []string `json:"errors,omitempty"`
}

// BackupManifest is the JSON manifest written alongside each backup file.
type BackupManifest struct {
	BackupID      string    `json:"backup_id"`
	CreatedAt     time.Time `json:"created_at"`
	DBName        string    `json:"db_name"`
	FileHash      string    `json:"file_hash"`      // SHA-256 of the encrypted file
	FileSizeBytes int64     `json:"file_size_bytes"`
	Encrypted     bool      `json:"encrypted"`
	KeyVersion    int       `json:"key_version,omitempty"`
}

// Service handles database backup operations.
type Service struct {
	db       *gorm.DB
	cfg      *config.Config
	security SecurityService
	audit    AuditLogger
}

// NewService creates a new backup Service.
func NewService(db *gorm.DB, cfg *config.Config, security SecurityService, audit AuditLogger) *Service {
	return &Service{
		db:       db,
		cfg:      cfg,
		security: security,
		audit:    audit,
	}
}

// CreateBackup runs mysqldump, encrypts the output, writes the encrypted dump and a
// manifest JSON file to cfg.Storage.BackupRoot, and returns a BackupRecord.
func (s *Service) CreateBackup(requestedBy uint64, ip, requestID string) (*BackupRecord, error) {
	if err := os.MkdirAll(s.cfg.Storage.BackupRoot, 0750); err != nil {
		return nil, fmt.Errorf("backup: cannot create backup directory: %w", err)
	}

	backupID := uuid.New().String()
	dateStr := time.Now().UTC().Format("2006-01-02")
	baseName := fmt.Sprintf("%s-%s", dateStr, backupID)

	// Run mysqldump.
	dumpBytes, err := s.runMysqldump()
	if err != nil {
		return nil, fmt.Errorf("backup: mysqldump failed: %w", err)
	}

	var fileBytes []byte
	encrypted := false
	var keyVersion int

	if s.cfg.Backup.EncryptionEnabled {
		enc, kv, encErr := s.security.Encrypt(dumpBytes)
		if encErr != nil {
			return nil, fmt.Errorf("backup: encryption failed: %w", encErr)
		}
		fileBytes = enc
		keyVersion = kv
		encrypted = true
	} else {
		fileBytes = dumpBytes
	}

	// Write backup file.
	backupFilename := baseName + ".sql.enc"
	backupPath := filepath.Join(s.cfg.Storage.BackupRoot, backupFilename)
	if err := os.WriteFile(backupPath, fileBytes, 0640); err != nil {
		return nil, fmt.Errorf("backup: failed to write backup file: %w", err)
	}

	// Compute SHA-256 of the written bytes.
	hash := sha256.Sum256(fileBytes)
	hashHex := hex.EncodeToString(hash[:])

	// Write manifest.
	manifest := BackupManifest{
		BackupID:      backupID,
		CreatedAt:     time.Now().UTC(),
		DBName:        s.cfg.DB.Name,
		FileHash:      hashHex,
		FileSizeBytes: int64(len(fileBytes)),
		Encrypted:     encrypted,
		KeyVersion:    keyVersion,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("backup: failed to marshal manifest: %w", err)
	}
	manifestFilename := baseName + ".manifest.json"
	manifestPath := filepath.Join(s.cfg.Storage.BackupRoot, manifestFilename)
	if err := os.WriteFile(manifestPath, manifestBytes, 0640); err != nil {
		return nil, fmt.Errorf("backup: failed to write manifest: %w", err)
	}

	record := &BackupRecord{
		Filename:     backupFilename,
		FilePath:     backupPath,
		SizeBytes:    int64(len(fileBytes)),
		CreatedAt:    manifest.CreatedAt,
		Encrypted:    encrypted,
		ManifestPath: manifestPath,
	}

	s.audit.Log(requestedBy, "Backup", "Database", 0,
		fmt.Sprintf("Created backup %s (encrypted=%v, size=%d bytes)", backupFilename, encrypted, record.SizeBytes),
		ip, requestID)

	return record, nil
}

// ValidateBackup reads a backup file, verifies the SHA-256 hash against the
// companion manifest, and checks that the (decrypted) dump starts with the
// expected MySQL dump header.
func (s *Service) ValidateBackup(backupPath string, requestedBy uint64, ip, requestID string) (*ValidationResult, error) {
	result := &ValidationResult{}

	fileBytes, err := os.ReadFile(backupPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cannot read backup file: %v", err))
		return result, nil
	}
	result.Checks = append(result.Checks, "backup_file_readable")

	// Derive manifest path from backup path.
	manifestPath := strings.TrimSuffix(backupPath, ".sql.enc") + ".manifest.json"
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cannot read manifest: %v", err))
		return result, nil
	}
	result.Checks = append(result.Checks, "manifest_readable")

	var manifest BackupManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid manifest JSON: %v", err))
		return result, nil
	}

	// Verify SHA-256.
	hash := sha256.Sum256(fileBytes)
	hashHex := hex.EncodeToString(hash[:])
	if hashHex != manifest.FileHash {
		result.Errors = append(result.Errors, fmt.Sprintf("checksum mismatch: got %s want %s", hashHex, manifest.FileHash))
	} else {
		result.Checks = append(result.Checks, "checksum_valid")
	}

	// Decrypt if needed and verify dump structure.
	var dumpBytes []byte
	if manifest.Encrypted {
		decrypted, decErr := s.security.Decrypt(fileBytes, manifest.KeyVersion)
		if decErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("decryption failed: %v", decErr))
		} else {
			dumpBytes = decrypted
			result.Checks = append(result.Checks, "decryption_ok")
		}
	} else {
		dumpBytes = fileBytes
	}

	if len(dumpBytes) > 0 {
		header := string(dumpBytes[:min(50, len(dumpBytes))])
		if strings.Contains(header, "-- MySQL dump") || strings.Contains(header, "-- MariaDB dump") {
			result.Checks = append(result.Checks, "dump_structure_valid")
		} else {
			result.Errors = append(result.Errors, "dump does not contain expected MySQL dump header")
		}
	}

	result.Valid = len(result.Errors) == 0

	s.audit.Log(requestedBy, "Restore", "Database", 0,
		fmt.Sprintf("Validated backup %s: valid=%v checks=%v errors=%v",
			filepath.Base(backupPath), result.Valid, result.Checks, result.Errors),
		ip, requestID)

	return result, nil
}

// ListBackups scans the backup directory and returns all recognised backup records.
func (s *Service) ListBackups() ([]BackupRecord, error) {
	entries, err := os.ReadDir(s.cfg.Storage.BackupRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupRecord{}, nil
		}
		return nil, fmt.Errorf("backup: failed to read backup directory: %w", err)
	}

	var records []BackupRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql.enc") {
			continue
		}
		fullPath := filepath.Join(s.cfg.Storage.BackupRoot, e.Name())
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

		// Try to load manifest for richer metadata.
		manifestPath := strings.TrimSuffix(fullPath, ".sql.enc") + ".manifest.json"
		if manifestBytes, err := os.ReadFile(manifestPath); err == nil {
			var m BackupManifest
			if err := json.Unmarshal(manifestBytes, &m); err == nil {
				rec.CreatedAt = m.CreatedAt
				rec.Encrypted = m.Encrypted
				rec.ManifestPath = manifestPath
			}
		}

		records = append(records, rec)
	}
	return records, nil
}

// ApplyRetention deletes backup files (and their manifests) older than
// cfg.Backup.RetentionDays days.
func (s *Service) ApplyRetention() error {
	cutoff := time.Now().UTC().AddDate(0, 0, -s.cfg.Backup.RetentionDays)

	records, err := s.ListBackups()
	if err != nil {
		return err
	}

	var lastErr error
	for _, rec := range records {
		if rec.CreatedAt.Before(cutoff) {
			if err := os.Remove(rec.FilePath); err != nil && !os.IsNotExist(err) {
				log.Printf("WARN backup retention: failed to remove %s: %v", rec.FilePath, err)
				lastErr = err
				continue
			}
			if rec.ManifestPath != "" {
				if err := os.Remove(rec.ManifestPath); err != nil && !os.IsNotExist(err) {
					log.Printf("WARN backup retention: failed to remove manifest %s: %v", rec.ManifestPath, err)
				}
			}
			log.Printf("INFO backup retention: removed %s (created %s)", rec.Filename, rec.CreatedAt.Format(time.RFC3339))
		}
	}
	return lastErr
}

// --- internal helpers ---

// runMysqldump executes mysqldump and returns the dump output.
// Returns an error if mysqldump is not available; a metadata-only fallback is not
// acceptable for production backups as it cannot be used for data recovery.
func (s *Service) runMysqldump() ([]byte, error) {
	mysqldumpPath, err := exec.LookPath("mysqldump")
	if err != nil {
		return nil, fmt.Errorf("mysqldump not found in PATH — install mysql-client to enable backups: %w", err)
	}

	args := []string{
		"--host=" + s.cfg.DB.Host,
		fmt.Sprintf("--port=%d", s.cfg.DB.Port),
		"--user=" + s.cfg.DB.User,
		"--password=" + s.cfg.DB.Password,
		"--single-transaction",
		"--routines",
		"--triggers",
		"--set-gtid-purged=OFF",
		s.cfg.DB.Name,
	}

	cmd := exec.Command(mysqldumpPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("mysqldump failed: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
