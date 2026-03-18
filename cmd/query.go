package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/datrics-ltd/gads-cli/internal/output"
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

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")
	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	fmtStr := viper.GetString("output")
	format, err := output.ParseFormat(fmtStr)
	if err != nil {
		return err
	}

	opts := output.Options{
		NoColor: viper.GetBool("no_color"),
		Compact: viper.GetBool("compact"),
		BOM:     viper.GetBool("bom"),
		Verbose: viper.GetBool("verbose"),
		Meta: output.Meta{
			CustomerID: customerID,
			Query:      gaql,
			Rows:       len(rows),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		},
	}

	formatter := output.New(format, opts)
	return formatter.Format(os.Stdout, headers, rows)
}

func init() {
	queryCmd.Flags().StringP("file", "f", "", "Read GAQL query from file")
	rootCmd.AddCommand(queryCmd)
}
