package database

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"kosync/internal/models"
)

func TestEnforceStorageCapIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	storage, err := InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer storage.Close()

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
		err := storage.UpsertProgress("testuser", p)
		if err != nil {
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
