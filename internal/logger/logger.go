package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// New initializes a new logger using slog.
func New(level string, json bool, logPath string) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	var output io.Writer = os.Stderr

	if logPath != "" {
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		} else {
			output = io.MultiWriter(os.Stderr, file)
		}
	}

	var handler slog.Handler
	if json {
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{Level: slogLevel})
	} else {
		handler = slog.NewTextHandler(output, &slog.HandlerOptions{Level: slogLevel})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
