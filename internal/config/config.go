package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Port                int    `mapstructure:"port"`
	DatabasePath        string `mapstructure:"database_path"`
	LogLevel            string `mapstructure:"log_level"`
	JSONLog             bool   `mapstructure:"json_log"`
	LogPath             string `mapstructure:"log_path"`
	DisableRegistration bool   `mapstructure:"disable_registration"`
	StorageCapMB        int    `mapstructure:"storage_cap_mb"`
}

// Validate checks that the configuration values are within acceptable bounds.
// Port must be 0 (for tests) or 1-65535. DatabasePath must not be empty.
// LogLevel must be one of debug, info, warn, or error. StorageCapMB must be non-negative.
func (c *Config) Validate() error {
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d: must be 0–65535", c.Port)
	}
	if c.DatabasePath == "" {
		return fmt.Errorf("database_path must not be empty")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
		// valid
	default:
		return fmt.Errorf("invalid log_level %q: must be debug, info, warn, or error", c.LogLevel)
	}
	if c.StorageCapMB < 0 {
		return fmt.Errorf("storage_cap_mb must be non-negative, got %d", c.StorageCapMB)
	}
	return nil
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.SetDefault("port", 8081)
	viper.SetDefault("database_path", "data/kosync.db")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("json_log", false)
	viper.SetDefault("log_path", "")
	viper.SetDefault("disable_registration", true)
	viper.SetDefault("storage_cap_mb", 0)

	viper.SetEnvPrefix("KOSYNC")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	if err := viper.BindEnv("database_path", "KOSYNC_DATABASE_PATH", "KOSYNC_DB_PATH"); err != nil {
		return nil, err
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if cfg.DatabasePath == "" || !databasePathConfigured() {
		if legacyDBPath := viper.GetString("db_path"); legacyDBPath != "" {
			cfg.DatabasePath = legacyDBPath
		}
	}

	resolveExecutablePaths("kosync", &cfg.DatabasePath, &cfg.LogPath)

	return &cfg, nil
}

func resolveExecutablePaths(appName string, databasePath *string, logPath *string, extraPaths ...*string) {
	exePath, err := os.Executable()
	if err != nil {
		return
	}

	exeDir := filepath.Dir(exePath)
	resolvePath(exeDir, databasePath)
	if logPath != nil && *logPath != "" {
		resolvePath(exeDir, logPath)
	} else if logPath != nil {
		defaultLog := filepath.Join(exeDir, appName+".log")
		if _, err := os.Stat(defaultLog); err == nil {
			*logPath = defaultLog
		}
	}
	for _, path := range extraPaths {
		resolvePath(exeDir, path)
	}
}

func resolvePath(exeDir string, path *string) {
	if path != nil && *path != "" && !filepath.IsAbs(*path) {
		*path = filepath.Join(exeDir, *path)
	}
}

func databasePathConfigured() bool {
	if viper.InConfig("database_path") {
		return true
	}
	_, ok := os.LookupEnv("KOSYNC_DATABASE_PATH")
	return ok
}
