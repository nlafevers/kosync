package main

import (
	"os"
	"testing"
)

func TestInitDB(t *testing.T) {
	dbPath := "test_init.db"
	defer os.Remove(dbPath)

	storage, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer storage.Close()

	// Check file permissions
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("failed to stat db file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
	}

	// Check if tables exist
	_, err = storage.db.Exec("SELECT 1 FROM users LIMIT 1")
	if err != nil {
		t.Errorf("users table does not exist or is not accessible: %v", err)
	}

	_, err = storage.db.Exec("SELECT 1 FROM progress LIMIT 1")
	if err != nil {
		t.Errorf("progress table does not exist or is not accessible: %v", err)
	}
}
