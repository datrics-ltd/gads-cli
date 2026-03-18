package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var keywordsCmd = &cobra.Command{
	Use:   "keywords",
	Short: "Manage Google Ads keywords",
	Long:  "List, inspect, pause, enable, and add Google Ads keywords.",
}

var keywordsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List keywords in a campaign or ad group",
	Example: `  gads keywords list --campaign 12345678901
  gads keywords list --ad-group 12345678901
  gads keywords list --campaign 12345678901 --output json`,
	RunE: runKeywordsList,
}

var keywordsGetCmd = &cobra.Command{
	Use:   "get <keyword-id>",
	Short: "Show detailed view of a single keyword",
	Args:  cobra.ExactArgs(1),
	Example: `  gads keywords get 12345678901
  gads keywords get 12345678901 --output json`,
	RunE: runKeywordsGet,
}

var keywordsPauseCmd = &cobra.Command{
	Use:   "pause <keyword-id>",
	Short: "Set keyword status to PAUSED",
	Args:  cobra.ExactArgs(1),
	Example: `  gads keywords pause 12345678901`,
	RunE:    runKeywordsMutate("PAUSED"),
}

var keywordsEnableCmd = &cobra.Command{
	Use:   "enable <keyword-id>",
	Short: "Set keyword status to ENABLED",
	Args:  cobra.ExactArgs(1),
	Example: `  gads keywords enable 12345678901`,
	RunE:    runKeywordsMutate("ENABLED"),
}

var keywordsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a keyword to an ad group",
	Example: `  gads keywords add --ad-group 12345678901 --text "running shoes" --match-type BROAD
  gads keywords add --ad-group 12345678901 --text "running shoes" --match-type EXACT`,
	RunE: runKeywordsAdd,
}

func runKeywordsList(cmd *cobra.Command, args []string) error {
	campaignID, _ := cmd.Flags().GetString("campaign")
	adGroupID, _ := cmd.Flags().GetString("ad-group")

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	var conditions []string
	conditions = append(conditions, "ad_group_criterion.status != 'REMOVED'")
	conditions = append(conditions, "ad_group_criterion.type = 'KEYWORD'")
	if campaignID != "" {
		conditions = append(conditions, fmt.Sprintf("campaign.id = %s", campaignID))
	}
	if adGroupID != "" {
		conditions = append(conditions, fmt.Sprintf("ad_group.id = %s", adGroupID))
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	gaql := fmt.Sprintf(`SELECT
  ad_group_criterion.criterion_id,
  ad_group_criterion.keyword.text,
  ad_group_criterion.keyword.match_type,
  ad_group_criterion.status,
  ad_group.id,
  ad_group.name,
  campaign.id,
  campaign.name,
  metrics.clicks,
  metrics.impressions,
  metrics.cost_micros,
  metrics.ctr
FROM keyword_view
%s
ORDER BY ad_group_criterion.keyword.text`, whereClause)

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func runKeywordsGet(cmd *cobra.Command, args []string) error {
	keywordID := args[0]

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	gaql := fmt.Sprintf(`SELECT
  ad_group_criterion.criterion_id,
  ad_group_criterion.keyword.text,
  ad_group_criterion.keyword.match_type,
  ad_group_criterion.status,
  ad_group_criterion.quality_info.quality_score,
  ad_group.id,
  ad_group.name,
  campaign.id,
  campaign.name,
  metrics.clicks,
  metrics.impressions,
  metrics.cost_micros,
  metrics.ctr,
  metrics.conversions,
  metrics.average_cpc
FROM keyword_view
WHERE ad_group_criterion.criterion_id = %s`, keywordID)

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func runKeywordsMutate(status string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		keywordID := args[0]

		client, err := buildAPIClient()
		if err != nil {
			return err
		}

		customerID := viper.GetString("customer_id")

		// Look up the resource name
		gaql := fmt.Sprintf(`SELECT
  ad_group_criterion.resource_name
FROM keyword_view
WHERE ad_group_criterion.criterion_id = %s
  AND ad_group_criterion.type = 'KEYWORD'
LIMIT 1`, keywordID)

		_, rows, err := client.ExecuteGAQL(customerID, gaql)
		if err != nil {
			return fmt.Errorf("looking up keyword %s: %w", keywordID, err)
		}
		if len(rows) == 0 {
			return fmt.Errorf("keyword %s not found", keywordID)
		}

		resourceName, ok := rows[0]["ad_group_criterion.resource_name"].(string)
		if !ok || resourceName == "" {
			return fmt.Errorf("could not determine resource name for keyword %s", keywordID)
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

		path := fmt.Sprintf("/v18/customers/%s/adGroupCriteria:mutate", normalizedCustomerID)
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
			fmt.Fprintf(os.Stderr, "Keyword %s %s.\n", keywordID, action)
		}

		return nil
	}
}

func runKeywordsAdd(cmd *cobra.Command, args []string) error {
	adGroupID, _ := cmd.Flags().GetString("ad-group")
	text, _ := cmd.Flags().GetString("text")
	matchType, _ := cmd.Flags().GetString("match-type")

	if adGroupID == "" {
		return fmt.Errorf("--ad-group is required")
	}
	if text == "" {
		return fmt.Errorf("--text is required")
	}
	if matchType == "" {
		return fmt.Errorf("--match-type is required")
	}

	matchType = strings.ToUpper(matchType)
	switch matchType {
	case "BROAD", "PHRASE", "EXACT":
		// valid
	default:
		return fmt.Errorf("--match-type must be one of: BROAD, PHRASE, EXACT")
	}

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")
	normalizedCustomerID := strings.ReplaceAll(customerID, "-", "")

	adGroupResourceName := fmt.Sprintf("customers/%s/adGroups/%s", normalizedCustomerID, adGroupID)

	body, err := json.Marshal(map[string]interface{}{
		"operations": []map[string]interface{}{
			{
				"create": map[string]interface{}{
					"adGroup": adGroupResourceName,
					"status":  "ENABLED",
					"keyword": map[string]interface{}{
						"text":      text,
						"matchType": matchType,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("encoding mutate request: %w", err)
	}

	path := fmt.Sprintf("/v18/customers/%s/adGroupCriteria:mutate", normalizedCustomerID)
	resp, err := client.Post(path, body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !viper.GetBool("quiet") {
		fmt.Fprintf(os.Stderr, "Keyword \"%s\" (%s) added to ad group %s.\n", text, matchType, adGroupID)
	}

	return nil
}

func init() {
	keywordsListCmd.Flags().String("campaign", "", "Filter by campaign ID")
	keywordsListCmd.Flags().String("ad-group", "", "Filter by ad group ID")

	keywordsAddCmd.Flags().String("ad-group", "", "Ad group ID to add the keyword to (required)")
	keywordsAddCmd.Flags().String("text", "", "Keyword text (required)")
	keywordsAddCmd.Flags().String("match-type", "", "Match type: BROAD, PHRASE, or EXACT (required)")

	keywordsCmd.AddCommand(keywordsListCmd, keywordsGetCmd, keywordsPauseCmd, keywordsEnableCmd, keywordsAddCmd)
	rootCmd.AddCommand(keywordsCmd)
}
