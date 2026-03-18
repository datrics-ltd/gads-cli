package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/datrics-ltd/gads-cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "gads",
	Short: "Google Ads CLI",
	Long: `gads is a command-line interface for the Google Ads API.

It supports three tiers of interaction:
  - Named commands for common operations (campaigns, ad-groups, keywords, etc.)
  - GAQL queries for arbitrary data retrieval
  - Raw API calls as a full-coverage escape hatch

Documentation: https://github.com/datrics-ltd/gads-cli`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return config.Init()
	},
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return err
	}
	return nil
}

// SetVersion sets the version string on the root command.
func SetVersion(v string) {
	rootCmd.Version = v
}

func init() {
	// Global persistent flags
	pf := rootCmd.PersistentFlags()

	pf.StringP("customer-id", "c", "", "Google Ads customer ID (overrides default from config)")
	pf.StringP("output", "o", "table", "Output format: table, json, csv")
	pf.StringP("profile", "p", "", "Named profile from config file")
	pf.BoolP("verbose", "v", false, "Show debug output (API requests/responses)")
	pf.BoolP("quiet", "q", false, "Suppress all non-data output")
	pf.Bool("no-color", false, "Disable colored output")
	pf.Int("retries", 3, "Maximum number of retries for transient API errors")

	// Bind flags to viper
	mustBind("customer_id", "customer-id")
	mustBind("output", "output")
	mustBind("profile", "profile")
	mustBind("verbose", "verbose")
	mustBind("quiet", "quiet")
	mustBind("no_color", "no-color")
	mustBind("retries", "retries")

	// Env var prefix: GADS_
	viper.SetEnvPrefix("GADS")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()
}

func mustBind(viperKey, flagName string) {
	if err := viper.BindPFlag(viperKey, rootCmd.PersistentFlags().Lookup(flagName)); err != nil {
		panic(fmt.Sprintf("failed to bind flag %q to viper key %q: %v", flagName, viperKey, err))
	}
}
