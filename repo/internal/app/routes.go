package app

import (
	"mime/multipart"

	"propertyops/backend/internal/admin"
	"propertyops/backend/internal/analytics"
	"propertyops/backend/internal/attachments"
	"propertyops/backend/internal/audit"
	"propertyops/backend/internal/auth"
	"propertyops/backend/internal/backups"
	"propertyops/backend/internal/common"
	"propertyops/backend/internal/config"
	"propertyops/backend/internal/governance"
	"propertyops/backend/internal/health"
	apphttp "propertyops/backend/internal/http"
	"propertyops/backend/internal/logs"
	"propertyops/backend/internal/notifications"
	"propertyops/backend/internal/payments"
	"propertyops/backend/internal/properties"
	"propertyops/backend/internal/security"
	"propertyops/backend/internal/tenants"
	"propertyops/backend/internal/users"
	"propertyops/backend/internal/workorders"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// attachmentUploaderAdapter wraps *attachments.Service to satisfy the workorders.AttachmentUploader
// interface, which expects a single UploadFile method returning only *common.AppError.
type attachmentUploaderAdapter struct {
	svc *attachments.Service
}

func (a *attachmentUploaderAdapter) UploadFile(workOrderID uint64, file *multipart.FileHeader, uploaderID uint64, uploaderRoles []string, ip, requestID string) *common.AppError {
	_, err := a.svc.Upload(workOrderID, file, uploaderID, uploaderRoles, ip, requestID)
	return err
}

func (a *attachmentUploaderAdapter) DeleteWorkOrderAttachments(workOrderID uint64) {
	a.svc.DeleteForWorkOrder(workOrderID)
}

// propQuerierAdapter adapts the properties.Repository to the workorders.PropertyQuerier interface.
// The properties repository stores full DispatchCursor objects; the workorders dispatch
// service only needs cursor positions and user-ID lists.
type propQuerierAdapter struct {
	repo properties.Repository
}

func (a *propQuerierAdapter) FindTechniciansByPropertyAndSkill(propertyID uint64, skillTag string) ([]uint64, error) {
	assignments, err := a.repo.FindTechniciansByPropertyAndSkill(propertyID, skillTag)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, len(assignments))
	for i, asgn := range assignments {
		ids[i] = asgn.UserID
	}
	return ids, nil
}

func (a *propQuerierAdapter) GetDispatchCursor(propertyID uint64, skillTag string) (int, error) {
	cursor, err := a.repo.GetDispatchCursor(propertyID, skillTag)
	if err != nil {
		// No cursor yet — start at position 0.
		return 0, nil
	}
	return cursor.CursorPosition, nil
}

func (a *propQuerierAdapter) UpdateDispatchCursor(propertyID uint64, skillTag string, position int, userID uint64) error {
	cursor := &properties.DispatchCursor{
		PropertyID:         propertyID,
		SkillTag:           skillTag,
		CursorPosition:     position,
		LastAssignedUserID: &userID,
	}
	return a.repo.UpsertDispatchCursor(cursor)
}

// RegisterRoutes wires up all application routes and middleware.
// This is the single centralized location where all route groups are registered.
func RegisterRoutes(engine *gin.Engine, db *gorm.DB, cfg *config.Config) {
	// --- Shared services ---
	auditService := audit.NewService(audit.NewRepository(db))
	securityService := security.NewService(cfg.Encryption)
	logService := logs.NewService(cfg.Storage.LogRoot)

	// --- Middleware ---
	authRepo := auth.NewRepository(db)
	authService := auth.NewService(authRepo, cfg.Auth, auditService)
	mw := apphttp.NewMiddleware(authService, auditService, db, cfg)

	// Apply global middleware
	engine.Use(mw.RequestID())
	engine.Use(mw.StructuredLogging(logService))
	engine.Use(mw.PanicRecovery())

	// --- Health endpoints (no auth required) ---
	health.RegisterRoutes(engine, db, cfg)

	// --- API v1 ---
	v1 := engine.Group("/api/v1")

	// Auth routes (public: login only)
	auth.RegisterRoutes(v1.Group("/auth"), authService)

	// Anomaly ingestion — local-network-only, NO bearer token required.
	// Must be registered BEFORE the authenticated protected group.
	admin.RegisterAnomalyRoute(v1.Group("/admin"), db, cfg, auditService, mw)

	// Protected routes
	protected := v1.Group("")
	protected.Use(mw.Authenticate())
	protected.Use(mw.CheckSuspension())

	// Auth protected routes (logout, me) — require authenticated session.
	auth.RegisterProtectedRoutes(protected.Group("/auth"), authService)

	// Users (admin only for management, self-service for profile)
	userRepo := users.NewRepository(db)
	userService := users.NewService(userRepo, auditService, securityService, cfg.Auth)
	users.RegisterRoutes(protected, userService, mw)

	// Tenants
	tenantRepo := tenants.NewRepository(db)
	tenantService := tenants.NewService(tenantRepo, auditService, securityService)
	tenants.RegisterRoutes(protected.Group("/tenants"), tenantService, mw)

	// Properties
	propRepo := properties.NewRepository(db)
	propService := properties.NewService(propRepo, auditService)
	properties.RegisterRoutes(protected.Group("/properties"), propService, mw)

	// Work Orders
	woRepo := workorders.NewRepository(db)
	notifRepo := notifications.NewRepository(db)
	notifService := notifications.NewService(notifRepo, auditService).WithWorkOrderChecker(woRepo)
	woService := workorders.NewService(woRepo, &propQuerierAdapter{repo: propRepo}, notifService, auditService, db, cfg)

	// Attachments (registered before work orders so the uploader adapter can be constructed)
	attachRepo := attachments.NewRepository(db)
	attachService := attachments.NewService(attachRepo, woRepo, auditService, cfg.Storage)
	attachments.RegisterRoutes(protected.Group("/work-orders"), attachService, mw)

	woUploader := &attachmentUploaderAdapter{svc: attachService}
	workorders.RegisterRoutes(protected.Group("/work-orders"), woService, mw, woUploader)

	// Notifications
	notifications.RegisterRoutes(protected.Group("/notifications"), notifService, mw)

	// Governance
	govRepo := governance.NewRepository(db)
	govService := governance.NewService(govRepo, auditService, notifService, db)
	governance.RegisterRoutes(protected.Group("/governance"), govService, attachService, mw)

	// Payments
	payRepo := payments.NewRepository(db)
	payService := payments.NewService(payRepo, auditService, notifService, db, cfg)
	payments.RegisterRoutes(protected.Group("/payments"), payService, mw)

	// Analytics
	analyticsRepo := analytics.NewRepository(db)
	analyticsService := analytics.NewService(analyticsRepo, cfg.Storage)
	analytics.RegisterRoutes(protected.Group("/analytics"), analyticsService, mw, woRepo)

	// Admin routes (SystemAdmin only)
	adminGroup := protected.Group("/admin")
	audit.RegisterRoutes(adminGroup.Group("/audit-logs"), auditService, mw)
	logs.RegisterRoutes(adminGroup.Group("/logs"), logService, mw)
	backupService := backups.NewService(db, cfg, securityService, auditService)
	backups.RegisterRoutes(adminGroup.Group("/backups"), backupService, mw)
	admin.RegisterRoutes(adminGroup, db, cfg, securityService, auditService, userService, mw)
}
