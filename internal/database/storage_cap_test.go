package database

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnforceStorageCapHelper(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.db")

	t.Run("disabled cap skips file stat and callbacks", func(t *testing.T) {
		pruned, err := enforceStorageCap(missingPath, 0, func() error {
			t.Fatal("prune should not run when cap is disabled")
			return nil
		}, func() error {
			t.Fatal("vacuum should not run when cap is disabled")
			return nil
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if pruned {
			t.Fatal("expected no pruning when cap is disabled")
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		pruned, err := enforceStorageCap(missingPath, 1, func() error {
			t.Fatal("prune should not run when stat fails")
			return nil
		}, func() error {
			t.Fatal("vacuum should not run when stat fails")
			return nil
		})
		if err == nil {
			t.Fatal("expected stat error")
		}
		if pruned {
			t.Fatal("expected no pruning when stat fails")
		}
	})

	t.Run("below cap skips callbacks", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "small.db")
		if err := os.WriteFile(path, []byte("small"), 0600); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		pruned, err := enforceStorageCap(path, 1, func() error {
			t.Fatal("prune should not run below cap")
			return nil
		}, func() error {
			t.Fatal("vacuum should not run below cap")
			return nil
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if pruned {
			t.Fatal("expected no pruning below cap")
		}
	})

	t.Run("at cap runs prune before vacuum", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "large.db")
		if err := os.WriteFile(path, make([]byte, 1024*1024), 0600); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		var calls []string
		pruned, err := enforceStorageCap(path, 1, func() error {
			calls = append(calls, "prune")
			return nil
		}, func() error {
			calls = append(calls, "vacuum")
			return nil
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !pruned {
			t.Fatal("expected pruning at cap")
		}
		if len(calls) != 2 || calls[0] != "prune" || calls[1] != "vacuum" {
			t.Fatalf("expected prune then vacuum, got %v", calls)
		}
	})

	t.Run("prune error skips vacuum", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "large.db")
		if err := os.WriteFile(path, make([]byte, 1024*1024), 0600); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		expectedErr := errors.New("prune failed")
		pruned, err := enforceStorageCap(path, 1, func() error {
			return expectedErr
		}, func() error {
			t.Fatal("vacuum should not run after prune error")
			return nil
		})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected prune error, got %v", err)
		}
		if pruned {
			t.Fatal("expected no pruning result after prune error")
		}
	})
}
