package app

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"propertyops/backend/internal/analytics"
	"propertyops/backend/internal/audit"
	"propertyops/backend/internal/backups"
	"propertyops/backend/internal/config"
	"propertyops/backend/internal/notifications"
	"propertyops/backend/internal/security"

	"gorm.io/gorm"
)

// KeyRotator abstracts security.Service so the scheduler can auto-rotate keys
// without importing the security package directly.
type KeyRotator interface {
	KeyRotationDue() bool
	RotateKey() (int, error)
}

// StartSchedulers launches all background scheduler goroutines.
// Each scheduler runs in a loop with a configurable interval and stops when ctx is cancelled.
func StartSchedulers(ctx context.Context, db *gorm.DB, cfg *config.Config) {
	// 1. Session cleanup — every 5 minutes
	go runScheduler(ctx, "session-cleanup", 5*time.Minute, func() {
		cleanExpiredSessions(db, cfg)
	})

	// 2. Notification delivery — configurable interval
	go runScheduler(ctx, "notification-delivery",
		time.Duration(cfg.Notification.PollIntervalSeconds)*time.Second, func() {
			processNotificationDelivery(db, cfg)
		})

	// 3. Payment intent expiry — every minute
	go runScheduler(ctx, "payment-expiry", 1*time.Minute, func() {
		expirePaymentIntents(db)
	})

	// 4. SLA breach check — every 5 minutes
	go runScheduler(ctx, "sla-breach-check", 5*time.Minute, func() {
		checkSLABreaches(db)
	})

	// 5. Scheduled report generation — configurable interval
	go runScheduler(ctx, "report-generation",
		time.Duration(cfg.Analytics.ReportPollIntervalSeconds)*time.Second, func() {
			processScheduledReports(db, cfg)
		})

	// 6. Encrypted backup — cron-scheduled via BACKUP_SCHEDULE_CRON
	go runCronScheduler(ctx, cfg.Backup.ScheduleCron, func() {
		runDailyBackup(db, cfg)
	})

	log.Println("All background schedulers started")
}

// StartKeyRotationScheduler starts the periodic encryption-key rotation check.
// It is kept separate from StartSchedulers so the caller can supply the security.Service
// without creating a circular import between app and security packages.
func StartKeyRotationScheduler(ctx context.Context, rotator KeyRotator, db *gorm.DB) {
	// Check once per day whether the active key is older than RotationDays.
	go runScheduler(ctx, "key-rotation", 24*time.Hour, func() {
		checkAndRotateKey(rotator, db)
	})
}

func checkAndRotateKey(rotator KeyRotator, db *gorm.DB) {
	if !rotator.KeyRotationDue() {
		return
	}
	newVersion, err := rotator.RotateKey()
	if err != nil {
		log.Printf("Key rotation: failed to rotate encryption key: %v", err)
		return
	}
	log.Printf("Key rotation: rotated to key version %d", newVersion)

	// Record the rotation event in the audit log table directly so it is visible
	// without needing the audit service as a scheduler dependency.
	db.Exec(`INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, description, created_at)
		VALUES (0, 'key_rotation', 'EncryptionKey', ?, 'Automatic 180-day key rotation', NOW())`, newVersion)
}

// parseDailyCron parses a 5-field cron expression of the form "M H * * *"
// (minute, hour, any day-of-month, any month, any weekday) and returns
// the UTC hour and minute at which the job should fire each day.
// Returns -1, -1 if the expression cannot be parsed or uses unsupported fields.
func parseDailyCron(expr string) (hour, minute int) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return -1, -1
	}
	m, err1 := strconv.Atoi(fields[0])
	h, err2 := strconv.Atoi(fields[1])
	if err1 != nil || err2 != nil {
		return -1, -1
	}
	if m < 0 || m > 59 || h < 0 || h > 23 {
		return -1, -1
	}
	// Only support the wildcard form (* * *) for the remaining fields.
	if fields[2] != "*" || fields[3] != "*" || fields[4] != "*" {
		return -1, -1
	}
	return h, m
}

