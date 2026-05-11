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
	LogPath             string
	DisableRegistration bool
	StorageCapMB        int
}

func LoadConfig() Config {
	exePath, err := os.Executable()
	exeDir := ""
	if err == nil {
		exeDir = filepath.Dir(exePath)
	}

	dbPath := getEnv("KOSYNC_DB_PATH", "kosync.db")
	if !filepath.IsAbs(dbPath) && exeDir != "" {
		dbPath = filepath.Join(exeDir, dbPath)
	}

	logPath := getEnv("KOSYNC_LOG_PATH", "")
	if logPath != "" {
		if !filepath.IsAbs(logPath) && exeDir != "" {
			logPath = filepath.Join(exeDir, logPath)
		}
	} else if exeDir != "" {
		// Auto-discover kosync.log in the application directory
		defaultLog := filepath.Join(exeDir, "kosync.log")
		if _, err := os.Stat(defaultLog); err == nil {
			logPath = defaultLog
		}
	}

	return Config{
		Port:                getEnv("KOSYNC_PORT", "8081"),
		DBPath:              dbPath,
		LogLevel:            getEnv("KOSYNC_LOG_LEVEL", "info"),
		LogPath:             logPath,
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
