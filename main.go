package main

import (
	"log/slog"
	"os"
)

func main() {
	config := LoadConfig()
	InitLogger(config.LogLevel)

	slog.Info("KOSYNC starting",
		"port", config.Port,
		"db_path", config.DBPath,
		"log_level", config.LogLevel,
	)

	storage, err := InitDB(config.DBPath)
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer storage.Close()

	slog.Info("database initialized successfully")
}
