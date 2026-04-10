package payments

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"propertyops/backend/internal/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupReconTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("SQLite unavailable (CGO disabled or no C compiler): %v", err)
	}
	if err := db.AutoMigrate(&Payment{}, &PaymentApproval{}, &ReconciliationRun{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestReconciliation_DailyAggregation(t *testing.T) {
	db := setupReconTestDB(t)
	repo := NewRepository(db)

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Storage: config.StorageConfig{Root: tmpDir},
		Payment: config.PaymentConfig{IntentExpiryMinutes: 30, DualApprovalThreshold: 500},
	}

	recon := NewReconciliationService(repo, cfg)

	runDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

	// Create test payments for the day.
	db.Create(&Payment{
		UUID:       "settle-1",
		PropertyID: 1,
		Kind:       "SettlementPosting",
		Amount:     100.00,
		Currency:   "USD",
		Status:     "Settled",
		CreatedBy:  1,
		CreatedAt:  runDate.Add(2 * time.Hour),
	})
	db.Create(&Payment{
		UUID:       "settle-2",
		PropertyID: 1,
		Kind:       "SettlementPosting",
		Amount:     250.50,
		Currency:   "USD",
		Status:     "Settled",
		CreatedBy:  1,
		CreatedAt:  runDate.Add(4 * time.Hour),
	})
	db.Create(&Payment{
		UUID:       "reversal-1",
		PropertyID: 1,
		Kind:       "Reversal",
		Amount:     50.00,
		Currency:   "USD",
		Status:     "Settled",
		CreatedBy:  1,
		CreatedAt:  runDate.Add(6 * time.Hour),
	})

	run, err := recon.RunDaily(runDate, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if run.Status != "Completed" {
		t.Errorf("expected status Completed, got %s", run.Status)
	}

	// TotalActual = settlements(350.50) + makeups(0) - reversals(50) = 300.50
	expectedActual := 300.50
	if run.TotalActual != expectedActual {
		t.Errorf("expected total_actual %.2f, got %.2f", expectedActual, run.TotalActual)
	}

	if run.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestReconciliation_DiscrepancyDetection(t *testing.T) {
	db := setupReconTestDB(t)
	repo := NewRepository(db)

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Storage: config.StorageConfig{Root: tmpDir},
		Payment: config.PaymentConfig{IntentExpiryMinutes: 30, DualApprovalThreshold: 500},
	}

	recon := NewReconciliationService(repo, cfg)

	runDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

	// Create a paid intent (expected) for $500.
	db.Create(&Payment{
		UUID:       "intent-1",
		PropertyID: 1,
		Kind:       "Intent",
		Amount:     500.00,
		Currency:   "USD",
		Status:     "Paid",
		CreatedBy:  1,
		CreatedAt:  runDate.Add(1 * time.Hour),
	})

	// Create settlement for only $300 (discrepancy).
	db.Create(&Payment{
		UUID:       "settle-1",
		PropertyID: 1,
		Kind:       "SettlementPosting",
		Amount:     300.00,
		Currency:   "USD",
		Status:     "Settled",
		CreatedBy:  1,
		CreatedAt:  runDate.Add(2 * time.Hour),
	})

	run, err := recon.RunDaily(runDate, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected = 500, Actual = 300.
	if run.TotalExpected != 500.00 {
		t.Errorf("expected total_expected 500.00, got %.2f", run.TotalExpected)
	}
	if run.TotalActual != 300.00 {
		t.Errorf("expected total_actual 300.00, got %.2f", run.TotalActual)
	}
	if run.DiscrepancyCount < 1 {
		t.Errorf("expected at least 1 discrepancy, got %d", run.DiscrepancyCount)
	}

	// Verify summary JSON contains discrepancy info.
	var summary ReconciliationSummary
	if err := json.Unmarshal(run.Summary, &summary); err != nil {
		t.Fatalf("failed to unmarshal summary: %v", err)
	}
	if len(summary.Discrepancies) == 0 {
		t.Error("expected discrepancies in summary")
	}
}

func TestReconciliation_CSVGeneration(t *testing.T) {
	db := setupReconTestDB(t)
	repo := NewRepository(db)

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Storage: config.StorageConfig{Root: tmpDir},
		Payment: config.PaymentConfig{IntentExpiryMinutes: 30, DualApprovalThreshold: 500},
	}

	recon := NewReconciliationService(repo, cfg)

	runDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

	db.Create(&Payment{
		UUID:       "settle-csv-1",
		PropertyID: 1,
		Kind:       "SettlementPosting",
		Amount:     200.00,
		Currency:   "USD",
		Status:     "Settled",
		CreatedBy:  1,
		CreatedAt:  runDate.Add(2 * time.Hour),
	})

	run, err := recon.RunDaily(runDate, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check CSV file was created.
	if run.StatementFilePath == nil {
		t.Fatal("expected statement_file_path to be set")
	}

	csvPath := *run.StatementFilePath
	expectedPath := filepath.Join(tmpDir, "reconciliation", "2025-06-15.csv")
	if csvPath != expectedPath {
		t.Errorf("expected CSV path %s, got %s", expectedPath, csvPath)
	}

	// Read and validate CSV.
	file, err := os.Open(csvPath)
	if err != nil {
		t.Fatalf("failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}

	// Should have header + at least 1 data row.
	if len(records) < 2 {
		t.Errorf("expected at least 2 rows (header + data), got %d", len(records))
	}

	// Check header.
	header := records[0]
	if header[0] != "PaymentID" || header[1] != "Kind" || header[2] != "Amount" {
		t.Errorf("unexpected header: %v", header)
	}
}

func TestReconciliation_EmptyDay(t *testing.T) {
	db := setupReconTestDB(t)
	repo := NewRepository(db)

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Storage: config.StorageConfig{Root: tmpDir},
		Payment: config.PaymentConfig{IntentExpiryMinutes: 30, DualApprovalThreshold: 500},
	}

	recon := NewReconciliationService(repo, cfg)

	runDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

	run, err := recon.RunDaily(runDate, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if run.Status != "Completed" {
		t.Errorf("expected status Completed, got %s", run.Status)
	}
	if run.TotalActual != 0 {
		t.Errorf("expected total_actual 0, got %.2f", run.TotalActual)
	}
	if run.TotalExpected != 0 {
		t.Errorf("expected total_expected 0, got %.2f", run.TotalExpected)
	}
	if run.DiscrepancyCount != 0 {
		t.Errorf("expected 0 discrepancies, got %d", run.DiscrepancyCount)
	}
}
