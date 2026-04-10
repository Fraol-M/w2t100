package analytics

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"gorm.io/gorm"
)

// WriteWorkOrdersCSV writes work orders matching filters to w as UTF-8 CSV.
// Returns the number of records written and any error.
func WriteWorkOrdersCSV(w io.Writer, db *gorm.DB, filters AnalyticsFilters) (int, error) {
	cw := csv.NewWriter(w)

	headers := []string{
		"id", "uuid", "property_id", "unit_id", "tenant_id", "assigned_to",
		"title", "priority", "status", "issue_type", "skill_tag",
		"rating", "completed_at", "created_at",
	}
	if err := cw.Write(headers); err != nil {
		return 0, fmt.Errorf("csv: failed to write headers: %w", err)
	}

	type woRow struct {
		ID          uint64
		UUID        string
		PropertyID  uint64
		UnitID      *uint64
		TenantID    uint64
		AssignedTo  *uint64
		Title       string
		Priority    string
		Status      string
		IssueType   string
		SkillTag    string
		Rating      *uint8
		CompletedAt *time.Time
		CreatedAt   time.Time
	}

	q := db.Table("work_orders").
		Select("id, uuid, property_id, unit_id, tenant_id, assigned_to, title, priority, status, issue_type, skill_tag, rating, completed_at, created_at").
		Where("deleted_at IS NULL")
	q = applyWorkOrderFilters(q, filters)

	rows, err := q.Rows()
	if err != nil {
		return 0, fmt.Errorf("csv: query failed: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var r woRow
		if err := rows.Scan(&r.ID, &r.UUID, &r.PropertyID, &r.UnitID, &r.TenantID, &r.AssignedTo,
			&r.Title, &r.Priority, &r.Status, &r.IssueType, &r.SkillTag, &r.Rating,
			&r.CompletedAt, &r.CreatedAt); err != nil {
			return count, fmt.Errorf("csv: row scan failed: %w", err)
		}

		record := []string{
			fmt.Sprintf("%d", r.ID),
			r.UUID,
			fmt.Sprintf("%d", r.PropertyID),
			nullUint64(r.UnitID),
			fmt.Sprintf("%d", r.TenantID),
			nullUint64(r.AssignedTo),
			r.Title,
			r.Priority,
			r.Status,
			r.IssueType,
			r.SkillTag,
			nullUint8(r.Rating),
			nullTime(r.CompletedAt),
			r.CreatedAt.UTC().Format(time.RFC3339),
		}
		if err := cw.Write(record); err != nil {
			return count, fmt.Errorf("csv: write record failed: %w", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, err
	}

	cw.Flush()
	return count, cw.Error()
}

// WritePaymentsCSV writes payments matching filters to w as UTF-8 CSV.
func WritePaymentsCSV(w io.Writer, db *gorm.DB, filters AnalyticsFilters) (int, error) {
	cw := csv.NewWriter(w)

	headers := []string{
		"id", "uuid", "work_order_id", "tenant_id", "unit_id", "property_id",
		"kind", "amount", "currency", "status", "paid_at", "created_at",
	}
	if err := cw.Write(headers); err != nil {
		return 0, fmt.Errorf("csv: failed to write headers: %w", err)
	}

	type payRow struct {
		ID          uint64
		UUID        string
		WorkOrderID *uint64
		TenantID    *uint64
		UnitID      *uint64
		PropertyID  uint64
		Kind        string
		Amount      float64
		Currency    string
		Status      string
		PaidAt      *time.Time
		CreatedAt   time.Time
	}

	q := db.Table("payments").
		Select("id, uuid, work_order_id, tenant_id, unit_id, property_id, kind, amount, currency, status, paid_at, created_at")

	if filters.PropertyID != nil {
		q = q.Where("property_id = ?", *filters.PropertyID)
	}
	if !filters.From.IsZero() {
		q = q.Where("created_at >= ?", filters.From)
	}
	if !filters.To.IsZero() {
		q = q.Where("created_at <= ?", filters.To)
	}

	rows, err := q.Rows()
	if err != nil {
		return 0, fmt.Errorf("csv: query failed: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var r payRow
		if err := rows.Scan(&r.ID, &r.UUID, &r.WorkOrderID, &r.TenantID, &r.UnitID, &r.PropertyID,
			&r.Kind, &r.Amount, &r.Currency, &r.Status, &r.PaidAt, &r.CreatedAt); err != nil {
			return count, fmt.Errorf("csv: row scan failed: %w", err)
		}

		record := []string{
			fmt.Sprintf("%d", r.ID),
			r.UUID,
			nullUint64(r.WorkOrderID),
			nullUint64(r.TenantID),
			nullUint64(r.UnitID),
			fmt.Sprintf("%d", r.PropertyID),
			r.Kind,
			fmt.Sprintf("%.2f", r.Amount),
			r.Currency,
			r.Status,
			nullTime(r.PaidAt),
			r.CreatedAt.UTC().Format(time.RFC3339),
		}
		if err := cw.Write(record); err != nil {
			return count, fmt.Errorf("csv: write record failed: %w", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, err
	}

	cw.Flush()
	return count, cw.Error()
}

// WriteAuditLogsCSV writes audit log entries matching filters to w as UTF-8 CSV.
func WriteAuditLogsCSV(w io.Writer, db *gorm.DB, filters AnalyticsFilters) (int, error) {
	cw := csv.NewWriter(w)

	headers := []string{
		"id", "uuid", "actor_id", "action", "resource_type", "resource_id",
		"description", "ip_address", "request_id", "created_at",
	}
	if err := cw.Write(headers); err != nil {
		return 0, fmt.Errorf("csv: failed to write headers: %w", err)
	}

	type logRow struct {
		ID           uint64
		UUID         string
		ActorID      *uint64
		Action       string
		ResourceType string
		ResourceID   *uint64
		Description  string
		IPAddress    string
		RequestID    string
		CreatedAt    time.Time
	}

	q := db.Table("audit_logs").
		Select("id, uuid, actor_id, action, resource_type, resource_id, description, ip_address, request_id, created_at")

	if !filters.From.IsZero() {
		q = q.Where("created_at >= ?", filters.From)
	}
	if !filters.To.IsZero() {
		q = q.Where("created_at <= ?", filters.To)
	}

	rows, err := q.Rows()
	if err != nil {
		return 0, fmt.Errorf("csv: query failed: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var r logRow
		if err := rows.Scan(&r.ID, &r.UUID, &r.ActorID, &r.Action, &r.ResourceType, &r.ResourceID,
			&r.Description, &r.IPAddress, &r.RequestID, &r.CreatedAt); err != nil {
			return count, fmt.Errorf("csv: row scan failed: %w", err)
		}

		record := []string{
			fmt.Sprintf("%d", r.ID),
			r.UUID,
			nullUint64(r.ActorID),
			r.Action,
			r.ResourceType,
			nullUint64(r.ResourceID),
			r.Description,
			r.IPAddress,
			r.RequestID,
			r.CreatedAt.UTC().Format(time.RFC3339),
		}
		if err := cw.Write(record); err != nil {
			return count, fmt.Errorf("csv: write record failed: %w", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, err
	}

	cw.Flush()
	return count, cw.Error()
}

// --- Nullable helpers ---

func nullUint64(v *uint64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%d", *v)
}

func nullUint8(v *uint8) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%d", *v)
}

func nullTime(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}
