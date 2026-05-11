package main

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Set test environment variables
	os.Setenv("KOSYNC_PORT", "9999")
	os.Setenv("KOSYNC_DB_PATH", "test.db")
	os.Setenv("KOSYNC_LOG_LEVEL", "debug")
	os.Setenv("KOSYNC_DISABLE_REGISTRATION", "true")
	os.Setenv("KOSYNC_STORAGE_CAP_MB", "50")

	// Ensure they are cleaned up
	defer func() {
		os.Unsetenv("KOSYNC_PORT")
		os.Unsetenv("KOSYNC_DB_PATH")
		os.Unsetenv("KOSYNC_LOG_LEVEL")
		os.Unsetenv("KOSYNC_DISABLE_REGISTRATION")
		os.Unsetenv("KOSYNC_STORAGE_CAP_MB")
	}()

	cfg := LoadConfig()

	if cfg.Port != "9999" {
		t.Errorf("expected port 9999, got %s", cfg.Port)
	}
	// DBPath is now resolved to an absolute path if relative
	if !strings.HasSuffix(cfg.DBPath, "test.db") {
		t.Errorf("expected db_path to end with test.db, got %s", cfg.DBPath)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level debug, got %s", cfg.LogLevel)
	}
	if cfg.DisableRegistration != true {
		t.Errorf("expected DisableRegistration true, got %v", cfg.DisableRegistration)
	}
	if cfg.StorageCapMB != 50 {
		t.Errorf("expected StorageCapMB 50, got %d", cfg.StorageCapMB)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	// Ensure env is clear
	os.Unsetenv("KOSYNC_PORT")
	os.Unsetenv("KOSYNC_DB_PATH")
	os.Unsetenv("KOSYNC_LOG_LEVEL")
	os.Unsetenv("KOSYNC_DISABLE_REGISTRATION")
	os.Unsetenv("KOSYNC_STORAGE_CAP_MB")

	cfg := LoadConfig()

	if cfg.Port != "8081" {
		t.Errorf("expected default port 8081, got %s", cfg.Port)
	}
	if !strings.HasSuffix(cfg.DBPath, "kosync.db") {
		t.Errorf("expected default db_path to end with kosync.db, got %s", cfg.DBPath)
	}
}