// runCronScheduler fires job once per day at the time specified by cronExpr
// ("M H * * *" UTC). If the expression is invalid it falls back to a 24-hour
// fixed interval so the backup always runs even with a misconfigured cron.
func runCronScheduler(ctx context.Context, cronExpr string, job func()) {
	h, m := parseDailyCron(cronExpr)
	if h < 0 {
		log.Printf("Scheduler [daily-backup]: cannot parse cron %q, falling back to 24h interval", cronExpr)
		runScheduler(ctx, "daily-backup", 24*time.Hour, job)
		return
	}

	log.Printf("Scheduler [daily-backup] started with cron %q (daily at %02d:%02d UTC)", cronExpr, h, m)
	for {
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}

		select {
		case <-ctx.Done():
			log.Printf("Scheduler [daily-backup] stopped")
			return
		case <-time.After(time.Until(next)):
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Scheduler [daily-backup] panic recovered: %v", r)
					}
				}()
				job()
			}()
		}
	}
}

func runScheduler(ctx context.Context, name string, interval time.Duration, job func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Scheduler [%s] started with interval %v", name, interval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("Scheduler [%s] stopped", name)
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Scheduler [%s] panic recovered: %v", name, r)
					}
				}()
				job()
			}()
		}
	}
}

// --- Scheduler job implementations ---

