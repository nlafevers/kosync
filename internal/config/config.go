package config

import (
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
	viper.SetDefault("disable_registration", false)
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

	// Absolute path resolution for DatabasePath and LogPath
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if !filepath.IsAbs(cfg.DatabasePath) {
			cfg.DatabasePath = filepath.Join(exeDir, cfg.DatabasePath)
		}
		if cfg.LogPath != "" && !filepath.IsAbs(cfg.LogPath) {
			cfg.LogPath = filepath.Join(exeDir, cfg.LogPath)
		} else if cfg.LogPath == "" {
			// Auto-discover kosync.log in the application directory
			defaultLog := filepath.Join(exeDir, "kosync.log")
			if _, err := os.Stat(defaultLog); err == nil {
				cfg.LogPath = defaultLog
			}
		}
	}

	return &cfg, nil
}

func databasePathConfigured() bool {
	if viper.InConfig("database_path") {
		return true
	}
	_, ok := os.LookupEnv("KOSYNC_DATABASE_PATH")
	return ok
}
