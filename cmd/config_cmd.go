package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/datrics-ltd/gads-cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage gads configuration",
	Long: `Manage the gads configuration file at ~/.gads/config.yaml.

Values are read with this precedence (highest wins):
  1. Command-line flags
  2. Environment variables (GADS_ prefix)
  3. Config file (~/.gads/config.yaml)
  4. Defaults`,
	Example: `  gads config set developer_token AbCdEf123456
  gads config set default_customer_id 123-456-7890
  gads config get default_customer_id
  gads config list
  gads config path`,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Example: `  gads config set developer_token AbCdEf123456
  gads config set default_customer_id 123-456-7890
  gads config set default_output json`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Example: `  gads config get developer_token
  gads config get default_customer_id`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

var configListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all configuration values (sensitive values are redacted)",
	Example: `  gads config list`,
	Args:    cobra.NoArgs,
	RunE:    runConfigList,
}

var configPathCmd = &cobra.Command{
	Use:     "path",
	Short:   "Print the config file path",
	Example: `  gads config path`,
	Args:    cobra.NoArgs,
	RunE:    runConfigPath,
}

func init() {
	configCmd.AddCommand(configSetCmd, configGetCmd, configListCmd, configPathCmd)
	rootCmd.AddCommand(configCmd)
}

// sensitiveKeys is the set of config keys whose values should be redacted in output.
var sensitiveKeys = map[string]bool{
	"developer_token": true,
	"client_secret":   true,
	"access_token":    true,
	"refresh_token":   true,
}

func isSensitive(key string) bool {
	lower := strings.ToLower(key)
	if sensitiveKeys[lower] {
		return true
	}
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password")
}

func redactValue(key, value string) string {
	if isSensitive(key) && value != "" {
		return "***"
	}
	return value
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]

	path, err := config.ConfigFilePath()
	if err != nil {
		return err
	}

	// Read existing config file.
	existing := make(map[string]interface{})
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config: %w", err)
	}
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parsing config: %w", err)
		}
	}

	existing[key] = value

	out, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Set %s\n", key)
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	val := viper.GetString(key)
	if val == "" {
		return fmt.Errorf("key %q not set", key)
	}
	fmt.Fprintln(cmd.OutOrStdout(), val)
	return nil
}

func runConfigList(cmd *cobra.Command, args []string) error {
	path, err := config.ConfigFilePath()
	if err != nil {
		return err
	}

	// Read only what's in the config file to avoid noise from flag defaults.
	fileSettings := make(map[string]interface{})
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config: %w", err)
	}
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &fileSettings); err != nil {
			return fmt.Errorf("parsing config: %w", err)
		}
	}

	// Merge env overrides for known keys (only if the env var is actually set).
	knownEnvKeys := map[string]string{
		"developer_token": "GADS_DEVELOPER_TOKEN",
		"client_id":       "GADS_CLIENT_ID",
		"client_secret":   "GADS_CLIENT_SECRET",
		"customer_id":     "GADS_CUSTOMER_ID",
		"output":          "GADS_OUTPUT",
		"access_token":    "GADS_ACCESS_TOKEN",
	}
	for k, envVar := range knownEnvKeys {
		if v := os.Getenv(envVar); v != "" {
			if _, inFile := fileSettings[k]; !inFile {
				fileSettings[k] = v
			}
		}
	}

	if len(fileSettings) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no configuration set)")
		return nil
	}

	keys := make([]string, 0, len(fileSettings))
	for k := range fileSettings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		val := fmt.Sprintf("%v", fileSettings[k])
		fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", k, redactValue(k, val))
	}
	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	path, err := config.ConfigFilePath()
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), path)
	return nil
}
