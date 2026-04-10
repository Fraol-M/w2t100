package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"propertyops/backend/internal/config"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var version = "dev"

func main() {
	seedFlag := flag.Bool("seed", false, "Run seed data after migrations")
	downFlag := flag.Bool("down", false, "Run down migrations (rollback)")
	flag.Parse()

	log.Printf("PropertyOps Migrate %s", version)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := gorm.Open(mysql.Open(cfg.DB.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying DB: %v", err)
	}
	defer sqlDB.Close()

	migrationDir := findMigrationDir()

	if *downFlag {
		if err := runDownMigrations(db, migrationDir); err != nil {
			log.Fatalf("Down migrations failed: %v", err)
		}
		log.Println("Down migrations completed successfully")
		return
	}

	if err := runUpMigrations(db, migrationDir); err != nil {
		log.Fatalf("Migrations failed: %v", err)
	}
	log.Println("Migrations completed successfully")

	if *seedFlag {
		if err := runSeeds(db); err != nil {
			log.Fatalf("Seeding failed: %v", err)
		}
		log.Println("Seeding completed successfully")
	}
}

func findMigrationDir() string {
	candidates := []string{
		"/app/migrations",
		"db/migrations",
		"../../db/migrations",
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	log.Fatal("Migration directory not found")
	return ""
}

func runUpMigrations(db *gorm.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}
	sort.Strings(files)

	// Create migration tracking table
	db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)

	for _, file := range files {
		version := extractVersion(file)
		var count int64
		db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Count(&count)
		if count > 0 {
			log.Printf("Skipping already applied migration: %s", filepath.Base(file))
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		statements := splitSQL(string(content))
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if err := db.Exec(stmt).Error; err != nil {
				return fmt.Errorf("migration %s failed: %w\nStatement: %s", filepath.Base(file), err, stmt)
			}
		}

		db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version)
		log.Printf("Applied migration: %s", filepath.Base(file))
	}
	return nil
}

func runDownMigrations(db *gorm.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.down.sql"))
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		statements := splitSQL(string(content))
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if err := db.Exec(stmt).Error; err != nil {
				return fmt.Errorf("down migration %s failed: %w\nStatement: %s", filepath.Base(file), err, stmt)
			}
		}

		version := extractVersion(file)
		db.Exec("DELETE FROM schema_migrations WHERE version = ?", version)
		log.Printf("Rolled back migration: %s", filepath.Base(file))
	}
	return nil
}

func runSeeds(db *gorm.DB) error {
	// Seed default roles
	roles := []string{"Tenant", "Technician", "PropertyManager", "ComplianceReviewer", "SystemAdmin"}
	for _, role := range roles {
		db.Exec("INSERT IGNORE INTO roles (name, description, created_at, updated_at) VALUES (?, ?, NOW(), NOW())", role, role+" role")
	}

	// Seed default admin user.
	// Password is read from ADMIN_BOOTSTRAP_PASSWORD env var at seed time.
	// If not set, admin user creation is skipped — operators must supply the var.
	bootstrapPassword := os.Getenv("ADMIN_BOOTSTRAP_PASSWORD")
	if bootstrapPassword == "" {
		log.Printf("WARN: ADMIN_BOOTSTRAP_PASSWORD not set; skipping admin user seed. Set this env var and re-run with -seed to bootstrap the admin account.")
	} else {
		hash, err := bcrypt.GenerateFromPassword([]byte(bootstrapPassword), 12)
		if err != nil {
			log.Printf("WARN: failed to hash admin password: %v", err)
		} else {
			adminUUID := uuid.New().String()
			db.Exec(`INSERT IGNORE INTO users (uuid, username, email, password_hash, is_active, created_at, updated_at)
				VALUES (?, 'admin', 'admin@propertyops.local', ?, true, NOW(), NOW())`, adminUUID, string(hash))

			// Assign SystemAdmin role to admin user.
			db.Exec(`INSERT IGNORE INTO user_roles (user_id, role_id, created_at)
				SELECT u.id, r.id, NOW() FROM users u, roles r
				WHERE u.username = 'admin' AND r.name = 'SystemAdmin'`)
		}
	}

	// Seed notification templates
	templates := []struct {
		Name     string
		Subject  string
		Body     string
		Category string
	}{
		{"work_order_created", "Work Order Created", "A new work order #{{.WorkOrderID}} has been created for {{.PropertyName}}.", "WorkOrder"},
		{"work_order_assigned", "Work Order Assigned", "Work order #{{.WorkOrderID}} has been assigned to {{.TechnicianName}}.", "WorkOrder"},
		{"work_order_reassigned", "Work Order Reassigned", "Work order #{{.WorkOrderID}} has been reassigned. Reason: {{.Reason}}", "WorkOrder"},
		{"sla_breached", "SLA Breach Alert", "Work order #{{.WorkOrderID}} has breached its SLA deadline.", "WorkOrder"},
		{"approval_requested", "Approval Requested", "Work order #{{.WorkOrderID}} is awaiting your approval.", "WorkOrder"},
		{"work_order_completed", "Work Order Completed", "Work order #{{.WorkOrderID}} has been completed.", "WorkOrder"},
		{"report_filed", "Report Filed", "A new {{.Category}} report has been filed against {{.TargetType}} #{{.TargetID}}.", "Governance"},
		{"enforcement_applied", "Enforcement Action", "An enforcement action ({{.ActionType}}) has been applied. Reason: {{.Reason}}", "Governance"},
		{"payment_marked_paid", "Payment Confirmed", "Payment #{{.PaymentID}} of ${{.Amount}} has been marked as paid.", "Payment"},
		{"reconciliation_generated", "Reconciliation Report", "Daily reconciliation report for {{.Date}} has been generated.", "Payment"},
		{"enforcement_revoked", "Enforcement Revoked", "The enforcement action ({{.ActionType}}) against you has been revoked.", "Governance"},
	}
	for _, t := range templates {
		db.Exec(`INSERT IGNORE INTO notification_templates (name, subject_template, body_template, category, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, true, NOW(), NOW())`, t.Name, t.Subject, t.Body, t.Category)
	}

	// Seed default system settings
	settings := []struct {
		Key   string
		Value string
	}{
		{"sla.emergency_hours", "4"},
		{"sla.high_hours", "24"},
		{"sla.normal_hours", "72"},
		{"sla.low_business_days", "5"},
		{"negative_rating_threshold", "2"},
	}
	for _, s := range settings {
		db.Exec(`INSERT IGNORE INTO system_settings (setting_key, setting_value, created_at, updated_at)
			VALUES (?, ?, NOW(), NOW())`, s.Key, s.Value)
	}

	return nil
}

func extractVersion(filename string) string {
	base := filepath.Base(filename)
	parts := strings.SplitN(base, "_", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return base
}

func splitSQL(content string) []string {
	var statements []string
	var current strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		statements = append(statements, s)
	}
	return statements
}
