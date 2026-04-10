package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Set minimum required env vars
	os.Setenv("DB_PASSWORD", "testpass")
	defer os.Unsetenv("DB_PASSWORD")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Auth.BcryptCost != 12 {
		t.Errorf("expected default bcrypt cost 12, got %d", cfg.Auth.BcryptCost)
	}
	if cfg.Auth.SessionIdleTimeout.Minutes() != 30 {
		t.Errorf("expected 30min idle timeout, got %v", cfg.Auth.SessionIdleTimeout)
	}
	if cfg.Auth.SessionMaxLifetime.Hours() != 168 {
		t.Errorf("expected 168h max lifetime, got %v", cfg.Auth.SessionMaxLifetime)
	}
	if cfg.Payment.DualApprovalThreshold != 500.00 {
		t.Errorf("expected $500 threshold, got %v", cfg.Payment.DualApprovalThreshold)
	}
}

func TestValidationRequiresDBPassword(t *testing.T) {
	os.Unsetenv("DB_PASSWORD")
	_, err := Load()
	if err == nil {
		t.Error("expected error when DB_PASSWORD is empty")
	}
}

func TestValidationRequiresBcryptCostMinimum(t *testing.T) {
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("BCRYPT_COST", "4")
	defer os.Unsetenv("DB_PASSWORD")
	defer os.Unsetenv("BCRYPT_COST")

	_, err := Load()
	if err == nil {
		t.Error("expected error when BCRYPT_COST < 12")
	}
}

func TestDSNFormat(t *testing.T) {
	dbCfg := DBConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "user",
		Password: "pass",
		Name:     "testdb",
	}
	dsn := dbCfg.DSN()
	expected := "user:pass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=UTC"
	if dsn != expected {
		t.Errorf("DSN mismatch:\n  got:  %s\n  want: %s", dsn, expected)
	}
}

func TestEnvOverrides(t *testing.T) {
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("BCRYPT_COST", "14")
	os.Setenv("RATE_LIMIT_MAX_SUBMISSIONS_PER_HOUR", "5")
	defer func() {
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("BCRYPT_COST")
		os.Unsetenv("RATE_LIMIT_MAX_SUBMISSIONS_PER_HOUR")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Auth.BcryptCost != 14 {
		t.Errorf("expected bcrypt cost 14, got %d", cfg.Auth.BcryptCost)
	}
	if cfg.RateLimit.MaxSubmissionsPerHour != 5 {
		t.Errorf("expected rate limit 5, got %d", cfg.RateLimit.MaxSubmissionsPerHour)
	}
}

func TestAnomalyAllowedCIDRs(t *testing.T) {
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("ANOMALY_ALLOWED_CIDRS", "10.0.0.0/8,172.16.0.0/12")
	defer os.Unsetenv("DB_PASSWORD")
	defer os.Unsetenv("ANOMALY_ALLOWED_CIDRS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.Anomaly.AllowedCIDRs) != 2 {
		t.Errorf("expected 2 CIDRs, got %d", len(cfg.Anomaly.AllowedCIDRs))
	}
}
