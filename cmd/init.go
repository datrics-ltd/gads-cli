package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/datrics-ltd/gads-cli/internal/auth"
	"github.com/datrics-ltd/gads-cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive setup wizard for gads",
	Long: `Walk through configuring gads for the first time.

Prompts for your Google Ads API credentials, writes them to
~/.gads/config.yaml, optionally runs the OAuth2 login flow, and
verifies connectivity with a test query.`,
	Example: `  gads init`,
	Args:    cobra.NoArgs,
	RunE:    runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Welcome to gads — the Google Ads CLI.")
	fmt.Println()
	fmt.Println("This wizard will configure your credentials in ~/.gads/config.yaml.")
	fmt.Println("You can re-run it at any time to update your settings.")
	fmt.Println()

	// Check for existing config and warn if present.
	configPath, err := config.ConfigFilePath()
	if err != nil {
		return err
	}
	existingData, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config: %w", err)
	}
	existing := make(map[string]interface{})
	if len(existingData) > 0 {
		if err := yaml.Unmarshal(existingData, &existing); err != nil {
			return fmt.Errorf("parsing config: %w", err)
		}
	}
	if len(existing) > 0 {
		fmt.Printf("Warning: a config file already exists at %s\n", configPath)
		fmt.Print("Overwrite it? [y/N] ")
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
			fmt.Println("Aborted. Existing config unchanged.")
			return nil
		}
		fmt.Println()
	}

	// Developer token
	fmt.Println("1. Developer token")
	fmt.Println("   Find it in Google Ads → Tools → API Center.")
	developerToken := promptRequired(scanner, "   Developer token: ")

	// OAuth2 client ID
	fmt.Println()
	fmt.Println("2. OAuth2 client ID")
	fmt.Println("   Find it in Google Cloud Console → APIs & Services → Credentials.")
	clientID := promptRequired(scanner, "   Client ID: ")

	// OAuth2 client secret
	fmt.Println()
	fmt.Println("3. OAuth2 client secret")
	fmt.Println("   Same location as the client ID above.")
	clientSecret := promptRequired(scanner, "   Client secret: ")

	// Customer ID
	fmt.Println()
	fmt.Println("4. Customer ID")
	fmt.Println("   The 10-digit number shown at the top of your Google Ads dashboard (e.g. 123-456-7890).")
	customerID := promptRequired(scanner, "   Customer ID: ")

	// Login customer ID (MCC) — optional
	fmt.Println()
	fmt.Println("5. Login customer ID (MCC) — optional")
	fmt.Println("   Only needed if you access this account through a manager (MCC) account.")
	fmt.Println("   Leave blank to skip.")
	fmt.Print("   Login customer ID: ")
	scanner.Scan()
	loginCustomerID := strings.TrimSpace(scanner.Text())

	// Write config
	cfg := map[string]interface{}{
		"developer_token": developerToken,
		"client_id":       clientID,
		"client_secret":   clientSecret,
		"customer_id":     customerID,
	}
	if loginCustomerID != "" {
		cfg["login_customer_id"] = loginCustomerID
	}

	// Preserve any keys we didn't touch (e.g. output, profiles).
	for k, v := range existing {
		if _, set := cfg[k]; !set {
			cfg[k] = v
		}
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.WriteFile(configPath, out, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	fmt.Printf("\nConfig written to %s\n", configPath)

	// Update viper so subsequent calls in this process see the new values.
	viper.Set("developer_token", developerToken)
	viper.Set("client_id", clientID)
	viper.Set("client_secret", clientSecret)
	viper.Set("customer_id", customerID)
	if loginCustomerID != "" {
		viper.Set("login_customer_id", loginCustomerID)
	}

	// Ask whether to authenticate now.
	fmt.Println()
	fmt.Print("Authenticate with Google now? [Y/n] ")
	scanner.Scan()
	authAnswer := strings.TrimSpace(scanner.Text())
	if authAnswer == "" || strings.EqualFold(authAnswer, "y") || strings.EqualFold(authAnswer, "yes") {
		creds, err := auth.Login(clientID, clientSecret)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		fmt.Println("Authentication successful.")
		if !creds.Expiry.IsZero() {
			fmt.Printf("Access token expires: %s\n", creds.Expiry.Local().Format("2006-01-02 15:04:05"))
		}

		// Run a test query to verify end-to-end connectivity.
		fmt.Println()
		fmt.Println("Running a test query to verify connectivity...")
		const testGAQL = "SELECT customer.descriptive_name FROM customer LIMIT 1"
		client, err := buildAPIClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not build API client for test query: %v\n", err)
		} else {
			_, rows, err := client.ExecuteGAQL(customerID, testGAQL)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: test query failed: %v\n", err)
				fmt.Fprintln(os.Stderr, "Your credentials are saved — re-run 'gads auth login' if needed.")
			} else if len(rows) > 0 {
				name, _ := rows[0]["customer.descriptive_name"].(string)
				if name != "" {
					fmt.Printf("Connected! Account: %s\n", name)
				} else {
					fmt.Println("Test query succeeded.")
				}
			} else {
				fmt.Println("Test query succeeded.")
			}
		}
	} else {
		fmt.Println("Skipping authentication. Run 'gads auth login' when you're ready.")
	}

	fmt.Println()
	fmt.Println("Setup complete. Here are some commands to try:")
	fmt.Println()
	fmt.Println("  gads campaigns list              # list campaigns")
	fmt.Println("  gads account info                # show account details")
	fmt.Println(`  gads query "SELECT campaign.name, metrics.clicks FROM campaign LIMIT 10"`)
	fmt.Println("  gads auth status                 # check authentication status")
	fmt.Println()

	return nil
}

// promptRequired reads a non-empty trimmed line, re-prompting until one is provided.
func promptRequired(scanner *bufio.Scanner, prompt string) string {
	for {
		fmt.Print(prompt)
		scanner.Scan()
		val := strings.TrimSpace(scanner.Text())
		if val != "" {
			return val
		}
		fmt.Println("   This field is required.")
	}
}
