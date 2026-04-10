package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Server     ServerConfig
	DB         DBConfig
	Auth       AuthConfig
	Encryption EncryptionConfig
	Storage    StorageConfig
	Backup     BackupConfig
	Retention  RetentionConfig
	RateLimit  RateLimitConfig
	Notification NotificationConfig
	Payment    PaymentConfig
	Analytics  AnalyticsConfig
	Anomaly    AnomalyConfig
	Timezone   string
}

type ServerConfig struct {
	Port           int
	GinMode        string
	TrustedProxies []string // IP/CIDR list of trusted reverse proxies; empty = trust no proxy
}

type DBConfig struct {
	Host               string
	Port               int
	User               string
	Password           string
	Name               string
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetime    time.Duration
}

// DSN returns the MySQL Data Source Name.
func (c DBConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		c.User, c.Password, c.Host, c.Port, c.Name)
}

type AuthConfig struct {
	BcryptCost            int
	SessionIdleTimeout    time.Duration
	SessionMaxLifetime    time.Duration
}

type EncryptionConfig struct {
	KeyDir         string
	ActiveKeyID    int
	RotationDays   int
}

type StorageConfig struct {
	Root       string
	BackupRoot string
	LogRoot    string
}

type BackupConfig struct {
	ScheduleCron      string
	RetentionDays     int
	EncryptionEnabled bool
}

type RetentionConfig struct {
	FinancialYears int
	MessageYears   int
}

type RateLimitConfig struct {
	MaxSubmissionsPerHour int
}

type NotificationConfig struct {
	PollIntervalSeconds int
	MaxRetries          int
	RetryDelaySeconds   int
}

type PaymentConfig struct {
	IntentExpiryMinutes    int
	DualApprovalThreshold  float64
}

type AnalyticsConfig struct {
	ReportPollIntervalSeconds int
}

type AnomalyConfig struct {
	AllowedCIDRs []string
}

// Load reads all configuration from environment variables and returns a validated Config.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:           envInt("SERVER_PORT", 8080),
			GinMode:        envStr("GIN_MODE", "release"),
			TrustedProxies: envStrSlice("TRUSTED_PROXIES", []string{}),
		},
		DB: DBConfig{
			Host:            envStr("DB_HOST", "localhost"),
			Port:            envInt("DB_PORT", 3306),
			User:            envStr("DB_USER", "propertyops"),
			Password:        envStr("DB_PASSWORD", ""),
			Name:            envStr("DB_NAME", "propertyops"),
			MaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: time.Duration(envInt("DB_CONN_MAX_LIFETIME_MINUTES", 30)) * time.Minute,
		},
		Auth: AuthConfig{
			BcryptCost:         envInt("BCRYPT_COST", 12),
			SessionIdleTimeout: time.Duration(envInt("SESSION_IDLE_TIMEOUT_MINUTES", 30)) * time.Minute,
			SessionMaxLifetime: time.Duration(envInt("SESSION_MAX_LIFETIME_HOURS", 168)) * time.Hour,
		},
		Encryption: EncryptionConfig{
			KeyDir:       envStr("ENCRYPTION_KEY_DIR", "/run/propertyops/keys"),
			ActiveKeyID:  envInt("ENCRYPTION_ACTIVE_KEY_ID", 1),
			RotationDays: envInt("ENCRYPTION_ROTATION_DAYS", 180),
		},
		Storage: StorageConfig{
			Root:       envStr("STORAGE_ROOT", "/var/lib/propertyops/storage"),
			BackupRoot: envStr("BACKUP_ROOT", "/var/lib/propertyops/backups"),
			LogRoot:    envStr("LOG_ROOT", "/var/log/propertyops"),
		},
		Backup: BackupConfig{
			ScheduleCron:      envStr("BACKUP_SCHEDULE_CRON", "0 2 * * *"),
			RetentionDays:     envInt("BACKUP_RETENTION_DAYS", 30),
			EncryptionEnabled: envBool("BACKUP_ENCRYPTION_ENABLED", true),
		},
		Retention: RetentionConfig{
			FinancialYears: envInt("FINANCIAL_RETENTION_YEARS", 7),
			MessageYears:   envInt("MESSAGE_RETENTION_YEARS", 2),
		},
		RateLimit: RateLimitConfig{
			MaxSubmissionsPerHour: envInt("RATE_LIMIT_MAX_SUBMISSIONS_PER_HOUR", 10),
		},
		Notification: NotificationConfig{
			PollIntervalSeconds: envInt("NOTIFICATION_POLL_INTERVAL_SECONDS", 60),
			MaxRetries:          envInt("NOTIFICATION_MAX_RETRIES", 3),
			RetryDelaySeconds:   envInt("NOTIFICATION_RETRY_DELAY_SECONDS", 300),
		},
		Payment: PaymentConfig{
			IntentExpiryMinutes:   envInt("PAYMENT_INTENT_EXPIRY_MINUTES", 30),
			DualApprovalThreshold: envFloat("PAYMENT_DUAL_APPROVAL_THRESHOLD", 500.00),
		},
		Analytics: AnalyticsConfig{
			ReportPollIntervalSeconds: envInt("REPORT_SCHEDULE_POLL_INTERVAL_SECONDS", 300),
		},
		Anomaly: AnomalyConfig{
			AllowedCIDRs: envStrSlice("ANOMALY_ALLOWED_CIDRS", []string{"127.0.0.1/32", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}),
		},
		Timezone: envStr("DEFAULT_TIMEZONE", "America/New_York"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	return cfg, nil
}

// Validate checks configuration invariants and returns an error if any are violated.
func (c *Config) Validate() error {
	if c.DB.Password == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	if c.Auth.BcryptCost < 12 {
		return fmt.Errorf("BCRYPT_COST must be >= 12, got %d", c.Auth.BcryptCost)
	}
	if c.Encryption.KeyDir == "" {
		return fmt.Errorf("ENCRYPTION_KEY_DIR is required")
	}
	if c.Encryption.ActiveKeyID < 1 {
		return fmt.Errorf("ENCRYPTION_ACTIVE_KEY_ID must be >= 1")
	}
	if c.Storage.Root == "" {
		return fmt.Errorf("STORAGE_ROOT is required")
	}
	if c.Payment.DualApprovalThreshold <= 0 {
		return fmt.Errorf("PAYMENT_DUAL_APPROVAL_THRESHOLD must be > 0")
	}
	return nil
}

// --- Env helpers ---

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func envStrSlice(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				result = append(result, t)
			}
		}
		return result
	}
	return fallback
}
