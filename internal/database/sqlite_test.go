package database

import (
	"path/filepath"
	"testing"
	"time"

	"kosync/internal/models"
)

func TestStorage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	storage, err := InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer storage.Close()

	t.Run("Create and Get User", func(t *testing.T) {
		err := storage.CreateUser("testuser", "hash123")
		if err != nil {
			t.Errorf("failed to create user: %v", err)
		}

		hash, err := storage.GetUserHash("testuser")
		if err != nil || hash != "hash123" {
			t.Errorf("failed to get correct hash: %v, got %s", err, hash)
		}
	})

	t.Run("Upsert and Get Progress", func(t *testing.T) {
		p := models.Progress{
			Document:   "doc123",
			Percentage: 0.75,
			Progress:   "loc1",
			DeviceID:   "dev1",
			Device:     "ereader",
			Timestamp:  time.Now().Unix(),
		}

		err := storage.UpsertProgress("testuser", p)
		if err != nil {
			t.Errorf("failed to upsert progress: %v", err)
		}

		saved, err := storage.GetProgress("testuser", "doc123")
		if err != nil || saved == nil || saved.Percentage != 0.75 {
			t.Errorf("failed to get correct progress: %v, got %+v", err, saved)
		}

		// Update with newer timestamp
		p2 := p
		p2.Percentage = 0.80
		p2.Timestamp += 10
		storage.UpsertProgress("testuser", p2)
		saved, _ = storage.GetProgress("testuser", "doc123")
		if saved.Percentage != 0.80 {
			t.Errorf("expected 0.80, got %f", saved.Percentage)
		}

		// Try update with older timestamp (should be ignored)
		p3 := p
		p3.Percentage = 0.60
		p3.Timestamp -= 20
		storage.UpsertProgress("testuser", p3)
		saved, _ = storage.GetProgress("testuser", "doc123")
		if saved.Percentage != 0.80 {
			t.Errorf("expected 0.80 (ignored older update), got %f", saved.Percentage)
		}
	})

	t.Run("Delete User", func(t *testing.T) {
		err := storage.DeleteUser("testuser")
		if err != nil {
			t.Errorf("failed to delete user: %v", err)
		}

		_, err = storage.GetUserHash("testuser")
		if err == nil {
			t.Error("user still exists after deletion")
		}
	})
}

func TestStorageCap(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cap_test.db")
	storage, _ := InitDB(dbPath, true)
	defer storage.Close()

	storage.CreateUser("user1", "hash")
	for i := 0; i < 100; i++ {
		storage.UpsertProgress("user1", models.Progress{
			Document:  "doc" + string(rune(i)),
			Timestamp: int64(i),
		})
	}

	// Force cap enforcement with small limit
	// info, _ := os.Stat(dbPath)
	// We'll just call it manually with a very low MB
	_, err := storage.EnforceStorageCap(dbPath, 1) // 1MB might still be larger than this tiny DB
	if err != nil {
		t.Errorf("EnforceStorageCap failed: %v", err)
	}
}
