package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCredentialsIsExpired(t *testing.T) {
	expired := &Credentials{
		AccessToken: "tok",
		Expiry:      time.Now().Add(-time.Hour),
	}
	if !expired.IsExpired() {
		t.Error("expected expired token to be detected as expired")
	}

	valid := &Credentials{
		AccessToken: "tok",
		Expiry:      time.Now().Add(2 * time.Hour),
	}
	if valid.IsExpired() {
		t.Error("expected valid token to not be expired")
	}

	// Empty access token should always be considered expired.
	empty := &Credentials{}
	if !empty.IsExpired() {
		t.Error("expected empty access token to be considered expired")
	}

	// Token expiring within the 30s buffer should be considered expired.
	almostExpired := &Credentials{
		AccessToken: "tok",
		Expiry:      time.Now().Add(10 * time.Second),
	}
	if !almostExpired.IsExpired() {
		t.Error("expected token expiring in 10s to be considered expired (within 30s buffer)")
	}
}

func TestCredentialsSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	creds := &Credentials{
		RefreshToken: "rtoken-abc123",
		AccessToken:  "atoken-xyz789",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
	}

	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	// Verify file exists and has correct permissions.
	path, err := CredentialsPath()
	if err != nil {
		t.Fatalf("CredentialsPath: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected 0600 permissions, got %04o", info.Mode().Perm())
	}

	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadCredentials returned nil")
	}
	if loaded.RefreshToken != creds.RefreshToken {
		t.Errorf("RefreshToken: got %q, want %q", loaded.RefreshToken, creds.RefreshToken)
	}
	if loaded.AccessToken != creds.AccessToken {
		t.Errorf("AccessToken: got %q, want %q", loaded.AccessToken, creds.AccessToken)
	}
	if loaded.TokenType != creds.TokenType {
		t.Errorf("TokenType: got %q, want %q", loaded.TokenType, creds.TokenType)
	}
}

func TestLoadCredentialsNotExist(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// No credentials file — should return nil, nil.
	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if creds != nil {
		t.Errorf("expected nil when credentials file doesn't exist, got %+v", creds)
	}
}

func TestDeleteCredentials(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	creds := &Credentials{
		RefreshToken: "rtoken",
		AccessToken:  "atoken",
		Expiry:       time.Now().Add(time.Hour),
		CreatedAt:    time.Now(),
	}
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	if err := DeleteCredentials(); err != nil {
		t.Fatalf("DeleteCredentials: %v", err)
	}

	// LoadCredentials after delete should return nil.
	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials after delete: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil after deletion")
	}
}

func TestDeleteCredentialsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Deleting when no file exists should not error.
	if err := DeleteCredentials(); err != nil {
		t.Errorf("DeleteCredentials on non-existent file: %v", err)
	}
}

func TestCredentialsPathLocation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	path, err := CredentialsPath()
	if err != nil {
		t.Fatalf("CredentialsPath: %v", err)
	}
	expectedDir := filepath.Join(tmp, ".gads")
	expectedFile := filepath.Join(expectedDir, "credentials.json")
	if path != expectedFile {
		t.Errorf("CredentialsPath: got %q, want %q", path, expectedFile)
	}
}

func TestSaveCredentialsCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// The ~/.gads directory does not exist yet.
	gadsDir := filepath.Join(tmp, ".gads")
	if _, err := os.Stat(gadsDir); !os.IsNotExist(err) {
		t.Fatal("expected .gads dir to not exist before save")
	}

	creds := &Credentials{
		RefreshToken: "tok",
		Expiry:       time.Now().Add(time.Hour),
	}
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	if _, err := os.Stat(gadsDir); err != nil {
		t.Errorf("expected .gads dir to be created: %v", err)
	}
}
