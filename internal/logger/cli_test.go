package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestCLILogging(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&buf, nil))

	t.Run("LogCLISuccess", func(t *testing.T) {
		buf.Reset()
		LogCLISuccess(l, "create-user", "testuser")

		var log map[string]any
		if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
			t.Fatalf("failed to unmarshal log: %v", err)
		}

		if log["level"] != "INFO" {
			t.Errorf("expected level INFO, got %v", log["level"])
		}
		if log["msg"] != "create-user successfully" {
			t.Errorf("expected msg 'create-user successfully', got %v", log["msg"])
		}
		if log["username"] != "testuser" {
			t.Errorf("expected username 'testuser', got %v", log["username"])
		}
		if log["source"] != "CLI" {
			t.Errorf("expected source 'CLI', got %v", log["source"])
		}
	})

	t.Run("LogCLIFailure", func(t *testing.T) {
		buf.Reset()
		LogCLIFailure(l, "delete-user", "testuser", "database error")

		var log map[string]any
		if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
			t.Fatalf("failed to unmarshal log: %v", err)
		}

		if log["level"] != "WARN" {
			t.Errorf("expected level WARN, got %v", log["level"])
		}
		if log["msg"] != "delete-user failed" {
			t.Errorf("expected msg 'delete-user failed', got %v", log["msg"])
		}
		if log["reason"] != "database error" {
			t.Errorf("expected reason 'database error', got %v", log["reason"])
		}
		if log["source"] != "CLI" {
			t.Errorf("expected source 'CLI', got %v", log["source"])
		}
	})
}
