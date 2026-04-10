package health

import (
	"net/http"
	"os"

	"propertyops/backend/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LivenessResponse is the response body for GET /health/live.
type LivenessResponse struct {
	Status string `json:"status"`
}

// Check represents the outcome of a single readiness sub-check.
type Check struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ReadinessResponse is the response body for GET /health/ready.
type ReadinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]Check  `json:"checks"`
}

// RegisterRoutes mounts the liveness and readiness endpoints on the engine.
func RegisterRoutes(engine *gin.Engine, db *gorm.DB, cfg *config.Config) {
	engine.GET("/health/live", livenessHandler())
	engine.GET("/health/ready", readinessHandler(db, cfg))
}

// livenessHandler returns 200 if the process is alive — no dependencies checked.
func livenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, LivenessResponse{Status: "ok"})
	}
}

// readinessHandler performs all dependency checks and returns 200 when all pass,
// or 503 when one or more checks fail.
func readinessHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		checks := make(map[string]Check, 4)
		allOK := true

		// --- DB check ---
		dbCheck := checkDB(db)
		checks["db"] = dbCheck
		if dbCheck.Status != "ok" {
			allOK = false
		}

		// --- Storage root check ---
		storageCheck := checkDir("storage", cfg.Storage.Root)
		checks["storage"] = storageCheck
		if storageCheck.Status != "ok" {
			allOK = false
		}

		// --- Encryption key directory check ---
		keysCheck := checkDir("keys", cfg.Encryption.KeyDir)
		checks["keys"] = keysCheck
		if keysCheck.Status != "ok" {
			allOK = false
		}

		// --- Backup directory check ---
		backupCheck := checkDir("backup_dir", cfg.Storage.BackupRoot)
		checks["backup_dir"] = backupCheck
		if backupCheck.Status != "ok" {
			allOK = false
		}

		overallStatus := "ok"
		statusCode := http.StatusOK
		if !allOK {
			overallStatus = "degraded"
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, ReadinessResponse{
			Status: overallStatus,
			Checks: checks,
		})
	}
}

// checkDB pings the database and returns the result as a Check.
func checkDB(db *gorm.DB) Check {
	if err := db.Raw("SELECT 1").Error; err != nil {
		return Check{Status: "fail", Message: "database unreachable: " + err.Error()}
	}
	return Check{Status: "ok"}
}

// checkDir verifies that the given directory path is accessible via os.Stat.
func checkDir(name, path string) Check {
	if path == "" {
		return Check{Status: "fail", Message: name + " path not configured"}
	}
	if _, err := os.Stat(path); err != nil {
		return Check{Status: "fail", Message: err.Error()}
	}
	return Check{Status: "ok"}
}
