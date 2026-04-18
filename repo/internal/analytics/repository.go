package analytics

import (
	"log"
	"sort"
	"time"

	"gorm.io/gorm"
)

// Repository handles all database queries for the analytics package.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new analytics Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// PopularityMetrics returns work-order counts grouped by issue_type, ordered by
// count descending.
func (r *Repository) PopularityMetrics(filters AnalyticsFilters) ([]PopularityMetric, error) {
	q := r.db.Table("work_orders").
		Select("issue_type, COUNT(*) as count").
		Where("deleted_at IS NULL AND issue_type != ''")

	q = applyWorkOrderFilters(q, filters)

	q = q.Group("issue_type").Order("count DESC")

	results := make([]PopularityMetric, 0)
	if err := q.Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// ConversionFunnel returns work-order counts for the main lifecycle statuses.
func (r *Repository) ConversionFunnel(filters AnalyticsFilters) (*FunnelMetric, error) {
	type row struct {
		Status string
		Count  int64
	}

	q := r.db.Table("work_orders").
		Select("status, COUNT(*) as count").
		Where("deleted_at IS NULL")

	q = applyWorkOrderFilters(q, filters)
	q = q.Group("status")

	var rows []row
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}

	m := &FunnelMetric{}
	for _, r := range rows {
		switch r.Status {
		case "New":
			m.New = r.Count
		case "Assigned":
			m.Assigned = r.Count
		case "InProgress":
			m.InProgress = r.Count
		case "Completed":
			m.Completed = r.Count
		}
		m.Total += r.Count
	}
	return m, nil
}

// RetentionMetrics returns repeat-request counts for 30-day and 90-day windows.
func (r *Repository) RetentionMetrics(filters AnalyticsFilters) (*RetentionMetric, error) {
	type unitCount struct {
		UnitID *uint64
		Count  int64
	}

	retentionForDays := func(days int) (unique int64, repeat int64, err error) {
		var rows []unitCount

		q := r.db.Table("work_orders").
			Select("unit_id, COUNT(*) as count").
			Where("deleted_at IS NULL AND unit_id IS NOT NULL")

		if !filters.From.IsZero() {
			q = q.Where("created_at >= ?", filters.From)
		} else {
			// Using dynamic string for sqlite compatibility as well, although in Go we can just compute the date
			fromDate := time.Now().AddDate(0, 0, -days)
			q = q.Where("created_at >= ?", fromDate)
		}
		if !filters.To.IsZero() {
			q = q.Where("created_at <= ?", filters.To)
		}
		if filters.PropertyID != nil {
			q = q.Where("property_id = ?", *filters.PropertyID)
		}

		q = q.Group("unit_id")

		if err = q.Scan(&rows).Error; err != nil {
			return 0, 0, err
		}

		for _, row := range rows {
			unique++
			if row.Count > 1 {
				repeat++
			}
		}
		return unique, repeat, nil
	}

	unique30, repeat30, err := retentionForDays(30)
	if err != nil {
		return nil, err
	}
	unique90, repeat90, err := retentionForDays(90)
	if err != nil {
		return nil, err
	}

	m := &RetentionMetric{
		UniqueUnits30d: unique30,
		RepeatUnits30d: repeat30,
		UniqueUnits90d: unique90,
		RepeatUnits90d: repeat90,
	}
	if unique30 > 0 {
		m.RepeatRate30d = float64(repeat30) / float64(unique30)
	}
	if unique90 > 0 {
		m.RepeatRate90d = float64(repeat90) / float64(unique90)
	}
	return m, nil
}

// TagAnalysis returns the most-used tags across work orders.
// It combines skill_tag (plain column) with individual terms from the JSON tags array.
func (r *Repository) TagAnalysis(filters AnalyticsFilters) ([]TagMetric, error) {
	// --- skill_tag column ---
	q := r.db.Table("work_orders").
		Select("skill_tag as tag, COUNT(*) as count").
		Where("deleted_at IS NULL AND skill_tag != ''")
	q = applyWorkOrderFilters(q, filters)
	q = q.Group("skill_tag").Order("count DESC").Limit(50)

	skillTagResults := make([]TagMetric, 0)
	if err := q.Scan(&skillTagResults).Error; err != nil {
		return nil, err
	}

	// --- JSON tags array (MySQL 8.0+ JSON_TABLE) ---
	// Build WHERE clause and args matching applyWorkOrderFilters.
	whereClause := "work_orders.deleted_at IS NULL AND work_orders.tags IS NOT NULL AND JSON_LENGTH(work_orders.tags) > 0"
	jsonArgs := []interface{}{}
	if filters.PropertyID != nil {
		whereClause += " AND work_orders.property_id = ?"
		jsonArgs = append(jsonArgs, *filters.PropertyID)
	}
	if !filters.From.IsZero() {
		whereClause += " AND work_orders.created_at >= ?"
		jsonArgs = append(jsonArgs, filters.From)
	}
	if !filters.To.IsZero() {
		whereClause += " AND work_orders.created_at <= ?"
		jsonArgs = append(jsonArgs, filters.To)
	}

	jsonTagSQL := `
		SELECT jt.tag, COUNT(*) as count
		FROM work_orders, JSON_TABLE(work_orders.tags, '$[*]' COLUMNS(tag VARCHAR(255) PATH '$')) jt
		WHERE ` + whereClause + `
		GROUP BY jt.tag
		ORDER BY count DESC
		LIMIT 50`

	jsonTagResults := make([]TagMetric, 0)
	if err := r.db.Raw(jsonTagSQL, jsonArgs...).Scan(&jsonTagResults).Error; err != nil {
		// JSON_TABLE requires MySQL 8.0+; log and fall back to skill_tag results only.
		log.Printf("WARN analytics: JSON tag analysis skipped (requires MySQL 8.0+): %v", err)
		return skillTagResults, nil
	}

	// Merge both sources, summing counts for tags that appear in both.
	counts := make(map[string]int64, len(skillTagResults)+len(jsonTagResults))
	for _, m := range skillTagResults {
		counts[m.Tag] += m.Count
	}
	for _, m := range jsonTagResults {
		if m.Tag != "" {
			counts[m.Tag] += m.Count
		}
	}

	merged := make([]TagMetric, 0, len(counts))
	for tag, count := range counts {
		merged = append(merged, TagMetric{Tag: tag, Count: count})
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Count > merged[j].Count
	})
	if len(merged) > 50 {
		merged = merged[:50]
	}
	return merged, nil
}

