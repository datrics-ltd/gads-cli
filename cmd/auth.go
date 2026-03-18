package cmd

import (
	"fmt"
	"os"

	"github.com/datrics-ltd/gads-cli/internal/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  `Commands for managing Google OAuth2 authentication.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google via OAuth2",
	Long: `Start a local HTTP server, open the browser to the Google OAuth2 consent
screen, and store the resulting refresh token in ~/.gads/credentials.json.

Requires client_id and client_secret to be configured:

  gads config set client_id     YOUR_CLIENT_ID
  gads config set client_secret YOUR_CLIENT_SECRET`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := viper.GetString("client_id")
		clientSecret := viper.GetString("client_secret")

		creds, err := auth.Login(clientID, clientSecret)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		fmt.Println("Authentication successful.")
		if !creds.Expiry.IsZero() {
			fmt.Printf("Access token expires: %s\n", creds.Expiry.Local().Format("2006-01-02 15:04:05"))
		}
		path, _ := auth.CredentialsPath()
		fmt.Printf("Credentials saved to: %s\n", path)
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication state",
	Long:  `Show whether you are logged in, when the access token expires, and whether required config values are set.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		developerToken := viper.GetString("developer_token")
		clientID := viper.GetString("client_id")

		status, err := auth.GetStatus(developerToken, clientID)
		if err != nil {
			return fmt.Errorf("checking auth status: %w", err)
		}

		// Developer token
		if status.DevTokenSet {
			fmt.Println("Developer token: configured")
		} else {
			fmt.Println("Developer token: NOT configured (run `gads config set developer_token <token>`)")
		}

		// Client ID
		if status.ClientIDSet {
			fmt.Println("Client ID:       configured")
		} else {
			fmt.Println("Client ID:       NOT configured (run `gads config set client_id <id>`)")
		}

		// Auth status
		if !status.LoggedIn {
			fmt.Println("Auth status:     not logged in (run `gads auth login`)")
			return nil
		}

		if status.UsingEnvToken {
			fmt.Println("Auth status:     using GADS_ACCESS_TOKEN environment variable")
			return nil
		}

		if status.TokenExpired {
			fmt.Printf("Auth status:     logged in (access token expired %s — will auto-refresh)\n",
				status.Expiry.Local().Format("2006-01-02 15:04:05"))
		} else {
			fmt.Printf("Auth status:     logged in (access token valid until %s)\n",
				status.Expiry.Local().Format("2006-01-02 15:04:05"))
		}

		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke tokens and delete local credentials",
	Long:  `Revoke the access token at Google and delete ~/.gads/credentials.json.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := viper.GetString("client_id")
		clientSecret := viper.GetString("client_secret")

		if err := auth.Logout(clientID, clientSecret); err != nil {
			return fmt.Errorf("logout failed: %w", err)
		}

		path, _ := auth.CredentialsPath()
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			fmt.Println("Logged out. (No credentials file found.)")
		} else {
			fmt.Println("Logged out. Credentials deleted.")
		}
		return nil
	},
}

var authRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Force-refresh the access token",
	Long:  `Use the stored refresh token to obtain a new access token immediately.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := viper.GetString("client_id")
		clientSecret := viper.GetString("client_secret")

		accessToken, err := auth.ForceRefresh(clientID, clientSecret)
		if err != nil {
			return fmt.Errorf("token refresh failed: %w", err)
		}

		quiet := viper.GetBool("quiet")
		if !quiet {
			fmt.Println("Access token refreshed successfully.")
		}

		verbose := viper.GetBool("verbose")
		if verbose {
			preview := accessToken
			if len(preview) > 20 {
				preview = preview[:20]
			}
			fmt.Fprintf(os.Stderr, "[DEBUG] access_token=%s...\n", preview)
		}

		return nil
	},
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authRefreshCmd)
	rootCmd.AddCommand(authCmd)
}
