package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Port                string `mapstructure:"port"`
	DBPath              string `mapstructure:"db_path"`
	LogLevel            string `mapstructure:"log_level"`
	LogPath             string `mapstructure:"log_path"`
	DisableRegistration bool   `mapstructure:"disable_registration"`
	StorageCapMB        int    `mapstructure:"storage_cap_mb"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.SetDefault("port", "8081")
	viper.SetDefault("db_path", "kosync.db")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("log_path", "")
	viper.SetDefault("disable_registration", false)
	viper.SetDefault("storage_cap_mb", 0)

	viper.SetEnvPrefix("KOSYNC")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Absolute path resolution for DBPath and LogPath
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if !filepath.IsAbs(cfg.DBPath) {
			cfg.DBPath = filepath.Join(exeDir, cfg.DBPath)
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
