package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var adGroupsCmd = &cobra.Command{
	Use:   "ad-groups",
	Short: "Manage Google Ads ad groups",
	Long:  "List, inspect, pause, enable, and view stats for Google Ads ad groups.",
}

var adGroupsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List ad groups in a campaign",
	Example: `  gads ad-groups list --campaign 12345678901
  gads ad-groups list --campaign 12345678901 --output json`,
	RunE: runAdGroupsList,
}

var adGroupsGetCmd = &cobra.Command{
	Use:   "get <ad-group-id>",
	Short: "Show detailed view of a single ad group",
	Args:  cobra.ExactArgs(1),
	Example: `  gads ad-groups get 12345678901
  gads ad-groups get 12345678901 --output json`,
	RunE: runAdGroupsGet,
}

var adGroupsPauseCmd = &cobra.Command{
	Use:   "pause <ad-group-id>",
	Short: "Set ad group status to PAUSED",
	Args:  cobra.ExactArgs(1),
	Example: `  gads ad-groups pause 12345678901`,
	RunE:    runAdGroupsMutate("PAUSED"),
}

var adGroupsEnableCmd = &cobra.Command{
	Use:   "enable <ad-group-id>",
	Short: "Set ad group status to ENABLED",
	Args:  cobra.ExactArgs(1),
	Example: `  gads ad-groups enable 12345678901`,
	RunE:    runAdGroupsMutate("ENABLED"),
}

var adGroupsStatsCmd = &cobra.Command{
	Use:   "stats <ad-group-id>",
	Short: "Show ad group performance metrics for a date range",
	Args:  cobra.ExactArgs(1),
	Example: `  gads ad-groups stats 12345678901
  gads ad-groups stats 12345678901 --date-range LAST_30_DAYS
  gads ad-groups stats 12345678901 --from 2026-03-01 --to 2026-03-18`,
	RunE: runAdGroupsStats,
}

func runAdGroupsList(cmd *cobra.Command, args []string) error {
	campaignID, _ := cmd.Flags().GetString("campaign")

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	whereClause := "WHERE ad_group.status != 'REMOVED'"
	if campaignID != "" {
		whereClause = fmt.Sprintf("WHERE campaign.id = %s AND ad_group.status != 'REMOVED'", campaignID)
	}

	gaql := fmt.Sprintf(`SELECT
  ad_group.id,
  ad_group.name,
  ad_group.status,
  campaign.id,
  campaign.name,
  metrics.clicks,
  metrics.impressions,
  metrics.cost_micros,
  metrics.ctr
FROM ad_group
%s
ORDER BY ad_group.name`, whereClause)

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func runAdGroupsGet(cmd *cobra.Command, args []string) error {
	adGroupID := args[0]

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	gaql := fmt.Sprintf(`SELECT
  ad_group.id,
  ad_group.name,
  ad_group.status,
  ad_group.type,
  campaign.id,
  campaign.name,
  metrics.clicks,
  metrics.impressions,
  metrics.cost_micros,
  metrics.ctr,
  metrics.conversions
FROM ad_group
WHERE ad_group.id = %s`, adGroupID)

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func runAdGroupsMutate(status string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		adGroupID := args[0]

		client, err := buildAPIClient()
		if err != nil {
			return err
		}

		customerID := viper.GetString("customer_id")
		normalizedCustomerID := strings.ReplaceAll(customerID, "-", "")

		resourceName := fmt.Sprintf("customers/%s/adGroups/%s", normalizedCustomerID, adGroupID)

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

		path := fmt.Sprintf("/v18/customers/%s/adGroups:mutate", normalizedCustomerID)
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
			fmt.Fprintf(os.Stderr, "Ad group %s %s.\n", adGroupID, action)
		}

		return nil
	}
}

func runAdGroupsStats(cmd *cobra.Command, args []string) error {
	adGroupID := args[0]
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
  ad_group.id,
  ad_group.name,
  metrics.clicks,
  metrics.impressions,
  metrics.cost_micros,
  metrics.ctr,
  metrics.conversions,
  metrics.conversion_rate,
  metrics.average_cpc
FROM ad_group
WHERE ad_group.id = %s
%s`, adGroupID, dateFilter)

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func init() {
	adGroupsListCmd.Flags().String("campaign", "", "Filter by campaign ID")

	adGroupsStatsCmd.Flags().String("date-range", "LAST_7_DAYS", "Date range preset (e.g. LAST_7_DAYS, LAST_30_DAYS, THIS_MONTH)")
	adGroupsStatsCmd.Flags().String("from", "", "Start date (YYYY-MM-DD), used with --to for a custom range")
	adGroupsStatsCmd.Flags().String("to", "", "End date (YYYY-MM-DD), used with --from for a custom range")

	adGroupsCmd.AddCommand(adGroupsListCmd, adGroupsGetCmd, adGroupsPauseCmd, adGroupsEnableCmd, adGroupsStatsCmd)
	rootCmd.AddCommand(adGroupsCmd)
}
