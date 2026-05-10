package main

import (
	"log/slog"
)

func main() {
	config := LoadConfig()
	logger := InitLogger(config.LogLevel)

	logger.Info("KOSYNC starting",
		"port", config.Port,
		"db_path", config.DBPath,
		"log_level", config.LogLevel,
	)
}
