package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Set some env vars
	os.Setenv("KOSYNC_PORT", "9090")
	os.Setenv("KOSYNC_DB_PATH", "/tmp/test.db")
	os.Setenv("KOSYNC_LOG_LEVEL", "debug")
	os.Setenv("KOSYNC_DISABLE_REGISTRATION", "true")
	os.Setenv("KOSYNC_STORAGE_CAP_MB", "100")

	defer func() {
		os.Unsetenv("KOSYNC_PORT")
		os.Unsetenv("KOSYNC_DB_PATH")
		os.Unsetenv("KOSYNC_LOG_LEVEL")
		os.Unsetenv("KOSYNC_DISABLE_REGISTRATION")
		os.Unsetenv("KOSYNC_STORAGE_CAP_MB")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("expected 9090, got %s", cfg.Port)
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("expected /tmp/test.db, got %s", cfg.DBPath)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug, got %s", cfg.LogLevel)
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
	os.Unsetenv("KOSYNC_DB_PATH")
	
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Port != "8081" {
		t.Errorf("expected 8081 default, got %s", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected info default, got %s", cfg.LogLevel)
	}
}
