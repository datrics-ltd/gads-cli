package cmd

import (
	"fmt"
	"os"

	"github.com/datrics-ltd/gads-cli/internal/api"
	"github.com/datrics-ltd/gads-cli/internal/auth"
	"github.com/spf13/viper"
)

// buildAPIClient constructs an authenticated API client from the current viper config.
// It prefers GADS_ACCESS_TOKEN env var for CI/non-interactive use; otherwise it
// loads stored OAuth2 credentials and creates a refreshing token source.
func buildAPIClient() (*api.Client, error) {
	var tokenSource api.TokenSource
	if envToken := os.Getenv("GADS_ACCESS_TOKEN"); envToken != "" {
		tokenSource = &staticTokenSource{token: envToken}
	} else {
		clientID := viper.GetString("client_id")
		clientSecret := viper.GetString("client_secret")

		creds, err := auth.LoadCredentials()
		if err != nil {
			return nil, fmt.Errorf("loading credentials: %w", err)
		}
		if creds == nil || creds.RefreshToken == "" {
			return nil, fmt.Errorf("not authenticated — run `gads auth login`")
		}
		tokenSource = auth.NewTokenSource(clientID, clientSecret, creds)
	}

	return api.NewClient(api.Config{
		DeveloperToken: viper.GetString("developer_token"),
		TokenSource:    tokenSource,
		CustomerID:     viper.GetString("customer_id"),
		Verbose:        viper.GetBool("verbose"),
		Retries:        viper.GetInt("retries"),
	}), nil
}

// staticTokenSource returns a fixed access token (used when GADS_ACCESS_TOKEN is set).
type staticTokenSource struct {
	token string
}

func (s *staticTokenSource) AccessToken() (string, error) {
	return s.token, nil
}
