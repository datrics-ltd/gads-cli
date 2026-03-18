package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// mockTokenSource returns a fixed access token without any network calls.
type mockTokenSource struct {
	token string
}

func (m *mockTokenSource) AccessToken() (string, error) {
	return m.token, nil
}

func TestClientInjectsAuthHeaders(t *testing.T) {
	var devToken, authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		devToken = r.Header.Get("developer-token")
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(Config{
		DeveloperToken: "dev-tok-123",
		TokenSource:    &mockTokenSource{token: "access-tok-456"},
		Retries:        1,
	})

	_, err := client.Get(srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if devToken != "dev-tok-123" {
		t.Errorf("developer-token: got %q, want %q", devToken, "dev-tok-123")
	}
	if authHeader != "Bearer access-tok-456" {
		t.Errorf("Authorization: got %q, want %q", authHeader, "Bearer access-tok-456")
	}
}

func TestClientPostBody(t *testing.T) {
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(Config{
		DeveloperToken: "tok",
		TokenSource:    &mockTokenSource{token: "acc"},
		Retries:        1,
	})

	body := []byte(`{"query": "SELECT *"}`)
	_, err := client.Post(srv.URL+"/test", body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody != string(body) {
		t.Errorf("body: got %q, want %q", receivedBody, body)
	}
}

func TestClientCustomHeaders(t *testing.T) {
	var customHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		customHeader = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(Config{
		DeveloperToken: "tok",
		TokenSource:    &mockTokenSource{token: "acc"},
		Retries:        1,
	})

	_, err := client.Get(srv.URL+"/test", map[string]string{"X-Custom": "custom-value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if customHeader != "custom-value" {
		t.Errorf("X-Custom header: got %q, want %q", customHeader, "custom-value")
	}
}

func TestClientRetriesOn429(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode (involves 1s sleep)")
	}

	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(Config{
		DeveloperToken: "tok",
		TokenSource:    &mockTokenSource{token: "acc"},
		Retries:        1, // 1 retry = 2 total attempts
	})

	_, err := client.Get(srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestClientRetriesOn500(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode (involves 1s sleep)")
	}

	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": {"code": 500, "message": "server error"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(Config{
		DeveloperToken: "tok",
		TokenSource:    &mockTokenSource{token: "acc"},
		Retries:        1,
	})

	_, err := client.Get(srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestClientNoRetryOn400(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"code": 400, "message": "bad request"}}`))
	}))
	defer srv.Close()

	client := NewClient(Config{
		DeveloperToken: "tok",
		TokenSource:    &mockTokenSource{token: "acc"},
		Retries:        3,
	})

	_, err := client.Get(srv.URL+"/test", nil)
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (no retry on 400), got %d", attempts)
	}
}

func TestClientRetriesBodyReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode (involves 1s sleep)")
	}

	var bodies []string
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(Config{
		DeveloperToken: "tok",
		TokenSource:    &mockTokenSource{token: "acc"},
		Retries:        1,
	})

	payload := []byte(`{"key": "value"}`)
	_, err := client.Post(srv.URL+"/test", payload, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, body := range bodies {
		if body != string(payload) {
			t.Errorf("attempt %d: body mismatch: got %q, want %q", i+1, body, payload)
		}
	}
}

func TestResolveURL(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/v18/customers/123/campaigns", BaseURL + "/v18/customers/123/campaigns"},
		{"https://example.com/full", "https://example.com/full"},
		{"http://localhost:9999/test", "http://localhost:9999/test"},
	}
	for _, c := range cases {
		got := resolveURL(c.path)
		if got != c.want {
			t.Errorf("resolveURL(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestIsRetryable(t *testing.T) {
	retryable := []int{429, 500, 503}
	notRetryable := []int{200, 201, 400, 401, 403, 404}

	for _, s := range retryable {
		if !isRetryable(s) {
			t.Errorf("isRetryable(%d) = false, want true", s)
		}
	}
	for _, s := range notRetryable {
		if isRetryable(s) {
			t.Errorf("isRetryable(%d) = true, want false", s)
		}
	}
}

func TestRedact(t *testing.T) {
	cases := []struct {
		header string
		value  string
		want   string
	}{
		{"Authorization", "Bearer ya29.verylongtoken", "Bear***"},
		{"developer-token", "AbCdEf123456", "AbCd***"},
		{"developer-token", "short", "***"},
		{"X-Custom", "plainvalue", "plainvalue"},
		{"Content-Type", "application/json", "application/json"},
	}
	for _, c := range cases {
		got := redact(c.header, c.value)
		if got != c.want {
			t.Errorf("redact(%q, %q) = %q, want %q", c.header, c.value, got, c.want)
		}
	}
}

func TestClientErrorOn4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"code": 401, "message": "Unauthorized"}}`))
	}))
	defer srv.Close()

	client := NewClient(Config{
		DeveloperToken: "tok",
		TokenSource:    &mockTokenSource{token: "acc"},
		Retries:        1,
	})

	_, err := client.Get(srv.URL+"/test", nil)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus: got %d, want %d", apiErr.HTTPStatus, http.StatusUnauthorized)
	}
	if !strings.Contains(apiErr.Message, "authentication") {
		t.Errorf("expected 'authentication' in error message, got: %q", apiErr.Message)
	}
}
