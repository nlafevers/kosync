package logger

import (
	"log/slog"
)

// LogCLISuccess logs a successful CLI operation at INFO level.
func LogCLISuccess(logger *slog.Logger, operation, username string) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info(operation+" successfully",
		"username", username,
		"operation", operation,
		"source", "CLI",
	)
}

// LogCLIFailure logs a failed CLI operation at WARN level.
func LogCLIFailure(logger *slog.Logger, operation, username, reason string) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn(operation+" failed",
		"username", username,
		"operation", operation,
		"reason", reason,
		"source", "CLI",
	)
}
