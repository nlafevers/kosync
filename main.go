package main

import (
	"log/slog"
)

func main() {
	config := LoadConfig()
	InitLogger(config.LogLevel)

	slog.Info("KOSYNC starting",
		"port", config.Port,
		"db_path", config.DBPath,
		"log_level", config.LogLevel,
	)
}
