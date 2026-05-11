package main

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Port                string
	DBPath              string
	LogLevel            string
	DisableRegistration bool
	StorageCapMB        int
}

func LoadConfig() Config {
	dbPath := getEnv("KOSYNC_DB_PATH", "kosync.db")

	// If the path is relative, resolve it relative to the executable's directory.
	if !filepath.IsAbs(dbPath) {
		exePath, err := os.Executable()
		if err == nil {
			exeDir := filepath.Dir(exePath)
			dbPath = filepath.Join(exeDir, dbPath)
		}
	}

	return Config{
		Port:                getEnv("KOSYNC_PORT", "8081"),
		DBPath:              dbPath,
		LogLevel:            getEnv("KOSYNC_LOG_LEVEL", "info"),
		DisableRegistration: getEnvBool("KOSYNC_DISABLE_REGISTRATION", false),
		StorageCapMB:        getEnvInt("KOSYNC_STORAGE_CAP_MB", 0),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return fallback
}
