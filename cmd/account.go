package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/datrics-ltd/gads-cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage Google Ads account settings",
	Long:  "View account info, list accessible customers, and switch the active customer ID.",
}

var accountInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show current account details",
	Example: `  gads account info
  gads account info --output json`,
	Args: cobra.NoArgs,
	RunE: runAccountInfo,
}

var accountCustomersCmd = &cobra.Command{
	Use:   "customers",
	Short: "List accessible customer accounts",
	Example: `  gads account customers
  gads account customers --output json`,
	Args: cobra.NoArgs,
	RunE: runAccountCustomers,
}

var accountSwitchCmd = &cobra.Command{
	Use:   "switch <customer-id>",
	Short: "Update the default customer ID in config",
	Example: `  gads account switch 123-456-7890
  gads account switch 1234567890`,
	Args: cobra.ExactArgs(1),
	RunE: runAccountSwitch,
}

func runAccountInfo(cmd *cobra.Command, args []string) error {
	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	customerID := viper.GetString("customer_id")

	gaql := `SELECT
  customer.id,
  customer.descriptive_name,
  customer.currency_code,
  customer.time_zone,
  customer.status,
  customer.manager,
  customer.test_account,
  customer.auto_tagging_enabled
FROM customer
LIMIT 1`

	headers, rows, err := client.ExecuteGAQL(customerID, gaql)
	if err != nil {
		return err
	}

	return renderOutput(headers, rows, customerID, gaql)
}

func runAccountCustomers(cmd *cobra.Command, args []string) error {
	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	resp, err := client.Get("/v18/customers:listAccessibleCustomers", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	var result struct {
		ResourceNames []string `json:"resourceNames"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	// Extract customer IDs from resource names like "customers/1234567890"
	headers := []string{"customer_id", "resource_name"}
	rows := make([]map[string]interface{}, 0, len(result.ResourceNames))
	for _, rn := range result.ResourceNames {
		parts := strings.SplitN(rn, "/", 2)
		id := rn
		if len(parts) == 2 {
			id = parts[1]
		}
		rows = append(rows, map[string]interface{}{
			"customer_id":   id,
			"resource_name": rn,
		})
	}

	return renderOutput(headers, rows, "", "")
}

func runAccountSwitch(cmd *cobra.Command, args []string) error {
	newCustomerID := args[0]

	path, err := config.ConfigFilePath()
	if err != nil {
		return err
	}

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

	existing["default_customer_id"] = newCustomerID

	out, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	if !viper.GetBool("quiet") {
		fmt.Fprintf(os.Stderr, "Default customer ID updated to %s.\n", newCustomerID)
	}
	return nil
}

func init() {
	accountCmd.AddCommand(accountInfoCmd, accountCustomersCmd, accountSwitchCmd)
	rootCmd.AddCommand(accountCmd)
}
