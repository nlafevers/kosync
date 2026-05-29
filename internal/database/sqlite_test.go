package database

import (
	"bytes"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kosync/internal/models"
)

func openTestStorage(t *testing.T, dbPath string) (*Storage, *sql.DB) {
	t.Helper()
	db, err := OpenSQLite(dbPath, true)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	if err := Migrate(db); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}
	return NewStorage(db, slog.Default()), db
}

func TestStorage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	storage, db := openTestStorage(t, dbPath)
	defer db.Close()

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
func TestStorageLogsDatabaseOperations(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "logging.db")
	storage, db := openTestStorage(t, dbPath)
	defer db.Close()

	if err := storage.CreateUser("alice", "hash123"); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if err := storage.CreateUserIfNotExists("alice", "hash123"); err == nil {
		t.Fatal("expected duplicate user error")
	}

	if _, err := storage.GetProgress("alice", "missing-doc"); err != nil {
		t.Fatalf("expected missing progress to return nil error, got %v", err)
	}

	db.Close()
	if _, err := storage.GetProgress("alice", "missing-doc"); err == nil {
		t.Fatal("expected closed database lookup to error")
	}

	output := buf.String()
	if !strings.Contains(output, "getting progress") {
		t.Fatalf("expected getting progress log, got %s", output)
	}
	if !strings.Contains(output, "progress not found") {
		t.Fatalf("expected progress not found log, got %s", output)
	}
	if !strings.Contains(output, "user already exists") {
		t.Fatalf("expected duplicate user warning, got %s", output)
	}
	if !strings.Contains(output, "failed to get progress") {
		t.Fatalf("expected failed to get progress log, got %s", output)
	}
}
func TestStorageCap(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cap_test.db")
	storage, db := openTestStorage(t, dbPath)
	defer db.Close()

	storage.CreateUser("user1", "hash")
	for i := 0; i < 100; i++ {
		storage.UpsertProgress("user1", models.Progress{
			Document:  "doc" + string(rune(i)),
			Timestamp: int64(i),
		})
	}

	// Force cap enforcement with small limit
	_, err := storage.EnforceStorageCap(dbPath, 1) // 1MB might still be larger than this tiny DB
	if err != nil {
		t.Errorf("EnforceStorageCap failed: %v", err)
	}
}

func TestStorageCapLogsMaintenance(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cap_logging.db")
	storage, db := openTestStorage(t, dbPath)
	defer db.Close()

	if err := storage.CreateUser("user1", "hash"); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if err := storage.UpsertProgress("user1", models.Progress{Document: "doc1", Timestamp: time.Now().Unix()}); err != nil {
		t.Fatalf("failed to upsert progress: %v", err)
	}

	if err := os.Truncate(dbPath, 2*1024*1024); err != nil {
		t.Fatalf("failed to enlarge database file: %v", err)
	}

	if _, err := storage.EnforceStorageCap(dbPath, 1); err != nil {
		t.Fatalf("EnforceStorageCap failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "checking storage cap") {
		t.Fatalf("expected storage cap check log, got %s", output)
	}
	if !strings.Contains(output, "storage cap exceeded") {
		t.Fatalf("expected storage cap exceeded log, got %s", output)
	}
	if !strings.Contains(output, "pruning storage cap records") {
		t.Fatalf("expected pruning log, got %s", output)
	}
	if !strings.Contains(output, "storage cap records pruned") {
		t.Fatalf("expected pruned summary log, got %s", output)
	}
	if !strings.Contains(output, "database vacuum completed") {
		t.Fatalf("expected vacuum completion log, got %s", output)
	}
}