// QualityMetrics returns aggregated rating and reporting quality data.
func (r *Repository) QualityMetrics(filters AnalyticsFilters) (*QualityMetric, error) {
	type ratingAgg struct {
		TotalRated    int64
		AvgRating     float64
		NegativeCount int64
	}

	var agg ratingAgg
	q := r.db.Table("work_orders").
		Select("COUNT(rating) as total_rated, AVG(rating) as avg_rating, SUM(CASE WHEN rating <= 2 THEN 1 ELSE 0 END) as negative_count").
		Where("deleted_at IS NULL AND rating IS NOT NULL")

	q = applyWorkOrderFilters(q, filters)

	if err := q.Scan(&agg).Error; err != nil {
		return nil, err
	}

	// Count governance reports linked to work orders.
	var reportedCount int64
	rq := r.db.Table("reports").
		Where("target_type = 'WorkOrder'")
	if filters.PropertyID != nil {
		rq = rq.Where("EXISTS (SELECT 1 FROM work_orders wo WHERE wo.id = reports.target_id AND wo.property_id = ?)", *filters.PropertyID)
	}
	_ = rq.Count(&reportedCount).Error // best-effort

	m := &QualityMetric{
		TotalRated:    agg.TotalRated,
		AverageRating: agg.AvgRating,
		NegativeCount: agg.NegativeCount,
		ReportedCount: reportedCount,
	}
	if agg.TotalRated > 0 {
		m.NegativeRate = float64(agg.NegativeCount) / float64(agg.TotalRated)
	}
	return m, nil
}

// --- Saved report CRUD ---

// CreateSavedReport inserts a new SavedReport.
func (r *Repository) CreateSavedReport(report *SavedReport) error {
	return r.db.Create(report).Error
}

// FindSavedReport fetches a SavedReport by primary key.
func (r *Repository) FindSavedReport(id uint64) (*SavedReport, error) {
	var report SavedReport
	if err := r.db.First(&report, id).Error; err != nil {
		return nil, err
	}
	return &report, nil
}

// ListSavedReports returns a paginated list of reports owned by ownerID.
func (r *Repository) ListSavedReports(ownerID uint64, page, perPage int) ([]SavedReport, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	var total int64
	q := r.db.Model(&SavedReport{}).Where("owner_id = ?", ownerID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	reports := make([]SavedReport, 0)
	if err := q.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&reports).Error; err != nil {
		return nil, 0, err
	}
	return reports, total, nil
}

// UpdateSavedReport saves changes to an existing SavedReport.
func (r *Repository) UpdateSavedReport(report *SavedReport) error {
	return r.db.Save(report).Error
}

// DeleteSavedReport hard-deletes a SavedReport.
func (r *Repository) DeleteSavedReport(id uint64) error {
	return r.db.Delete(&SavedReport{}, id).Error
}

// --- Generated report CRUD ---

// CreateGeneratedReport inserts a new GeneratedReport record.
func (r *Repository) CreateGeneratedReport(report *GeneratedReport) error {
	return r.db.Create(report).Error
}

// FindGeneratedReport fetches a GeneratedReport by primary key.
func (r *Repository) FindGeneratedReport(id uint64) (*GeneratedReport, error) {
	var report GeneratedReport
	if err := r.db.First(&report, id).Error; err != nil {
		return nil, err
	}
	return &report, nil
}

// ListGeneratedReports returns a paginated list of generated reports, optionally
// filtered by savedID.
func (r *Repository) ListGeneratedReports(savedID *uint64, page, perPage int) ([]GeneratedReport, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	q := r.db.Model(&GeneratedReport{})
	if savedID != nil {
		q = q.Where("saved_report_id = ?", *savedID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	reports := make([]GeneratedReport, 0)
	if err := q.Order("generated_at DESC").Offset(offset).Limit(perPage).Find(&reports).Error; err != nil {
		return nil, 0, err
	}
	return reports, total, nil
}

// --- helpers ---

// applyWorkOrderFilters appends common filter clauses to a work_orders query.
func applyWorkOrderFilters(q *gorm.DB, f AnalyticsFilters) *gorm.DB {
	if f.PropertyID != nil {
		q = q.Where("property_id = ?", *f.PropertyID)
	}
	if !f.From.IsZero() {
		q = q.Where("created_at >= ?", f.From)
	}
	if !f.To.IsZero() {
		q = q.Where("created_at <= ?", f.To)
	}
	return q
}
