// Package api provides the HTTP client for the Google Ads REST API.
// It handles dual authentication (developer token + OAuth2 Bearer),
// retry logic with exponential backoff, and error mapping.
package api

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// BaseURL is the Google Ads REST API base URL.
	BaseURL = "https://googleads.googleapis.com"
	// DefaultRetries is the default number of retry attempts for transient errors.
	DefaultRetries = 3
	// defaultTimeout is the per-request HTTP timeout.
	defaultTimeout = 60 * time.Second
)

// errWriter is where debug/progress output goes (stderr).
var errWriter = os.Stderr

// TokenSource provides a valid (auto-refreshed) OAuth2 access token.
type TokenSource interface {
	AccessToken() (string, error)
}

// Client is an authenticated HTTP client for the Google Ads REST API.
type Client struct {
	developerToken  string
	tokenSource     TokenSource
	customerID      string
	loginCustomerID string
	retries         int
	verbose         bool
	http            *http.Client
}

// Config holds options for creating a new Client.
type Config struct {
	DeveloperToken  string
	TokenSource     TokenSource
	CustomerID      string
	LoginCustomerID string
	Retries         int
	Verbose         bool
}

// NewClient creates a new API client with the provided configuration.
func NewClient(cfg Config) *Client {
	retries := cfg.Retries
	if retries <= 0 {
		retries = DefaultRetries
	}
	return &Client{
		developerToken:  cfg.DeveloperToken,
		tokenSource:     cfg.TokenSource,
		customerID:      cfg.CustomerID,
		loginCustomerID: cfg.LoginCustomerID,
		retries:         retries,
		verbose:         cfg.Verbose,
		http:            &http.Client{Timeout: defaultTimeout},
	}
}

// Do executes an HTTP request with Google Ads auth headers and retry logic.
// The request body (if any) is buffered internally so it can be replayed on retry.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Snapshot the body so we can replay it on retry.
	var bodySnapshot []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodySnapshot, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		_ = req.Body.Close()
	}

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			if c.verbose {
				fmt.Fprintf(errWriter, "[DEBUG] Retrying in %s (attempt %d/%d)...\n",
					backoff, attempt, c.retries)
			} else {
				fmt.Fprintf(errWriter, "Retrying in %s (attempt %d/%d)...\n",
					backoff, attempt, c.retries)
			}
			time.Sleep(backoff)
		}

		// Re-attach body for each attempt.
		if len(bodySnapshot) > 0 {
			req.Body = io.NopCloser(bytes.NewReader(bodySnapshot))
			req.ContentLength = int64(len(bodySnapshot))
		}

		// Inject auth headers (fresh token each attempt to handle mid-retry expiry).
		if err := c.injectAuthHeaders(req); err != nil {
			return nil, err
		}

		if c.verbose {
			logRequest(req, bodySnapshot)
		}

		start := time.Now()
		resp, err := c.http.Do(req)
		elapsed := time.Since(start)

		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			continue // network error — retry
		}

		if c.verbose {
			logResponse(resp, elapsed)
		}

		// Determine if this status is retryable.
		if isRetryable(resp.StatusCode) && attempt < c.retries {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("transient error: HTTP %d", resp.StatusCode)
			continue
		}

		// Non-2xx is an API error.
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := ParseAPIError(resp)
			_ = resp.Body.Close()
			return nil, apiErr
		}

		return resp, nil
	}

	return nil, lastErr
}

// Get executes a GET request to the given path.
// If path starts with "/", it is appended to BaseURL.
func (c *Client) Get(path string, headers map[string]string) (*http.Response, error) {
	url := resolveURL(path)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building GET request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.Do(req)
}

// Post executes a POST request with the given JSON body.
func (c *Client) Post(path string, body []byte, headers map[string]string) (*http.Response, error) {
	url := resolveURL(path)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.Do(req)
}

// CustomerID returns the configured default customer ID.
func (c *Client) CustomerID() string {
	return c.customerID
}

// injectAuthHeaders adds the developer-token and Authorization headers.
// It auto-refreshes the access token if needed.
func (c *Client) injectAuthHeaders(req *http.Request) error {
	if c.developerToken != "" {
		req.Header.Set("developer-token", c.developerToken)
	}
	if c.loginCustomerID != "" {
		req.Header.Set("login-customer-id", c.loginCustomerID)
	}
	if c.tokenSource != nil {
		token, err := c.tokenSource.AccessToken()
		if err != nil {
			return fmt.Errorf("getting access token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

// resolveURL prepends BaseURL if path starts with "/".
func resolveURL(path string) string {
	if strings.HasPrefix(path, "/") {
		return BaseURL + path
	}
	return path
}

// isRetryable returns true for status codes that warrant a retry.
func isRetryable(status int) bool {
	return status == 429 || status == 500 || status == 503
}

// logRequest writes request details to stderr (with token redaction).
func logRequest(req *http.Request, body []byte) {
	fmt.Fprintf(errWriter, "[DEBUG] %s %s\n", req.Method, req.URL.String())
	for k, vs := range req.Header {
		for _, v := range vs {
			fmt.Fprintf(errWriter, "[DEBUG] Request header: %s=%s\n", k, redact(k, v))
		}
	}
	if len(body) > 0 {
		fmt.Fprintf(errWriter, "[DEBUG] Request body: %s\n", body)
	}
}

// logResponse writes response details to stderr.
func logResponse(resp *http.Response, elapsed time.Duration) {
	fmt.Fprintf(errWriter, "[DEBUG] Response: %s (%dms)\n",
		resp.Status, elapsed.Milliseconds())
}

// redact returns a masked value for sensitive headers.
func redact(header, value string) string {
	lower := strings.ToLower(header)
	if lower == "authorization" || lower == "developer-token" {
		if len(value) <= 8 {
			return "***"
		}
		return value[:4] + "***"
	}
	return value
}
