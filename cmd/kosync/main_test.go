package main

import (
	"bytes"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nlafevers/kosync/internal/api"
	"github.com/nlafevers/kosync/internal/database"
)

func openTestStorageReadOnly(t *testing.T, dbPath string) *database.Storage {
	t.Helper()
	db, err := database.OpenSQLite(dbPath, false)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return database.NewStorage(db, slog.Default())
}

func TestCLIUserManagement(t *testing.T) {
	// 1. Setup: Build binary
	exe := "./kosync_test_bin"
	cmd := exec.Command("go", "build", "-o", exe, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}
	defer os.Remove(exe)

	dbPath := filepath.Join(t.TempDir(), "cli_test.db")

	// Set env for the binary
	os.Setenv("KOSYNC_DATABASE_PATH", dbPath)
	defer os.Unsetenv("KOSYNC_DATABASE_PATH")

	// 2. Test create-user (non-interactive)
	t.Run("Create User", func(t *testing.T) {
		cmd := exec.Command(exe, "create-user", "clitest", "--password-stdin")
		cmd.Stdin = bytes.NewBufferString("clipass\n")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("create-user failed: %v, output: %s", err, output)
		}

		if bytes.Contains(output, []byte("Using database:")) || bytes.Contains(output, []byte("Using log:")) {
			t.Errorf("unexpected config path output: %s", output)
		}
		if !bytes.Contains(output, []byte("User 'clitest' created successfully.")) {
			t.Errorf("unexpected output: %s", output)
		}

		// Verify in DB
		s := openTestStorageReadOnly(t, dbPath)
		hash, err := s.GetUserHash("clitest")
		if err != nil {
			t.Errorf("user not found in db: %v", err)
		}
		// The CLI stores bcrypt(md5(password)) so the hash matches the md5 key
		// the KOReader client sends, not the raw password.
		if !api.CheckPassword(hash, api.KOReaderKey("clipass")) {
			t.Error("password mismatch")
		}
		if api.CheckPassword(hash, "clipass") {
			t.Error("hash unexpectedly matched the raw password instead of the md5 key")
		}
	})

	t.Run("Change Password", func(t *testing.T) {
		cmd := exec.Command(exe, "change-password", "clitest", "--password-stdin")
		cmd.Stdin = bytes.NewBufferString("newclipass\n")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("change-password failed: %v, output: %s", err, output)
		}

		if !bytes.Contains(output, []byte("Password for user 'clitest' updated successfully.")) {
			t.Errorf("unexpected output: %s", output)
		}

		// Verify in DB
		s := openTestStorageReadOnly(t, dbPath)
		hash, _ := s.GetUserHash("clitest")
		if !api.CheckPassword(hash, api.KOReaderKey("newclipass")) {
			t.Error("password update failed")
		}
	})

	t.Run("Create Existing User Fails", func(t *testing.T) {
		cmd := exec.Command(exe, "create-user", "clitest", "--password-stdin")
		cmd.Stdin = bytes.NewBufferString("failpass\n")
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected create existing user to fail, but it succeeded")
		}

		if !bytes.Contains(output, []byte("Error: User 'clitest' already exists")) {
			t.Errorf("unexpected output: %s", output)
		}

		s := openTestStorageReadOnly(t, dbPath)
		hash, _ := s.GetUserHash("clitest")
		if api.CheckPassword(hash, "failpass") {
			t.Error("password was updated for existing user")
		}
	})
	t.Run("Delete User", func(t *testing.T) {
		cmd := exec.Command(exe, "delete-user", "clitest")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("delete-user failed: %v, output: %s", err, output)
		}

		if !bytes.Contains(output, []byte("User 'clitest' deleted successfully.")) {
			t.Errorf("unexpected output: %s", output)
		}

		// Verify in DB
		s := openTestStorageReadOnly(t, dbPath)
		_, err = s.GetUserHash("clitest")
		if err == nil {
			t.Error("user still exists after deletion")
		}
	})

	t.Run("Delete Non-Existent User", func(t *testing.T) {
		cmd := exec.Command(exe, "delete-user", "noone")
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Error("expected failure for non-existent user, but it succeeded")
		}
		if !bytes.Contains(output, []byte("Failed to delete user: user not found")) {
			t.Errorf("expected 'Failed to delete user: user not found', got: %s", output)
		}
	})

	t.Run("Missing DB Is Created", func(t *testing.T) {
		os.Remove(dbPath)
		cmd := exec.Command(exe, "delete-user", "noone")
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Error("expected failure for non-existent user, but it succeeded")
		}
		if !bytes.Contains(output, []byte("Failed to delete user: user not found")) {
			t.Errorf("expected 'Failed to delete user: user not found', got: %s", output)
		}
		if _, err := os.Stat(dbPath); err != nil {
			t.Errorf("expected CLI to create database, got: %v", err)
		}
	})
}
