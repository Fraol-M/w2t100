package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"propertyops/backend/internal/config"
	"propertyops/backend/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Run is the main application entry point. It initializes all subsystems, registers all
// routes, starts background schedulers, and runs the HTTP server with graceful shutdown.
func Run(cfg *config.Config) error {
	// --- Validate storage paths ---
	for _, dir := range []string{cfg.Storage.Root, cfg.Storage.BackupRoot, cfg.Storage.LogRoot} {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// --- Validate encryption key directory ---
	if _, err := os.Stat(cfg.Encryption.KeyDir); os.IsNotExist(err) {
		return fmt.Errorf("encryption key directory does not exist: %s", cfg.Encryption.KeyDir)
	}

	// --- Database connection ---
	db, err := initDB(cfg)
	if err != nil {
		return fmt.Errorf("database init failed: %w", err)
	}

	// --- Gin engine ---
	gin.SetMode(cfg.Server.GinMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// Explicitly configure trusted proxies to prevent X-Forwarded-For spoofing.
	// Set to the load-balancer/reverse-proxy IP(s) in your environment, or nil
	// to trust only the direct connection (safe default for single-node deployments).
	if err := engine.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return fmt.Errorf("failed to configure trusted proxies: %w", err)
	}

	// --- Register all modules ---
	RegisterRoutes(engine, db, cfg)

	// --- Start background schedulers ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartSchedulers(ctx, db, cfg)

	// --- Key rotation scheduler ---
	// security.Service is created independently here so the scheduler can own it
	// without creating import cycles through RegisterRoutes.
	keySvc := security.NewService(cfg.Encryption)
	StartKeyRotationScheduler(ctx, keySvc, db)

	// --- HTTP server with graceful shutdown ---
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Server listening on :%d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	cancel() // Stop background schedulers

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("Server exited cleanly")
	return nil
}

func initDB(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DB.DSN()), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Warn),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.DB.ConnMaxLifetime)

	// Verify connectivity
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return db, nil
}