func cleanExpiredSessions(db *gorm.DB, cfg *config.Config) {
	result := db.Exec(`UPDATE sessions SET revoked_at = NOW()
		WHERE revoked_at IS NULL AND (
			last_active_at < DATE_SUB(NOW(), INTERVAL ? MINUTE) OR
			created_at < DATE_SUB(NOW(), INTERVAL ? HOUR)
		)`, int(cfg.Auth.SessionIdleTimeout.Minutes()), int(cfg.Auth.SessionMaxLifetime.Hours()))
	if result.Error != nil {
		log.Printf("Session cleanup error: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("Session cleanup: revoked %d expired sessions", result.RowsAffected)
	}
}

func processNotificationDelivery(db *gorm.DB, cfg *config.Config) {
	repo := notifications.NewRepository(db)

	// Fetch all pending notifications that are due (scheduled_for is past or unset).
	pending, err := repo.FindPendingNotifications(200)
	if err != nil {
		log.Printf("Notification delivery: failed to fetch pending: %v", err)
		return
	}
	if len(pending) == 0 {
		return
	}

	now := time.Now().UTC()
	retryDelay := time.Duration(cfg.Notification.RetryDelaySeconds) * time.Second
	sent, failed := 0, 0

	for _, n := range pending {
		// Enforce retry delay: skip notifications that were attempted recently.
		if n.RetryCount > 0 && n.UpdatedAt.Add(retryDelay).After(now) {
			continue
		}

		// Notifications that exceeded the retry limit transition to Failed.
		if n.RetryCount >= cfg.Notification.MaxRetries {
			if err := repo.UpdateNotificationStatus(n.ID, "Failed"); err != nil {
				log.Printf("Notification delivery: failed to mark notification %d as Failed: %v", n.ID, err)
			} else {
				failed++
			}
			continue
		}

		// Attempt delivery: mark as Sent via the repository method.
		if err := repo.UpdateNotificationStatus(n.ID, "Sent"); err != nil {
			log.Printf("Notification delivery: failed to deliver notification %d, incrementing retry: %v", n.ID, err)
			_ = repo.IncrementRetry(n.ID)
		} else {
			sent++
		}
	}

	if sent+failed > 0 {
		log.Printf("Notification delivery: sent=%d failed=%d", sent, failed)
	}
}

func expirePaymentIntents(db *gorm.DB) {
	result := db.Exec(`UPDATE payments SET status = 'Expired', updated_at = NOW()
		WHERE kind = 'Intent' AND status = 'Pending'
		AND expires_at IS NOT NULL AND expires_at <= NOW()`)
	if result.Error != nil {
		log.Printf("Payment expiry error: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("Payment expiry: expired %d intents", result.RowsAffected)
	}
}

func checkSLABreaches(db *gorm.DB) {
	// Mark work orders that have breached their SLA.
	result := db.Exec(`UPDATE work_orders SET sla_breached_at = NOW(), updated_at = NOW()
		WHERE sla_due_at IS NOT NULL AND sla_breached_at IS NULL
		AND sla_due_at <= NOW()
		AND status NOT IN ('Completed', 'Archived')`)
	if result.Error != nil {
		log.Printf("SLA breach check error: %v", result.Error)
		return
	}
	if result.RowsAffected == 0 {
		return
	}

	log.Printf("SLA breach check: flagged %d work orders", result.RowsAffected)

	// Insert SLA breach events for the newly-breached work orders.
	db.Exec(`INSERT INTO work_order_events (work_order_id, event_type, description, created_at)
		SELECT id, 'sla_breached', 'SLA deadline has been breached', NOW()
		FROM work_orders
		WHERE sla_breached_at >= DATE_SUB(NOW(), INTERVAL 1 MINUTE)
		AND status NOT IN ('Completed', 'Archived')`)

	// Fan-out in-app notifications to tenants and assigned technicians.
	// Uses a direct INSERT so the scheduler does not need a service dependency.
	db.Exec(`INSERT INTO notifications (uuid, recipient_id, template_id, subject, body, category, status, created_at, updated_at)
		SELECT UUID(), wo.tenant_id,
		       (SELECT id FROM notification_templates WHERE name = 'sla_breached' LIMIT 1),
		       CONCAT('SLA Breach: Work Order #', wo.id),
		       CONCAT('Work order #', wo.id, ' has breached its SLA deadline.'),
		       'WorkOrder', 'Pending', NOW(), NOW()
		FROM work_orders wo
		WHERE wo.sla_breached_at >= DATE_SUB(NOW(), INTERVAL 1 MINUTE)
		AND wo.status NOT IN ('Completed', 'Archived')`)

	db.Exec(`INSERT INTO notifications (uuid, recipient_id, template_id, subject, body, category, status, created_at, updated_at)
		SELECT UUID(), wo.assigned_to,
		       (SELECT id FROM notification_templates WHERE name = 'sla_breached' LIMIT 1),
		       CONCAT('SLA Breach: Work Order #', wo.id),
		       CONCAT('Work order #', wo.id, ' assigned to you has breached its SLA deadline.'),
		       'WorkOrder', 'Pending', NOW(), NOW()
		FROM work_orders wo
		WHERE wo.assigned_to IS NOT NULL
		AND wo.sla_breached_at >= DATE_SUB(NOW(), INTERVAL 1 MINUTE)
		AND wo.status NOT IN ('Completed', 'Archived')`)
}

// processScheduledReports queries saved reports that are due and generates them.
func processScheduledReports(db *gorm.DB, cfg *config.Config) {
	// Find saved reports with a recognised schedule value that are due.
	// Currently the only supported schedule value is 'daily'.
	var dueIDs []uint64
	db.Raw(`SELECT id FROM saved_reports
		WHERE schedule = 'daily'
		AND is_active = true
		AND (last_generated_at IS NULL OR last_generated_at < DATE_SUB(NOW(), INTERVAL 1 DAY))`).
		Scan(&dueIDs)

	if len(dueIDs) == 0 {
		return
	}

	svc := analytics.NewService(analytics.NewRepository(db), cfg.Storage)
	generated := 0
	for _, id := range dueIDs {
		if _, err := svc.GenerateReport(id, 0); err != nil {
			log.Printf("Report generation: failed for saved_report %d: %s", id, err.Message)
			continue
		}
		// Stamp last_generated_at so the report won't run again today.
		db.Exec("UPDATE saved_reports SET last_generated_at = NOW() WHERE id = ?", id)
		generated++
	}
	log.Printf("Report generation: generated %d of %d due reports", generated, len(dueIDs))
}

// runDailyBackup creates an encrypted mysqldump backup.
// It constructs local service instances because the scheduler does not hold
// long-lived service references (avoids circular dependency with routes.go).
func runDailyBackup(db *gorm.DB, cfg *config.Config) {
	secSvc := security.NewService(cfg.Encryption)
	auditSvc := audit.NewService(audit.NewRepository(db))
	backupSvc := backups.NewService(db, cfg, secSvc, auditSvc)

	record, err := backupSvc.CreateBackup(0, "127.0.0.1", "scheduler")
	if err != nil {
		log.Printf("Daily backup error: %v", err)
		return
	}
	log.Printf("Daily backup: completed successfully (%s)", record.Filename)

	// Immediately validate the newly-created backup to detect corruption early.
	result, valErr := backupSvc.ValidateBackup(record.FilePath, 0, "127.0.0.1", "scheduler")
	if valErr != nil {
		log.Printf("Daily backup validation error: %v", valErr)
		return
	}
	if len(result.Errors) > 0 {
		log.Printf("Daily backup validation FAILED for %s: %v", record.Filename, result.Errors)
		// Record in audit log so operators are alerted.
		db.Exec(`INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, description, created_at)
			VALUES (0, 'backup_validation_failed', 'Backup', 0, ?, NOW())`, record.Filename)
		return
	}
	log.Printf("Daily backup validation passed (%d checks)", len(result.Checks))
}
