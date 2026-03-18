package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	// DefaultConfigDir is the directory under the user's home for gads config.
	DefaultConfigDir = ".gads"
	// DefaultConfigFile is the name of the config file.
	DefaultConfigFile = "config.yaml"
)

// Init loads the configuration from the config file, environment variables,
// and previously bound flags. It auto-creates ~/.gads/ if needed.
//
// Precedence (highest to lowest): flag > env var > config file > default.
func Init() error {
	configDir, err := defaultConfigDir()
	if err != nil {
		return fmt.Errorf("resolving config directory: %w", err)
	}

	// Create config directory if it doesn't exist.
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("creating config directory %s: %w", configDir, err)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	// Env prefix and automatic binding are already set in cmd/root.go, but
	// re-asserting here is safe and makes this package self-contained.
	viper.SetEnvPrefix("GADS")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	// Explicitly bind known env vars so they override config file values even
	// when no flag was provided.
	envBindings := map[string]string{
		"developer_token": "GADS_DEVELOPER_TOKEN",
		"client_id":       "GADS_CLIENT_ID",
		"client_secret":   "GADS_CLIENT_SECRET",
		"customer_id":     "GADS_CUSTOMER_ID",
		"output":          "GADS_OUTPUT",
		"access_token":    "GADS_ACCESS_TOKEN",
	}
	for key, env := range envBindings {
		if err := viper.BindEnv(key, env); err != nil {
			return fmt.Errorf("binding env var %s: %w", env, err)
		}
	}

	// Read config file — it's OK if it doesn't exist yet.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("reading config file: %w", err)
		}
	}

	return nil
}

// ConfigFilePath returns the full path to the config file.
func ConfigFilePath() (string, error) {
	dir, err := defaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DefaultConfigFile), nil
}

// ConfigDir returns the resolved path to the gads config directory (~/.gads).
func ConfigDir() (string, error) {
	return defaultConfigDir()
}

func defaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, DefaultConfigDir), nil
}
