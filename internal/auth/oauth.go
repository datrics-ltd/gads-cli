package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/pkg/browser"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// GoogleAdsScope is the OAuth2 scope required for the Google Ads API.
	GoogleAdsScope = "https://www.googleapis.com/auth/adwords"
	// revokeURL is Google's token revocation endpoint.
	revokeURL = "https://oauth2.googleapis.com/revoke"
)

// TokenSource is an interface for retrieving a valid OAuth2 access token.
// Implementations handle auto-refresh using the stored refresh token.
type TokenSource interface {
	// AccessToken returns a valid (non-expired) access token, refreshing if necessary.
	AccessToken() (string, error)
}

// OAuthTokenSource implements TokenSource using stored credentials and OAuth2.
type OAuthTokenSource struct {
	clientID     string
	clientSecret string
	creds        *Credentials
}

// NewTokenSource creates a TokenSource backed by the given credentials.
func NewTokenSource(clientID, clientSecret string, creds *Credentials) *OAuthTokenSource {
	return &OAuthTokenSource{
		clientID:     clientID,
		clientSecret: clientSecret,
		creds:        creds,
	}
}

// AccessToken returns a valid access token, auto-refreshing if expired.
func (ts *OAuthTokenSource) AccessToken() (string, error) {
	if !ts.creds.IsExpired() {
		return ts.creds.AccessToken, nil
	}
	return ts.Refresh()
}

// Refresh force-refreshes the access token using the stored refresh token.
func (ts *OAuthTokenSource) Refresh() (string, error) {
	if ts.creds.RefreshToken == "" {
		return "", errors.New("no refresh token stored — run `gads auth login`")
	}
	cfg := oauthConfig(ts.clientID, ts.clientSecret, "")
	// Construct an expired token to force a refresh.
	tok := &oauth2.Token{
		RefreshToken: ts.creds.RefreshToken,
		Expiry:       time.Now().Add(-time.Hour),
	}
	newTok, err := cfg.TokenSource(context.Background(), tok).Token()
	if err != nil {
		return "", fmt.Errorf("refreshing access token: %w", err)
	}
	ts.creds.AccessToken = newTok.AccessToken
	ts.creds.Expiry = newTok.Expiry
	if newTok.RefreshToken != "" {
		ts.creds.RefreshToken = newTok.RefreshToken
	}
	if err := SaveCredentials(ts.creds); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save refreshed token: %v\n", err)
	}
	return newTok.AccessToken, nil
}

// AuthStatus summarises the current authentication state.
type AuthStatus struct {
	LoggedIn       bool
	TokenExpired   bool
	Expiry         time.Time
	UsingEnvToken  bool
	DevTokenSet    bool
	ClientIDSet    bool
}

// GetStatus returns the current authentication status without making API calls.
func GetStatus(developerToken, clientID string) (*AuthStatus, error) {
	status := &AuthStatus{
		DevTokenSet: developerToken != "",
		ClientIDSet: clientID != "",
	}

	// Check for direct access token via env var.
	if os.Getenv("GADS_ACCESS_TOKEN") != "" {
		status.LoggedIn = true
		status.UsingEnvToken = true
		return status, nil
	}

	creds, err := LoadCredentials()
	if err != nil {
		return nil, err
	}
	if creds == nil || creds.RefreshToken == "" {
		return status, nil
	}

	status.LoggedIn = true
	status.Expiry = creds.Expiry
	status.TokenExpired = creds.IsExpired()
	return status, nil
}

// Login performs the full OAuth2 browser-based login flow.
// It starts a local HTTP server, opens the browser to the consent page,
// waits for the callback, exchanges the code for tokens, and persists them.
func Login(clientID, clientSecret string) (*Credentials, error) {
	if clientID == "" {
		return nil, errors.New("client_id not configured — run `gads config set client_id <id>`")
	}
	if clientSecret == "" {
		return nil, errors.New("client_secret not configured — run `gads config set client_secret <secret>`")
	}

	// Bind on a random available port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting callback server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	cfg := oauthConfig(clientID, clientSecret, redirectURI)

	state, err := randomState()
	if err != nil {
		return nil, err
	}

	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	type callbackResult struct {
		code string
		err  error
	}
	ch := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "invalid state parameter", http.StatusBadRequest)
			ch <- callbackResult{err: errors.New("invalid OAuth2 state — possible CSRF attack")}
			return
		}
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			http.Error(w, "authentication denied", http.StatusUnauthorized)
			ch <- callbackResult{err: fmt.Errorf("Google denied authentication: %s", errParam)}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "no authorization code received", http.StatusBadRequest)
			ch <- callbackResult{err: errors.New("no authorization code in callback")}
			return
		}
		_, _ = fmt.Fprint(w, "<html><body style=\"font-family:sans-serif;text-align:center;padding:60px\">"+
			"<h2>&#10003; Authentication successful</h2>"+
			"<p>You can close this tab and return to the terminal.</p></body></html>")
		ch <- callbackResult{code: code}
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(listener) }()
	defer func() { _ = srv.Close() }()

	fmt.Println("Opening browser for Google authentication...")
	fmt.Printf("If the browser does not open automatically, visit:\n%s\n\n", authURL)

	_ = browser.OpenURL(authURL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var res callbackResult
	select {
	case res = <-ch:
	case <-ctx.Done():
		return nil, errors.New("authentication timed out after 5 minutes")
	}
	if res.err != nil {
		return nil, res.err
	}

	tok, err := cfg.Exchange(context.Background(), res.code)
	if err != nil {
		return nil, fmt.Errorf("exchanging authorization code for tokens: %w", err)
	}
	if tok.RefreshToken == "" {
		return nil, errors.New("Google did not return a refresh token — ensure access_type=offline and prompt=consent are set")
	}

	creds := &Credentials{
		RefreshToken: tok.RefreshToken,
		AccessToken:  tok.AccessToken,
		TokenType:    tok.TokenType,
		Expiry:       tok.Expiry,
		CreatedAt:    time.Now().UTC(),
	}
	if err := SaveCredentials(creds); err != nil {
		return nil, fmt.Errorf("saving credentials: %w", err)
	}
	return creds, nil
}

// Logout revokes the access token at Google and deletes local credentials.
func Logout(clientID, clientSecret string) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}
	if creds == nil {
		return nil // already logged out
	}
	// Best-effort: try to revoke the token at Google.
	if creds.RefreshToken != "" && clientID != "" && clientSecret != "" {
		cfg := oauthConfig(clientID, clientSecret, "")
		tok := &oauth2.Token{
			RefreshToken: creds.RefreshToken,
			Expiry:       time.Now().Add(-time.Hour),
		}
		if freshTok, err := cfg.TokenSource(context.Background(), tok).Token(); err == nil {
			_, _ = http.PostForm(revokeURL, map[string][]string{"token": {freshTok.AccessToken}})
		}
	}
	return DeleteCredentials()
}

// ForceRefresh refreshes the access token using the stored refresh token.
func ForceRefresh(clientID, clientSecret string) (string, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	if creds == nil || creds.RefreshToken == "" {
		return "", errors.New("not logged in — run `gads auth login`")
	}
	ts := NewTokenSource(clientID, clientSecret, creds)
	return ts.Refresh()
}

func oauthConfig(clientID, clientSecret, redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes:       []string{GoogleAdsScope},
		Endpoint:     google.Endpoint,
	}
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating state token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
