package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datrics-ltd/gads-cli/internal/config"
	"github.com/datrics-ltd/gads-cli/internal/schema"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var queryCmd = &cobra.Command{
	Use:   "query [GAQL]",
	Short: "Execute a GAQL query against the Google Ads API",
	Long: `Execute a Google Ads Query Language (GAQL) query and display results.

Examples:
  # Inline query
  gads query "SELECT campaign.name, metrics.clicks FROM campaign WHERE segments.date DURING LAST_7_DAYS"

  # Query from file
  gads query -f ./weekly-spend.gaql

  # Output as CSV, redirect to file
  gads query -f ./weekly-spend.gaql --output csv > report.csv

  # Output as JSON, pipe to jq
  gads query "SELECT campaign.name, metrics.clicks FROM campaign" --output json | jq '.[].metrics.clicks'`,
	Args: cobra.MaximumNArgs(1),
	RunE: runQuery,
}

func runQuery(cmd *cobra.Command, args []string) error {
	file, _ := cmd.Flags().GetString("file")

	var gaql string
	switch {
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("reading query file %q: %w", file, err)
		}
		gaql = strings.TrimSpace(string(data))
	case len(args) == 1:
		gaql = args[0]
	default:
		return fmt.Errorf("provide a GAQL query as an argument or use -f <file.gaql>")
	}

	// Validate GAQL before sending to the API.
	if !viper.GetBool("no_validate") {
		if err := schema.ValidateGAQL(gaql); err != nil {
			return err
		}
	}

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")
	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

// savedQueriesDir returns the path to the saved queries directory (~/.gads/queries).
func savedQueriesDir() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "queries"), nil
}

// savedQueryPath returns the full path for a saved query file.
func savedQueryPath(name string) (string, error) {
	dir, err := savedQueriesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".gaql"), nil
}

var querySaveCmd = &cobra.Command{
	Use:   "save <name> [GAQL]",
	Short: "Save a GAQL query for later use",
	Long: `Save a GAQL query to the config directory for later execution with 'gads query run'.

Examples:
  # Save an inline query
  gads query save weekly-spend "SELECT campaign.name, metrics.clicks FROM campaign WHERE segments.date DURING LAST_7_DAYS"

  # Save from file
  gads query save weekly-spend -f ./weekly-spend.gaql`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		file, _ := cmd.Flags().GetString("file")

		var gaql string
		switch {
		case file != "":
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("reading query file %q: %w", file, err)
			}
			gaql = strings.TrimSpace(string(data))
		case len(args) == 2:
			gaql = args[1]
		default:
			return fmt.Errorf("provide a GAQL query as an argument or use -f <file.gaql>")
		}

		dir, err := savedQueriesDir()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("creating queries directory: %w", err)
		}

		path, err := savedQueryPath(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(gaql+"\n"), 0o600); err != nil {
			return fmt.Errorf("saving query: %w", err)
		}

		if !viper.GetBool("quiet") {
			fmt.Fprintf(os.Stderr, "Saved query %q to %s\n", name, path)
		}
		return nil
	},
}

var queryRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Execute a saved GAQL query",
	Long: `Execute a previously saved GAQL query.

Examples:
  gads query run weekly-spend
  gads query run weekly-spend --output csv > report.csv`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		path, err := savedQueryPath(name)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("saved query %q not found — use 'gads query saved' to list saved queries", name)
			}
			return fmt.Errorf("reading saved query: %w", err)
		}
		gaql := strings.TrimSpace(string(data))

		if !viper.GetBool("no_validate") {
			if err := schema.ValidateGAQL(gaql); err != nil {
				return err
			}
		}

		client, err := buildAPIClient()
		if err != nil {
			return err
		}

		customerID := viper.GetString("customer_id")
		headers, rows, err := client.ExecuteGAQL(customerID, gaql)
		if err != nil {
			return err
		}

		return renderOutput(headers, rows, customerID, gaql)
	},
}

var querySavedCmd = &cobra.Command{
	Use:     "saved",
	Short:   "List all saved GAQL queries",
	Long:    `List all GAQL queries that have been saved with 'gads query save'.`,
	Example: `  gads query saved`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := savedQueriesDir()
		if err != nil {
			return err
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No saved queries.")
				return nil
			}
			return fmt.Errorf("reading queries directory: %w", err)
		}

		var names []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".gaql") {
				names = append(names, strings.TrimSuffix(e.Name(), ".gaql"))
			}
		}

		if len(names) == 0 {
			fmt.Println("No saved queries.")
			return nil
		}

		for _, name := range names {
			path, _ := savedQueryPath(name)
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("%-30s (unreadable)\n", name)
				continue
			}
			// Show first line of the query as preview
			preview := strings.TrimSpace(string(data))
			if nl := strings.Index(preview, "\n"); nl != -1 {
				preview = preview[:nl] + "..."
			}
			if len(preview) > 60 {
				preview = preview[:60] + "..."
			}
			fmt.Printf("%-20s  %s\n", name, preview)
		}
		return nil
	},
}

func init() {
	queryCmd.Flags().StringP("file", "f", "", "Read GAQL query from file")
	queryCmd.Flags().Bool("no-validate", false, "Skip local GAQL validation (send query directly to API)")
	_ = viper.BindPFlag("no_validate", queryCmd.Flags().Lookup("no-validate"))

	querySaveCmd.Flags().StringP("file", "f", "", "Read GAQL query from file")

	queryCmd.AddCommand(querySaveCmd)
	queryCmd.AddCommand(queryRunCmd)
	queryCmd.AddCommand(querySavedCmd)
	rootCmd.AddCommand(queryCmd)
}
