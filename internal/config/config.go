package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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

// ApplyProfile overlays the named profile's values on top of the base config.
// Profile values override config-file defaults but are themselves overridden by
// explicit flags (cmd.Changed) and environment variables.
//
// Profiles are stored in config.yaml under the "profiles" key:
//
//	profiles:
//	  client-a:
//	    customer_id: "111-222-3333"
//	  client-b:
//	    customer_id: "444-555-6666"
//	    output: json
func ApplyProfile(cmd *cobra.Command) error {
	profile := viper.GetString("profile")
	if profile == "" {
		return nil
	}

	profileKey := "profiles." + profile
	profileMap := viper.GetStringMap(profileKey)
	if len(profileMap) == 0 {
		return fmt.Errorf("profile %q not found in config", profile)
	}

	for k, v := range profileMap {
		// Convert profile key to flag name (underscores → dashes).
		flagName := strings.ReplaceAll(k, "_", "-")

		// If the user explicitly passed this flag on the command line, it wins.
		if f := cmd.Root().PersistentFlags().Lookup(flagName); f != nil && f.Changed {
			continue
		}

		// If a GADS_ env var for this key is set, it wins.
		envKey := "GADS_" + strings.ToUpper(k)
		if os.Getenv(envKey) != "" {
			continue
		}

		// Apply the profile value at the highest viper priority so it overrides
		// anything that came from the config file but yields to flags and env vars
		// (which we already checked above).
		viper.Set(k, v)
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
