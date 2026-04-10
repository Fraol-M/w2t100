package analytics

import (
	"time"

	"gorm.io/datatypes"
)

// SavedReport is a persisted report configuration that can be run on demand or
// on a schedule.
type SavedReport struct {
	ID              uint64         `gorm:"primaryKey"            json:"id"`
	UUID            string         `gorm:"size:36;uniqueIndex"   json:"uuid"`
	Name            string         `gorm:"size:255"              json:"name"`
	ReportType      string         `gorm:"size:50"               json:"report_type"`
	Filters         datatypes.JSON `gorm:"type:json"             json:"filters,omitempty"`
	OutputFormat    string         `gorm:"size:20;default:CSV"   json:"output_format"`
	Schedule        string         `gorm:"size:100"              json:"schedule,omitempty"`
	OwnerID         uint64         `gorm:"index"                 json:"owner_id"`
	IsActive        bool           `gorm:"default:true"          json:"is_active"`
	LastGeneratedAt *time.Time     `                             json:"last_generated_at,omitempty"`
	CreatedAt       time.Time      `                             json:"created_at"`
	UpdatedAt       time.Time      `                             json:"updated_at"`
}

// TableName overrides the GORM default.
func (SavedReport) TableName() string { return "saved_reports" }

// GeneratedReport tracks every file produced by running a SavedReport or an
// ad-hoc export.
type GeneratedReport struct {
	ID            uint64         `gorm:"primaryKey"             json:"id"`
	UUID          string         `gorm:"size:36;uniqueIndex"    json:"uuid"`
	SavedReportID *uint64        `gorm:"index"                  json:"saved_report_id,omitempty"`
	ReportType    string         `gorm:"size:50;index"          json:"report_type"`
	Format        string         `gorm:"size:20;default:CSV"    json:"format"`
	StoragePath   string         `gorm:"size:512"               json:"storage_path"`
	FileSize      uint64         `                              json:"file_size"`
	RecordCount   int            `                              json:"record_count"`
	Parameters    datatypes.JSON `gorm:"type:json"              json:"parameters,omitempty"`
	GeneratedBy   uint64         `                              json:"generated_by"`
	GeneratedAt   time.Time      `gorm:"index"                  json:"generated_at"`
}

// TableName overrides the GORM default.
func (GeneratedReport) TableName() string { return "generated_reports" }
