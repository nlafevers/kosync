package database

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/nlafevers/kosync/internal/models"
)

func TestEnforceStorageCapIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := OpenSQLite(dbPath, true)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	if err := Migrate(db); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}
	storage := NewStorage(db, slog.Default())
	defer db.Close()

	// Create the user referenced by progress records
	if err := storage.CreateUserIfNotExists("testuser", "hash"); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Bloat the database with dummy progress records
	// We need enough data to exceed 1MB
	for i := 0; i < 20000; i++ {
		p := models.Progress{
			Document:   fmt.Sprintf("doc_%d", i),
			Percentage: 0.5,
			Progress:   "loc1",
			DeviceID:   "dev1",
			Device:     "ereader",
			Timestamp:  time.Now().Unix(),
		}
		if _, err := storage.UpsertProgress("testuser", p); err != nil {
			t.Fatalf("failed to upsert progress: %v", err)
		}
	}

	// Enforce 1MB cap
	pruned, err := storage.EnforceStorageCap(dbPath, 1)
	if err != nil {
		t.Fatalf("EnforceStorageCap failed: %v", err)
	}
	if !pruned {
		t.Fatal("expected pruning to occur")
	}

	// Verify pruning worked
	var count int
	err = storage.db.QueryRow("SELECT COUNT(*) FROM progress").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query count: %v", err)
	}
	if count >= 20000 {
		t.Errorf("expected fewer than 20000 records, got %d", count)
	}
	t.Logf("Final record count: %d", count)
}
