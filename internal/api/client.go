// Package api provides the HTTP client for the Google Ads REST API.
// It handles dual authentication (developer token + OAuth2 Bearer),
// retry logic with exponential backoff, and error mapping.
package api

// Client is a placeholder for the Google Ads API HTTP client.
// It will be fully implemented in Phase 1 — Foundation.
type Client struct {
	// developerToken is sent as the developer-token header on every request.
	developerToken string
	// accessToken is the OAuth2 Bearer token.
	accessToken string
	// customerID is the default customer ID used when not specified per-call.
	customerID string
	// verbose enables debug logging of requests and responses.
	verbose bool
}

// NewClient creates a new API client with the provided credentials.
func NewClient(developerToken, accessToken, customerID string, verbose bool) *Client {
	return &Client{
		developerToken: developerToken,
		accessToken:    accessToken,
		customerID:     customerID,
		verbose:        verbose,
	}
}
