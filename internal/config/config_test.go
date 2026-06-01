package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Set some env vars
	os.Setenv("KOSYNC_PORT", "9090")
	os.Setenv("KOSYNC_DATABASE_PATH", "/tmp/test.db")
	os.Setenv("KOSYNC_LOG_LEVEL", "debug")
	os.Setenv("KOSYNC_JSON_LOG", "true")
	os.Setenv("KOSYNC_DISABLE_REGISTRATION", "true")
	os.Setenv("KOSYNC_STORAGE_CAP_MB", "100")

	defer func() {
		os.Unsetenv("KOSYNC_PORT")
		os.Unsetenv("KOSYNC_DATABASE_PATH")
		os.Unsetenv("KOSYNC_LOG_LEVEL")
		os.Unsetenv("KOSYNC_JSON_LOG")
		os.Unsetenv("KOSYNC_DISABLE_REGISTRATION")
		os.Unsetenv("KOSYNC_STORAGE_CAP_MB")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("expected 9090, got %d", cfg.Port)
	}
	if cfg.DatabasePath != "/tmp/test.db" {
		t.Errorf("expected /tmp/test.db, got %s", cfg.DatabasePath)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug, got %s", cfg.LogLevel)
	}
	if cfg.JSONLog != true {
		t.Errorf("expected true, got %v", cfg.JSONLog)
	}
	if cfg.DisableRegistration != true {
		t.Errorf("expected true, got %v", cfg.DisableRegistration)
	}
	if cfg.StorageCapMB != 100 {
		t.Errorf("expected 100, got %d", cfg.StorageCapMB)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	// Ensure env is clean
	os.Unsetenv("KOSYNC_PORT")
	os.Unsetenv("KOSYNC_DATABASE_PATH")
	os.Unsetenv("KOSYNC_DB_PATH")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Port != 8081 {
		t.Errorf("expected 8081 default, got %d", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected info default, got %s", cfg.LogLevel)
	}
}

func TestLoadConfigDisableRegistrationDefaultTrue(t *testing.T) {
	// Ensure env is clean so the default is used
	os.Unsetenv("KOSYNC_DISABLE_REGISTRATION")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.DisableRegistration {
		t.Error("expected DisableRegistration to default to true")
	}
}

func TestLoadConfigDisableRegistrationOptIn(t *testing.T) {
	// Verify that setting the env var to false enables registration
	os.Setenv("KOSYNC_DISABLE_REGISTRATION", "false")
	defer os.Unsetenv("KOSYNC_DISABLE_REGISTRATION")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DisableRegistration {
		t.Error("expected DisableRegistration to be false when env var is false")
	}
}

func TestLoadConfigLegacyDBPathEnv(t *testing.T) {
	os.Setenv("KOSYNC_DB_PATH", "/tmp/legacy.db")
	defer os.Unsetenv("KOSYNC_DB_PATH")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DatabasePath != "/tmp/legacy.db" {
		t.Errorf("expected /tmp/legacy.db, got %s", cfg.DatabasePath)
	}
}

func TestValidateValid(t *testing.T) {
	cfg := &Config{
		Port:         8081,
		DatabasePath: "/tmp/test.db",
		LogLevel:     "info",
		StorageCapMB: 0,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidatePortZeroAllowed(t *testing.T) {
	cfg := &Config{
		Port:         0,
		DatabasePath: "/tmp/test.db",
		LogLevel:     "debug",
		StorageCapMB: 0,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error for port 0, got %v", err)
	}
}

func TestValidatePortOutOfRange(t *testing.T) {
	cfg := &Config{
		Port:         99999,
		DatabasePath: "/tmp/test.db",
		LogLevel:     "info",
		StorageCapMB: 0,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for port 99999, got nil")
	}
}

func TestValidateNegativePort(t *testing.T) {
	cfg := &Config{
		Port:         -1,
		DatabasePath: "/tmp/test.db",
		LogLevel:     "info",
		StorageCapMB: 0,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative port, got nil")
	}
}

func TestValidateEmptyDatabasePath(t *testing.T) {
	cfg := &Config{
		Port:         8081,
		DatabasePath: "",
		LogLevel:     "info",
		StorageCapMB: 0,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty database_path, got nil")
	}
}

func TestValidateInvalidLogLevel(t *testing.T) {
	cfg := &Config{
		Port:         8081,
		DatabasePath: "/tmp/test.db",
		LogLevel:     "verbose",
		StorageCapMB: 0,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid log_level, got nil")
	}
}

func TestValidateAllLogLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		cfg := &Config{
			Port:         8081,
			DatabasePath: "/tmp/test.db",
			LogLevel:     level,
			StorageCapMB: 0,
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("expected no error for log_level %q, got %v", level, err)
		}
	}
}

func TestValidateNegativeStorageCap(t *testing.T) {
	cfg := &Config{
		Port:         8081,
		DatabasePath: "/tmp/test.db",
		LogLevel:     "info",
		StorageCapMB: -1,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative storage_cap_mb, got nil")
	}
}

func TestValidateStorageCapZeroAllowed(t *testing.T) {
	cfg := &Config{
		Port:         8081,
		DatabasePath: "/tmp/test.db",
		LogLevel:     "info",
		StorageCapMB: 0,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error for storage_cap_mb=0, got %v", err)
	}
}
