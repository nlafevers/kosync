package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
	os.Setenv("KOSYNC_DB_PATH", dbPath)
	defer os.Unsetenv("KOSYNC_DB_PATH")

	// 2. Test create-user (non-interactive)
	t.Run("Create User", func(t *testing.T) {
		// First, we need the DB to exist since CLI doesn't create it.
		// We'll use the binary to start the server briefly or just touch it?
		// Actually, the server creates it. Let's create it manually for the test.
		s, err := InitDB(dbPath, true)
		if err != nil {
			t.Fatalf("failed to create db: %v", err)
		}
		s.Close()

		cmd := exec.Command(exe, "create-user", "clitest", "--password-stdin")
		cmd.Stdin = bytes.NewBufferString("clipass\n")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("create-user failed: %v, output: %s", err, output)
		}

		if !bytes.Contains(output, []byte("User 'clitest' created successfully")) {
			t.Errorf("unexpected output: %s", output)
		}

		// Verify in DB
		s, _ = InitDB(dbPath, false)
		defer s.Close()
		hash, err := s.GetUserHash("clitest")
		if err != nil {
			t.Errorf("user not found in db: %v", err)
		}
		if !CheckPassword(hash, "clipass") {
			t.Error("password mismatch")
		}
	})

	t.Run("Change Password", func(t *testing.T) {
		cmd := exec.Command(exe, "change-password", "clitest", "--password-stdin")
		cmd.Stdin = bytes.NewBufferString("newclipass\n")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("change-password failed: %v, output: %s", err, output)
		}

		if !bytes.Contains(output, []byte("Password for user 'clitest' updated successfully")) {
			t.Errorf("unexpected output: %s", output)
		}

		// Verify in DB
		s, _ := InitDB(dbPath, false)
		defer s.Close()
		hash, _ := s.GetUserHash("clitest")
		if !CheckPassword(hash, "newclipass") {
			t.Error("password update failed")
		}
	})

	t.Run("Delete User", func(t *testing.T) {
		cmd := exec.Command(exe, "delete-user", "clitest")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("delete-user failed: %v, output: %s", err, output)
		}

		if !bytes.Contains(output, []byte("User 'clitest' deleted successfully")) {
			t.Errorf("unexpected output: %s", output)
		}

		// Verify in DB
		s, _ := InitDB(dbPath, false)
		defer s.Close()
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
		if !bytes.Contains(output, []byte("Error: user not found")) {
			t.Errorf("expected 'Error: user not found', got: %s", output)
		}
	})

	t.Run("DB Guard", func(t *testing.T) {
		os.Remove(dbPath)
		cmd := exec.Command(exe, "delete-user", "noone")
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Error("expected failure for non-existent db, but it succeeded")
		}
		if !bytes.Contains(output, []byte("database file does not exist")) {
			t.Errorf("expected 'database file does not exist' error, got: %s", output)
		}
	})
}
