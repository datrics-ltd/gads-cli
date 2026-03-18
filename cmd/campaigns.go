package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var campaignsCmd = &cobra.Command{
	Use:   "campaigns",
	Short: "Manage Google Ads campaigns",
	Long:  "List, inspect, pause, enable, and view stats for Google Ads campaigns.",
}

var campaignsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List campaigns with ID, name, status, budget, and basic metrics",
	Example: `  gads campaigns list
  gads campaigns list --status ENABLED
  gads campaigns list --output json`,
	RunE: runCampaignsList,
}

var campaignsGetCmd = &cobra.Command{
	Use:   "get <campaign-id>",
	Short: "Show detailed view of a single campaign",
	Args:  cobra.ExactArgs(1),
	Example: `  gads campaigns get 12345678901
  gads campaigns get 12345678901 --output json`,
	RunE: runCampaignsGet,
}

var campaignsPauseCmd = &cobra.Command{
	Use:   "pause <campaign-id>",
	Short: "Set campaign status to PAUSED",
	Args:  cobra.ExactArgs(1),
	Example: `  gads campaigns pause 12345678901`,
	RunE:    runCampaignsMutate("PAUSED"),
}

var campaignsEnableCmd = &cobra.Command{
	Use:   "enable <campaign-id>",
	Short: "Set campaign status to ENABLED",
	Args:  cobra.ExactArgs(1),
	Example: `  gads campaigns enable 12345678901`,
	RunE:    runCampaignsMutate("ENABLED"),
}

var campaignsStatsCmd = &cobra.Command{
	Use:   "stats <campaign-id>",
	Short: "Show campaign performance metrics for a date range",
	Args:  cobra.ExactArgs(1),
	Example: `  gads campaigns stats 12345678901
  gads campaigns stats 12345678901 --date-range LAST_30_DAYS
  gads campaigns stats 12345678901 --from 2026-03-01 --to 2026-03-18`,
	RunE: runCampaignsStats,
}

func runCampaignsList(cmd *cobra.Command, args []string) error {
	statusFilter, _ := cmd.Flags().GetString("status")

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	whereClause := "WHERE campaign.status != 'REMOVED'"
	if statusFilter != "" {
		whereClause = fmt.Sprintf("WHERE campaign.status = '%s'", strings.ToUpper(statusFilter))
	}

	gaql := fmt.Sprintf(`SELECT
  campaign.id,
  campaign.name,
  campaign.status,
  campaign_budget.amount_micros,
  metrics.clicks,
  metrics.impressions,
  metrics.cost_micros,
  metrics.ctr
FROM campaign
%s
ORDER BY campaign.name`, whereClause)

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func runCampaignsGet(cmd *cobra.Command, args []string) error {
	campaignID := args[0]

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	gaql := fmt.Sprintf(`SELECT
  campaign.id,
  campaign.name,
  campaign.status,
  campaign.advertising_channel_type,
  campaign_budget.amount_micros,
  metrics.clicks,
  metrics.impressions,
  metrics.cost_micros,
  metrics.ctr,
  metrics.conversions
FROM campaign
WHERE campaign.id = %s`, campaignID)

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func runCampaignsMutate(status string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		campaignID := args[0]

		client, err := buildAPIClient()
		if err != nil {
			return err
		}

		customerID := viper.GetString("customer_id")
		normalizedCustomerID := strings.ReplaceAll(customerID, "-", "")

		resourceName := fmt.Sprintf("customers/%s/campaigns/%s", normalizedCustomerID, campaignID)

		body, err := json.Marshal(map[string]interface{}{
			"operations": []map[string]interface{}{
				{
					"update": map[string]interface{}{
						"resourceName": resourceName,
						"status":       status,
					},
					"updateMask": "status",
				},
			},
		})
		if err != nil {
			return fmt.Errorf("encoding mutate request: %w", err)
		}

		path := fmt.Sprintf("/v18/customers/%s/campaigns:mutate", normalizedCustomerID)
		resp, err := client.Post(path, body, nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if !viper.GetBool("quiet") {
			action := "paused"
			if status == "ENABLED" {
				action = "enabled"
			}
			fmt.Fprintf(os.Stderr, "Campaign %s %s.\n", campaignID, action)
		}

		return nil
	}
}

func runCampaignsStats(cmd *cobra.Command, args []string) error {
	campaignID := args[0]
	dateRange, _ := cmd.Flags().GetString("date-range")
	fromDate, _ := cmd.Flags().GetString("from")
	toDate, _ := cmd.Flags().GetString("to")

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	var dateFilter string
	if fromDate != "" && toDate != "" {
		dateFilter = fmt.Sprintf("AND segments.date BETWEEN '%s' AND '%s'", fromDate, toDate)
	} else {
		dateFilter = fmt.Sprintf("AND segments.date DURING %s", dateRange)
	}

	gaql := fmt.Sprintf(`SELECT
  campaign.id,
  campaign.name,
  metrics.clicks,
  metrics.impressions,
  metrics.cost_micros,
  metrics.ctr,
  metrics.conversions,
  metrics.conversion_rate,
  metrics.average_cpc
FROM campaign
WHERE campaign.id = %s
%s`, campaignID, dateFilter)

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func init() {
	campaignsListCmd.Flags().String("status", "", "Filter by status: ENABLED, PAUSED, REMOVED")

	campaignsStatsCmd.Flags().String("date-range", "LAST_7_DAYS", "Date range preset (e.g. LAST_7_DAYS, LAST_30_DAYS, THIS_MONTH)")
	campaignsStatsCmd.Flags().String("from", "", "Start date (YYYY-MM-DD), used with --to for a custom range")
	campaignsStatsCmd.Flags().String("to", "", "End date (YYYY-MM-DD), used with --from for a custom range")

	campaignsCmd.AddCommand(campaignsListCmd, campaignsGetCmd, campaignsPauseCmd, campaignsEnableCmd, campaignsStatsCmd)
	rootCmd.AddCommand(campaignsCmd)
}
