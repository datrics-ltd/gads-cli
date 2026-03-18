package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestConfigFileReadWrite(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	content := "developer_token: test-dev-token\ndefault_customer_id: 123-456-7890\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	v := viper.New()
	v.SetConfigFile(cfgFile)
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("reading config: %v", err)
	}

	if got := v.GetString("developer_token"); got != "test-dev-token" {
		t.Errorf("developer_token: got %q, want %q", got, "test-dev-token")
	}
	if got := v.GetString("default_customer_id"); got != "123-456-7890" {
		t.Errorf("default_customer_id: got %q, want %q", got, "123-456-7890")
	}
}

func TestConfigEnvVarOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(cfgFile, []byte("developer_token: from-file\n"), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	t.Setenv("GADS_DEVELOPER_TOKEN", "from-env")

	v := viper.New()
	v.SetConfigFile(cfgFile)
	v.SetEnvPrefix("GADS")
	v.AutomaticEnv()
	_ = v.BindEnv("developer_token", "GADS_DEVELOPER_TOKEN")

	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("reading config: %v", err)
	}

	// Env var should take precedence over file value.
	if got := v.GetString("developer_token"); got != "from-env" {
		t.Errorf("expected env var to override file: got %q, want %q", got, "from-env")
	}
}

func TestConfigMissingFileIsOK(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "nonexistent.yaml")

	v := viper.New()
	v.SetConfigFile(cfgFile)
	err := v.ReadInConfig()
	// Missing file should return an error but Init() handles ConfigFileNotFoundError gracefully.
	// When using SetConfigFile (explicit path), viper returns an os.PathError, not ConfigFileNotFoundError.
	// We just verify the read attempt doesn't panic and the config is otherwise usable.
	_ = err // either nil or os.PathError — both are acceptable

	// Values should default to zero values.
	if got := v.GetString("developer_token"); got != "" {
		t.Errorf("expected empty developer_token for missing file, got %q", got)
	}
}

func TestConfigPrecedenceOrder(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	// Write file with one value.
	if err := os.WriteFile(cfgFile, []byte("output: table\n"), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	// Set env var to override.
	t.Setenv("GADS_OUTPUT", "json")

	v := viper.New()
	v.SetConfigFile(cfgFile)
	v.SetEnvPrefix("GADS")
	v.AutomaticEnv()
	_ = v.BindEnv("output", "GADS_OUTPUT")

	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("reading config: %v", err)
	}

	// Env > file: output should be "json" not "table".
	if got := v.GetString("output"); got != "json" {
		t.Errorf("env var should override file: got %q, want %q", got, "json")
	}

	// Programmatic Set > env: highest precedence.
	v.Set("output", "csv")
	if got := v.GetString("output"); got != "csv" {
		t.Errorf("Set() should override env: got %q, want %q", got, "csv")
	}
}

func TestConfigFilePathIsUnderGadsDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	path, err := ConfigFilePath()
	if err != nil {
		t.Fatalf("ConfigFilePath: %v", err)
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("expected path to end with config.yaml, got: %s", path)
	}
	expectedDir := filepath.Join(tmp, ".gads")
	if filepath.Dir(path) != expectedDir {
		t.Errorf("expected config dir %q, got %q", expectedDir, filepath.Dir(path))
	}
}

func TestConfigDirAutoCreate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Ensure .gads does not exist yet.
	gadsDir := filepath.Join(tmp, ".gads")
	if _, err := os.Stat(gadsDir); !os.IsNotExist(err) {
		t.Fatal("expected .gads dir to not exist before Init()")
	}

	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if _, err := os.Stat(gadsDir); err != nil {
		t.Errorf("expected .gads dir to be created by Init(), got: %v", err)
	}
}

func TestConfigMultipleKeys(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	content := `developer_token: abc123
client_id: my-client-id
client_secret: my-secret
default_customer_id: 999-888-7777
default_output: csv
`
	if err := os.WriteFile(cfgFile, []byte(content), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	v := viper.New()
	v.SetConfigFile(cfgFile)
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("reading config: %v", err)
	}

	checks := map[string]string{
		"developer_token":    "abc123",
		"client_id":          "my-client-id",
		"client_secret":      "my-secret",
		"default_customer_id": "999-888-7777",
		"default_output":     "csv",
	}
	for key, want := range checks {
		if got := v.GetString(key); got != want {
			t.Errorf("%s: got %q, want %q", key, got, want)
		}
	}
}
