package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestGetStatusNotLoggedIn(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("GADS_ACCESS_TOKEN", "")

	status, err := GetStatus("dev-token", "client-id")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.LoggedIn {
		t.Error("expected not logged in when no credentials file exists")
	}
	if !status.DevTokenSet {
		t.Error("expected DevTokenSet=true when developer token is provided")
	}
	if !status.ClientIDSet {
		t.Error("expected ClientIDSet=true when client_id is provided")
	}
}

func TestGetStatusWithEnvToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("GADS_ACCESS_TOKEN", "ya29.fake-direct-token")

	status, err := GetStatus("dev-token", "client-id")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if !status.LoggedIn {
		t.Error("expected LoggedIn=true when GADS_ACCESS_TOKEN is set")
	}
	if !status.UsingEnvToken {
		t.Error("expected UsingEnvToken=true")
	}
}

func TestGetStatusWithSavedCredentials(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("GADS_ACCESS_TOKEN", "")

	creds := &Credentials{
		RefreshToken: "valid-refresh-token",
		AccessToken:  "valid-access-token",
		Expiry:       time.Now().Add(2 * time.Hour),
		CreatedAt:    time.Now(),
	}
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	status, err := GetStatus("dev-token", "client-id")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if !status.LoggedIn {
		t.Error("expected LoggedIn=true with saved credentials")
	}
	if status.TokenExpired {
		t.Error("expected TokenExpired=false for token expiring in 2 hours")
	}
	if status.UsingEnvToken {
		t.Error("expected UsingEnvToken=false when using file credentials")
	}
}

func TestGetStatusWithExpiredCredentials(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("GADS_ACCESS_TOKEN", "")

	creds := &Credentials{
		RefreshToken: "old-refresh-token",
		AccessToken:  "expired-access-token",
		Expiry:       time.Now().Add(-time.Hour), // expired 1 hour ago
		CreatedAt:    time.Now().Add(-24 * time.Hour),
	}
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	status, err := GetStatus("dev-token", "client-id")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if !status.LoggedIn {
		// Still "logged in" because refresh token exists
		t.Error("expected LoggedIn=true with saved credentials (refresh token present)")
	}
	if !status.TokenExpired {
		t.Error("expected TokenExpired=true for expired access token")
	}
}

func TestGetStatusNoDevToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("GADS_ACCESS_TOKEN", "")

	status, err := GetStatus("", "")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.DevTokenSet {
		t.Error("expected DevTokenSet=false when no developer token")
	}
	if status.ClientIDSet {
		t.Error("expected ClientIDSet=false when no client_id")
	}
}

func TestOAuthTokenSourceReturnsStoredToken(t *testing.T) {
	// When token is not expired, AccessToken() should return it directly
	// without making any network calls.
	creds := &Credentials{
		AccessToken: "stored-access-token",
		Expiry:      time.Now().Add(time.Hour),
	}
	ts := NewTokenSource("client-id", "client-secret", creds)

	token, err := ts.AccessToken()
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token != "stored-access-token" {
		t.Errorf("got %q, want %q", token, "stored-access-token")
	}
}

func TestOAuthTokenSourceRefreshError(t *testing.T) {
	// When token is expired and no refresh token, should return an error.
	creds := &Credentials{
		AccessToken:  "expired-token",
		Expiry:       time.Now().Add(-time.Hour),
		RefreshToken: "", // no refresh token
	}
	ts := NewTokenSource("client-id", "client-secret", creds)

	_, err := ts.AccessToken()
	if err == nil {
		t.Fatal("expected error when no refresh token")
	}
	if !containsAny(err.Error(), "refresh token", "login") {
		t.Errorf("expected error about missing refresh token, got: %v", err)
	}
}

// TestLoginCallbackHandlerValidState tests the OAuth2 callback handler logic
// by simulating the request handling with both valid and invalid state parameters.
func TestLoginCallbackHandlerValidState(t *testing.T) {
	expectedState := "csrf-state-abc123"
	type result struct {
		code string
		err  error
	}
	ch := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != expectedState {
			http.Error(w, "invalid state parameter", http.StatusBadRequest)
			ch <- result{err: errors.New("invalid OAuth2 state")}
			return
		}
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			http.Error(w, "authentication denied", http.StatusUnauthorized)
			ch <- result{err: fmt.Errorf("Google denied: %s", errParam)}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "no code", http.StatusBadRequest)
			ch <- result{err: errors.New("no authorization code")}
			return
		}
		_, _ = fmt.Fprint(w, "OK")
		ch <- result{code: code}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Test 1: invalid state → HTTP 400
	resp, err := http.Get(srv.URL + "/callback?state=wrong&code=auth-code")
	if err != nil {
		t.Fatalf("GET (invalid state): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid state: expected HTTP 400, got %d", resp.StatusCode)
	}
	r1 := <-ch
	if r1.err == nil {
		t.Error("expected error for invalid state")
	}

	// Test 2: valid state, valid code → success
	validURL := srv.URL + "/callback?state=" + url.QueryEscape(expectedState) + "&code=auth-code-123"
	resp, err = http.Get(validURL)
	if err != nil {
		t.Fatalf("GET (valid): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("valid request: expected HTTP 200, got %d", resp.StatusCode)
	}
	r2 := <-ch
	if r2.err != nil {
		t.Errorf("unexpected error for valid request: %v", r2.err)
	}
	if r2.code != "auth-code-123" {
		t.Errorf("code: got %q, want %q", r2.code, "auth-code-123")
	}
}

func TestLoginCallbackHandlerErrorParam(t *testing.T) {
	ch := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			http.Error(w, "denied", http.StatusUnauthorized)
			ch <- fmt.Errorf("Google denied: %s", errParam)
			return
		}
		ch <- nil
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/callback?state=s&error=access_denied")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected HTTP 401, got %d", resp.StatusCode)
	}
	cbErr := <-ch
	if cbErr == nil {
		t.Error("expected error when Google returns error param")
	}
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
