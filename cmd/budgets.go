package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var budgetsCmd = &cobra.Command{
	Use:   "budgets",
	Short: "Manage Google Ads campaign budgets",
	Long:  "List, inspect, and update Google Ads campaign budgets.",
}

var budgetsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all campaign budgets",
	Example: `  gads budgets list
  gads budgets list --output json`,
	RunE: runBudgetsList,
}

var budgetsGetCmd = &cobra.Command{
	Use:   "get <budget-id>",
	Short: "Show detailed view of a single budget",
	Args:  cobra.ExactArgs(1),
	Example: `  gads budgets get 12345678
  gads budgets get 12345678 --output json`,
	RunE: runBudgetsGet,
}

var budgetsSetCmd = &cobra.Command{
	Use:   "set <budget-id>",
	Short: "Update the daily budget amount",
	Args:  cobra.ExactArgs(1),
	Example: `  gads budgets set 12345678 --amount 50.00
  gads budgets set 12345678 --amount 100`,
	RunE: runBudgetsSet,
}

func runBudgetsList(cmd *cobra.Command, args []string) error {
	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	gaql := `SELECT
  campaign_budget.id,
  campaign_budget.name,
  campaign_budget.amount_micros,
  campaign_budget.status,
  campaign_budget.delivery_method,
  campaign_budget.explicitly_shared,
  campaign_budget.reference_count
FROM campaign_budget
WHERE campaign_budget.status != 'REMOVED'
ORDER BY campaign_budget.name`

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func runBudgetsGet(cmd *cobra.Command, args []string) error {
	budgetID := args[0]

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	gaql := fmt.Sprintf(`SELECT
  campaign_budget.id,
  campaign_budget.name,
  campaign_budget.amount_micros,
  campaign_budget.status,
  campaign_budget.delivery_method,
  campaign_budget.explicitly_shared,
  campaign_budget.reference_count,
  campaign_budget.recommended_budget_amount_micros,
  campaign_budget.total_amount_micros
FROM campaign_budget
WHERE campaign_budget.id = %s`, budgetID)

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func runBudgetsSet(cmd *cobra.Command, args []string) error {
	budgetID := args[0]
	amountStr, _ := cmd.Flags().GetString("amount")
	if amountStr == "" {
		return fmt.Errorf("--amount is required")
	}

	amountFloat, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return fmt.Errorf("invalid amount %q: must be a number (e.g. 50.00)", amountStr)
	}
	if amountFloat <= 0 {
		return fmt.Errorf("amount must be greater than 0")
	}

	// Google Ads stores budgets in micros (1 unit = 1,000,000 micros)
	amountMicros := int64(amountFloat * 1_000_000)

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")
	normalizedCustomerID := strings.ReplaceAll(customerID, "-", "")

	resourceName := fmt.Sprintf("customers/%s/campaignBudgets/%s", normalizedCustomerID, budgetID)

	body, err := json.Marshal(map[string]interface{}{
		"operations": []map[string]interface{}{
			{
				"update": map[string]interface{}{
					"resourceName":  resourceName,
					"amountMicros": amountMicros,
				},
				"updateMask": "amountMicros",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("encoding mutate request: %w", err)
	}

	path := fmt.Sprintf("/v18/customers/%s/campaignBudgets:mutate", normalizedCustomerID)
	resp, err := client.Post(path, body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !viper.GetBool("quiet") {
		fmt.Fprintf(os.Stderr, "Budget %s updated to %.2f.\n", budgetID, amountFloat)
	}

	return nil
}

func init() {
	budgetsSetCmd.Flags().String("amount", "", "Daily budget amount (e.g. 50.00)")

	budgetsCmd.AddCommand(budgetsListCmd, budgetsGetCmd, budgetsSetCmd)
	rootCmd.AddCommand(budgetsCmd)
}
