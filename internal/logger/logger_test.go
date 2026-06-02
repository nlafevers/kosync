package logger

import (
	"os"
	"strings"
	"testing"
)

func TestNewCLI(t *testing.T) {
	t.Run("NoLogPath returns non-nil logger", func(t *testing.T) {
		l := NewCLI("info", false, "")
		if l == nil {
			t.Fatal("expected non-nil logger")
		}
		// Calling Log must not panic.
		l.Info("discarded message")
	})

	t.Run("WithLogPath writes to file and not to stderr", func(t *testing.T) {
		tmp := t.TempDir()
		logFile := tmp + "/test.log"

		l := NewCLI("info", false, logFile)
		if l == nil {
			t.Fatal("expected non-nil logger")
		}
		l.Info("hello from cli", "key", "val")

		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}
		if !strings.Contains(string(data), "hello from cli") {
			t.Errorf("log file should contain message, got: %s", data)
		}
	})

	t.Run("WithLogPath JSON format writes JSON", func(t *testing.T) {
		tmp := t.TempDir()
		logFile := tmp + "/test.json"

		l := NewCLI("info", true, logFile)
		l.Info("json message")

		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}
		if !strings.Contains(string(data), `"msg":"json message"`) {
			t.Errorf("expected JSON log entry, got: %s", data)
		}
	})
}
