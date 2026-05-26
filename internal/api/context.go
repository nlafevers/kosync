package api

import (
	"context"
	"log/slog"
)

type contextKey string

const (
	ContextKeyRequestID contextKey = "request_id"
	ContextKeyUser      contextKey = "user"
	ContextKeyLogger    contextKey = "logger"
)

// GetLogger returns the request-scoped logger from context or the default logger if not found.
func GetLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(ContextKeyLogger).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// GetUser returns the authenticated username from context.
func GetUser(ctx context.Context) string {
	if user, ok := ctx.Value(ContextKeyUser).(string); ok {
		return user
	}
	return ""
}

// GetRequestID returns the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return id
	}
	return ""
}
