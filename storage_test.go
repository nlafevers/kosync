package main

import (
	"database/sql"
	"os"
	"testing"
)

func TestInitDB(t *testing.T) {
	dbPath := "test_init.db"
	defer os.Remove(dbPath)

	storage, err := InitDB(dbPath, true)
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

func TestProgress(t *testing.T) {
	dbPath := "test_progress.db"
	defer os.Remove(dbPath)

	storage, err := InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer storage.Close()

	username := "testuser"
	docID := "testdoc"

	// Create user first due to foreign key
	_, err = storage.db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, "hash")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	p1 := Progress{
		Document:   docID,
		Percentage: 0.5,
		Progress:   "loc1",
		DeviceID:   "dev1",
		Device:     "kindle",
		Timestamp:  100,
	}

	// Test Insert
	if err := storage.UpsertProgress(username, p1); err != nil {
		t.Fatalf("failed to upsert progress: %v", err)
	}

	got, err := storage.GetProgress(username, docID)
	if err != nil {
		t.Fatalf("failed to get progress: %v", err)
	}
	if got == nil {
		t.Fatal("expected progress, got nil")
	}
	if got.Timestamp != 100 {
		t.Errorf("expected timestamp 100, got %d", got.Timestamp)
	}

	// Test Update with newer timestamp
	p2 := p1
	p2.Timestamp = 200
	p2.Percentage = 0.6
	if err := storage.UpsertProgress(username, p2); err != nil {
		t.Fatalf("failed to update progress: %v", err)
	}

	got, err = storage.GetProgress(username, docID)
	if err != nil {
		t.Fatalf("failed to get progress: %v", err)
	}
	if got.Timestamp != 200 {
		t.Errorf("expected timestamp 200, got %d", got.Timestamp)
	}

	// Test Update with older timestamp (should be ignored)
	p3 := p1
	p3.Timestamp = 150
	p3.Percentage = 0.7
	if err := storage.UpsertProgress(username, p3); err != nil {
		t.Fatalf("failed to update progress: %v", err)
	}

	got, err = storage.GetProgress(username, docID)
	if err != nil {
		t.Fatalf("failed to get progress: %v", err)
	}
	if got.Timestamp != 200 {
		t.Errorf("expected timestamp 200 to be preserved, got %d", got.Timestamp)
	}
	if got.Percentage != 0.6 {
		t.Errorf("expected percentage 0.6 to be preserved, got %f", got.Percentage)
	}
}
func TestUserCRUD(t *testing.T) {
	dbPath := "test_users.db"
	defer os.Remove(dbPath)

	storage, err := InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer storage.Close()

	username := "testuser"
	password := "testpass"
	hash, _ := HashPassword(password)

	// Create
	if err := storage.CreateUser(username, hash); err != nil {
		t.Errorf("failed to create user: %v", err)
	}

	// Read
	gotHash, err := storage.GetUserHash(username)
	if err != nil {
		t.Errorf("failed to get user hash: %v", err)
	}
	if gotHash != hash {
		t.Errorf("expected hash %q, got %q", hash, gotHash)
	}

	// Update
	newPassword := "newpass"
	newHash, _ := HashPassword(newPassword)
	if err := storage.UpdateUserPassword(username, newHash); err != nil {
		t.Errorf("failed to update user password: %v", err)
	}

	gotHash, _ = storage.GetUserHash(username)
	if gotHash != newHash {
		t.Errorf("expected new hash %q, got %q", newHash, gotHash)
	}

	// Delete
	if err := storage.DeleteUser(username); err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}

	_, err = storage.GetUserHash(username)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after delete, got %v", err)
	}

	// Test deleting non-existent user
	err = storage.DeleteUser("nonexistent")
	if err == nil || err.Error() != "user not found" {
		t.Errorf("expected 'user not found' error for non-existent user, got %v", err)
	}
}

func TestStorageCap(t *testing.T) {
	dbPath := "test_cap.db"
	defer os.Remove(dbPath)

	storage, err := InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer storage.Close()

	// Just verify it doesn't error on a small file with a 1MB cap
	triggered, err := storage.EnforceStorageCap(dbPath, 1)
	if err != nil {
		t.Errorf("EnforceStorageCap failed: %v", err)
	}
	if triggered {
		t.Errorf("EnforceStorageCap triggered unexpectedly on a %d byte file", func() int64 {
			s, _ := os.Stat(dbPath)
			return s.Size()
		}())
	}
}



