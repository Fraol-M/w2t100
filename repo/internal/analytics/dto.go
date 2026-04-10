package analytics

import "time"

// --- Metric response types ---

// PopularityMetric holds the count of work orders per issue type.
type PopularityMetric struct {
	IssueType string `json:"issue_type"`
	Count     int64  `json:"count"`
}

// FunnelMetric holds work-order counts broken down by lifecycle status.
type FunnelMetric struct {
	New        int64 `json:"new"`
	Assigned   int64 `json:"assigned"`
	InProgress int64 `json:"in_progress"`
	Completed  int64 `json:"completed"`
	Total      int64 `json:"total"`
}

// RetentionMetric captures repeat-request behaviour within time windows.
type RetentionMetric struct {
	UniqueUnits30d  int64   `json:"unique_units_30d"`
	RepeatUnits30d  int64   `json:"repeat_units_30d"`
	RepeatRate30d   float64 `json:"repeat_rate_30d"`
	UniqueUnits90d  int64   `json:"unique_units_90d"`
	RepeatUnits90d  int64   `json:"repeat_units_90d"`
	RepeatRate90d   float64 `json:"repeat_rate_90d"`
}

// TagMetric holds the usage count for a single tag or skill tag.
type TagMetric struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

// QualityMetric aggregates feedback quality indicators.
type QualityMetric struct {
	TotalRated        int64   `json:"total_rated"`
	AverageRating     float64 `json:"average_rating"`
	NegativeCount     int64   `json:"negative_count"`    // rating <= 2
	NegativeRate      float64 `json:"negative_rate"`     // negative / total_rated
	ReportedCount     int64   `json:"reported_count"`    // governance reports linked to work orders
}

// --- Filter & request types ---

// AnalyticsFilters are the common query parameters accepted by all analytics endpoints.
type AnalyticsFilters struct {
	PropertyID *uint64   `form:"property_id"`
	From       time.Time `form:"from"  time_format:"2006-01-02"`
	To         time.Time `form:"to"    time_format:"2006-01-02"`
	Period     string    `form:"period"` // e.g. "30d", "90d", "1y"
}

// ExportRequest specifies what data to export and in what format.
type ExportRequest struct {
	Type    string `json:"type"    binding:"required"` // work_orders | payments | audit_logs
	Format  string `json:"format"`                     // CSV (default)
	Purpose string `json:"purpose"`                    // required for PII types
}

// ListReportsRequest holds pagination parameters for listing saved reports.
type ListReportsRequest struct {
	Page    int `form:"page"`
	PerPage int `form:"per_page"`
}

// --- Saved report DTO ---

// SavedReportRequest is the payload for creating a saved report.
type SavedReportRequest struct {
	Name         string      `json:"name"          binding:"required"`
	ReportType   string      `json:"report_type"   binding:"required"`
	Filters      interface{} `json:"filters"`
	OutputFormat string      `json:"output_format"` // defaults to CSV
	Schedule     string      `json:"schedule"`
}

// SavedReportResponse is the API response for a saved report.
type SavedReportResponse struct {
	ID              uint64     `json:"id"`
	UUID            string     `json:"uuid"`
	Name            string     `json:"name"`
	ReportType      string     `json:"report_type"`
	OutputFormat    string     `json:"output_format"`
	Schedule        string     `json:"schedule,omitempty"`
	OwnerID         uint64     `json:"owner_id"`
	IsActive        bool       `json:"is_active"`
	LastGeneratedAt *time.Time `json:"last_generated_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ToSavedReportResponse converts a model to a response DTO.
func ToSavedReportResponse(r *SavedReport) SavedReportResponse {
	return SavedReportResponse{
		ID:              r.ID,
		UUID:            r.UUID,
		Name:            r.Name,
		ReportType:      r.ReportType,
		OutputFormat:    r.OutputFormat,
		Schedule:        r.Schedule,
		OwnerID:         r.OwnerID,
		IsActive:        r.IsActive,
		LastGeneratedAt: r.LastGeneratedAt,
		CreatedAt:       r.CreatedAt,
	}
}

// --- Generated report DTO ---

// GeneratedReportResponse is the API response for a generated report record.
type GeneratedReportResponse struct {
	ID            uint64    `json:"id"`
	UUID          string    `json:"uuid"`
	SavedReportID *uint64   `json:"saved_report_id,omitempty"`
	ReportType    string    `json:"report_type"`
	Format        string    `json:"format"`
	StoragePath   string    `json:"storage_path"`
	FileSize      uint64    `json:"file_size"`
	RecordCount   int       `json:"record_count"`
	GeneratedBy   uint64    `json:"generated_by"`
	GeneratedAt   time.Time `json:"generated_at"`
}

// ToGeneratedReportResponse converts a model to a response DTO.
func ToGeneratedReportResponse(r *GeneratedReport) GeneratedReportResponse {
	return GeneratedReportResponse{
		ID:            r.ID,
		UUID:          r.UUID,
		SavedReportID: r.SavedReportID,
		ReportType:    r.ReportType,
		Format:        r.Format,
		StoragePath:   r.StoragePath,
		FileSize:      r.FileSize,
		RecordCount:   r.RecordCount,
		GeneratedBy:   r.GeneratedBy,
		GeneratedAt:   r.GeneratedAt,
	}
}
