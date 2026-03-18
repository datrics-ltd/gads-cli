// Package auth handles OAuth2 authentication for the Google Ads API.
// The full flow (browser-based interactive login, token refresh, credential
// storage) will be implemented in Phase 1 — Foundation.
package auth

// TokenSource is a placeholder interface for retrieving a valid OAuth2 access token.
// Implementations will handle auto-refresh using the stored refresh token.
type TokenSource interface {
	// AccessToken returns a valid (non-expired) access token, refreshing if necessary.
	AccessToken() (string, error)
}

// Credentials holds the OAuth2 tokens stored in ~/.gads/credentials.json.
type Credentials struct {
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	CreatedAt    string `json:"created_at"`
}
