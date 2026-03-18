package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var adsCmd = &cobra.Command{
	Use:   "ads",
	Short: "Manage Google Ads ads",
	Long:  "List, inspect, pause, and enable Google Ads ads.",
}

var adsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List ads in a campaign or ad group",
	Example: `  gads ads list --campaign 12345678901
  gads ads list --ad-group 12345678901
  gads ads list --campaign 12345678901 --output json`,
	RunE: runAdsList,
}

var adsGetCmd = &cobra.Command{
	Use:   "get <ad-id>",
	Short: "Show detailed view of a single ad",
	Args:  cobra.ExactArgs(1),
	Example: `  gads ads get 12345678901
  gads ads get 12345678901 --output json`,
	RunE: runAdsGet,
}

var adsPauseCmd = &cobra.Command{
	Use:   "pause <ad-id>",
	Short: "Set ad status to PAUSED",
	Args:  cobra.ExactArgs(1),
	Example: `  gads ads pause 12345678901`,
	RunE:    runAdsMutate("PAUSED"),
}

var adsEnableCmd = &cobra.Command{
	Use:   "enable <ad-id>",
	Short: "Set ad status to ENABLED",
	Args:  cobra.ExactArgs(1),
	Example: `  gads ads enable 12345678901`,
	RunE:    runAdsMutate("ENABLED"),
}

func runAdsList(cmd *cobra.Command, args []string) error {
	campaignID, _ := cmd.Flags().GetString("campaign")
	adGroupID, _ := cmd.Flags().GetString("ad-group")

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	var conditions []string
	conditions = append(conditions, "ad_group_ad.status != 'REMOVED'")
	if campaignID != "" {
		conditions = append(conditions, fmt.Sprintf("campaign.id = %s", campaignID))
	}
	if adGroupID != "" {
		conditions = append(conditions, fmt.Sprintf("ad_group.id = %s", adGroupID))
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	gaql := fmt.Sprintf(`SELECT
  ad_group_ad.ad.id,
  ad_group_ad.ad.name,
  ad_group_ad.ad.type,
  ad_group_ad.status,
  ad_group.id,
  ad_group.name,
  campaign.id,
  campaign.name,
  metrics.clicks,
  metrics.impressions,
  metrics.cost_micros,
  metrics.ctr
FROM ad_group_ad
%s
ORDER BY ad_group_ad.ad.name`, whereClause)

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func runAdsGet(cmd *cobra.Command, args []string) error {
	adID := args[0]

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	gaql := fmt.Sprintf(`SELECT
  ad_group_ad.ad.id,
  ad_group_ad.ad.name,
  ad_group_ad.ad.type,
  ad_group_ad.ad.final_urls,
  ad_group_ad.status,
  ad_group.id,
  ad_group.name,
  campaign.id,
  campaign.name,
  metrics.clicks,
  metrics.impressions,
  metrics.cost_micros,
  metrics.ctr,
  metrics.conversions
FROM ad_group_ad
WHERE ad_group_ad.ad.id = %s`, adID)

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func runAdsMutate(status string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		adID := args[0]

		client, err := buildAPIClient()
		if err != nil {
			return err
		}

		customerID := viper.GetString("customer_id")

		// Look up the resource name — ad_group_ad uses customers/{cid}/adGroupAds/{ag_id}~{ad_id}
		gaql := fmt.Sprintf(`SELECT
  ad_group_ad.resource_name
FROM ad_group_ad
WHERE ad_group_ad.ad.id = %s
LIMIT 1`, adID)

		_, rows, err := client.ExecuteGAQL(customerID, gaql)
		if err != nil {
			return fmt.Errorf("looking up ad %s: %w", adID, err)
		}
		if len(rows) == 0 {
			return fmt.Errorf("ad %s not found", adID)
		}

		resourceName, ok := rows[0]["ad_group_ad.resource_name"].(string)
		if !ok || resourceName == "" {
			return fmt.Errorf("could not determine resource name for ad %s", adID)
		}

		normalizedCustomerID := strings.ReplaceAll(customerID, "-", "")

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

		path := fmt.Sprintf("/v18/customers/%s/adGroupAds:mutate", normalizedCustomerID)
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
			fmt.Fprintf(os.Stderr, "Ad %s %s.\n", adID, action)
		}

		return nil
	}
}

func init() {
	adsListCmd.Flags().String("campaign", "", "Filter by campaign ID")
	adsListCmd.Flags().String("ad-group", "", "Filter by ad group ID")

	adsCmd.AddCommand(adsListCmd, adsGetCmd, adsPauseCmd, adsEnableCmd)
	rootCmd.AddCommand(adsCmd)
}
