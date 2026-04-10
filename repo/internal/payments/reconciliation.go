package payments

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"propertyops/backend/internal/config"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ReconciliationService handles daily reconciliation runs and statement generation.
type ReconciliationService struct {
	repo *Repository
	cfg  *config.Config
}

// NewReconciliationService creates a new ReconciliationService.
func NewReconciliationService(repo *Repository, cfg *config.Config) *ReconciliationService {
	return &ReconciliationService{
		repo: repo,
		cfg:  cfg,
	}
}

// ReconciliationSummary holds the details of a reconciliation run.
type ReconciliationSummary struct {
	Date             string                `json:"date"`
	TotalSettlements float64               `json:"total_settlements"`
	TotalReversals   float64               `json:"total_reversals"`
	TotalMakeups     float64               `json:"total_makeups"`
	NetAmount        float64               `json:"net_amount"`
	Discrepancies    []ReconciliationEntry `json:"discrepancies,omitempty"`
}

// ReconciliationEntry represents a single line item in the reconciliation report.
type ReconciliationEntry struct {
	PaymentID  uint64  `json:"payment_id"`
	Kind       string  `json:"kind"`
	Amount     float64 `json:"amount"`
	PropertyID uint64  `json:"property_id"`
	Issue      string  `json:"issue,omitempty"`
}

// RunDaily queries all settlements/postings for the given date, computes totals,
// identifies discrepancies, generates a CSV statement, and creates a ReconciliationRun record.
func (rs *ReconciliationService) RunDaily(date time.Time, generatedBy uint64) (*ReconciliationRun, error) {
	now := time.Now().UTC()

	run := &ReconciliationRun{
		UUID:        uuid.New().String(),
		RunDate:     date,
		Status:      "Running",
		GeneratedBy: generatedBy,
		StartedAt:   now,
	}

	if err := rs.repo.CreateReconciliation(run); err != nil {
		return nil, fmt.Errorf("failed to create reconciliation run: %w", err)
	}

	// Query all settlement and posting payments for the date.
	kinds := []string{"SettlementPosting", "MakeupPosting", "Reversal"}
	payments, err := rs.repo.FindByDateAndKinds(date, kinds)
	if err != nil {
		run.Status = "Failed"
		rs.repo.UpdateReconciliation(run)
		return nil, fmt.Errorf("failed to query payments: %w", err)
	}

	// Also query paid intents for the day to compute expected total.
	allKinds := []string{"SettlementPosting", "MakeupPosting", "Reversal", "Intent"}
	allPayments, err := rs.repo.FindByDateAndKinds(date, allKinds)
	if err != nil {
		allPayments = payments // fallback
	}

	// Compute totals.
	var totalSettlements, totalReversals, totalMakeups, totalExpected float64
	var entries []ReconciliationEntry
	var discrepancies []ReconciliationEntry

	for _, p := range payments {
		entry := ReconciliationEntry{
			PaymentID:  p.ID,
			Kind:       p.Kind,
			Amount:     p.Amount,
			PropertyID: p.PropertyID,
		}

		switch p.Kind {
		case "SettlementPosting":
			totalSettlements += p.Amount
		case "Reversal":
			totalReversals += p.Amount
		case "MakeupPosting":
			totalMakeups += p.Amount
		}

		entries = append(entries, entry)
	}

	// Expected total from intents that have been paid or subsequently settled (approved).
	// After dual-approval, ApprovePayment transitions the intent to "Settled", so both
	// statuses must be included to avoid systematically under-counting expected revenue.
	for _, p := range allPayments {
		if p.Kind == "Intent" && (p.Status == "Paid" || p.Status == "Settled") {
			totalExpected += p.Amount
		}
	}

	// Actual = settlements + makeups - reversals.
	totalActual := totalSettlements + totalMakeups - totalReversals

	// Check for discrepancies: if expected != actual (with tolerance).
	discrepancyCount := 0
	if math.Abs(totalExpected-totalActual) > 0.01 {
		discrepancies = append(discrepancies, ReconciliationEntry{
			Kind:   "Summary",
			Amount: totalExpected - totalActual,
			Issue:  fmt.Sprintf("Expected $%.2f but actual is $%.2f", totalExpected, totalActual),
		})
		discrepancyCount++
	}

	// Check for individual payment issues (e.g., reversals without matching settlement).
	for _, p := range payments {
		if p.Kind == "Reversal" && p.RelatedPaymentID == nil {
			discrepancies = append(discrepancies, ReconciliationEntry{
				PaymentID:  p.ID,
				Kind:       p.Kind,
				Amount:     p.Amount,
				PropertyID: p.PropertyID,
				Issue:      "Reversal without related payment",
			})
			discrepancyCount++
		}
	}

	summary := ReconciliationSummary{
		Date:             date.Format("2006-01-02"),
		TotalSettlements: totalSettlements,
		TotalReversals:   totalReversals,
		TotalMakeups:     totalMakeups,
		NetAmount:        totalActual,
		Discrepancies:    discrepancies,
	}

	summaryJSON, _ := json.Marshal(summary)

	// Generate CSV statement.
	csvPath, err := rs.GenerateStatement(date, entries, discrepancies)
	if err != nil {
		// Non-fatal: continue but note the error.
		csvPath = ""
	}

	completedAt := time.Now().UTC()
	run.TotalExpected = totalExpected
	run.TotalActual = totalActual
	run.DiscrepancyCount = discrepancyCount
	run.Summary = datatypes.JSON(summaryJSON)
	run.Status = "Completed"
	run.CompletedAt = &completedAt

	if csvPath != "" {
		run.StatementFilePath = &csvPath
	}

	if err := rs.repo.UpdateReconciliation(run); err != nil {
		return nil, fmt.Errorf("failed to update reconciliation run: %w", err)
	}

	return run, nil
}

// GenerateStatement writes a CSV file to STORAGE_ROOT/reconciliation/YYYY-MM-DD.csv.
func (rs *ReconciliationService) GenerateStatement(date time.Time, entries []ReconciliationEntry, discrepancies []ReconciliationEntry) (string, error) {
	storageRoot := "/var/lib/propertyops/storage"
	if rs.cfg != nil && rs.cfg.Storage.Root != "" {
		storageRoot = rs.cfg.Storage.Root
	}

	dir := filepath.Join(storageRoot, "reconciliation")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create reconciliation directory: %w", err)
	}

	filename := fmt.Sprintf("%s.csv", date.Format("2006-01-02"))
	fullPath := filepath.Join(dir, filename)

	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Header.
	writer.Write([]string{"PaymentID", "Kind", "Amount", "PropertyID", "Issue"})

	// Entries.
	for _, e := range entries {
		writer.Write([]string{
			fmt.Sprintf("%d", e.PaymentID),
			e.Kind,
			fmt.Sprintf("%.2f", e.Amount),
			fmt.Sprintf("%d", e.PropertyID),
			e.Issue,
		})
	}

	// Discrepancies section.
	if len(discrepancies) > 0 {
		writer.Write([]string{"", "", "", "", ""})
		writer.Write([]string{"--- DISCREPANCIES ---", "", "", "", ""})
		for _, d := range discrepancies {
			writer.Write([]string{
				fmt.Sprintf("%d", d.PaymentID),
				d.Kind,
				fmt.Sprintf("%.2f", d.Amount),
				fmt.Sprintf("%d", d.PropertyID),
				d.Issue,
			})
		}
	}

	return fullPath, nil
}
