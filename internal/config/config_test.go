package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
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

// newTestCmd returns a minimal Cobra command with a --customer-id persistent flag for profile tests.
func newTestCmd() *cobra.Command {
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().String("customer-id", "", "")
	root.PersistentFlags().String("output", "table", "")
	return root
}

func TestApplyProfileNoOp(t *testing.T) {
	viper.Reset()
	// No profile set — ApplyProfile should be a no-op and return nil.
	cmd := newTestCmd()
	if err := ApplyProfile(cmd); err != nil {
		t.Fatalf("expected no error with empty profile, got: %v", err)
	}
}

func TestApplyProfileOverridesBaseConfig(t *testing.T) {
	viper.Reset()
	// Set base config customer_id.
	viper.Set("customer_id", "base-customer")
	// Set profile data in viper.
	viper.Set("profiles.client-a.customer_id", "profile-customer")
	viper.Set("profile", "client-a")

	cmd := newTestCmd()
	if err := ApplyProfile(cmd); err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	if got := viper.GetString("customer_id"); got != "profile-customer" {
		t.Errorf("customer_id: got %q, want %q", got, "profile-customer")
	}
}

func TestApplyProfileFlagTakesPrecedence(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.client-a.customer_id", "profile-customer")
	viper.Set("profile", "client-a")

	cmd := newTestCmd()
	// Simulate user explicitly passing --customer-id on the command line.
	if err := cmd.PersistentFlags().Set("customer-id", "flag-customer"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	// Bind the pflag to viper so viper sees the flag value.
	_ = viper.BindPFlag("customer_id", cmd.PersistentFlags().Lookup("customer-id"))

	if err := ApplyProfile(cmd); err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	// Flag value must win over profile.
	if got := viper.GetString("customer_id"); got != "flag-customer" {
		t.Errorf("customer_id: got %q, want %q (flag should win)", got, "flag-customer")
	}
}

func TestApplyProfileEnvVarTakesPrecedence(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.client-a.customer_id", "profile-customer")
	viper.Set("profile", "client-a")
	t.Setenv("GADS_CUSTOMER_ID", "env-customer")

	cmd := newTestCmd()
	if err := ApplyProfile(cmd); err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	// Env var should win — ApplyProfile skips keys whose env var is set.
	// After ApplyProfile, customer_id should NOT be the profile value because we skipped it.
	// The actual resolved value comes from viper's env binding (done elsewhere in Init),
	// but here we verify ApplyProfile did NOT call viper.Set("customer_id", "profile-customer").
	if got := viper.GetString("customer_id"); got == "profile-customer" {
		t.Errorf("profile value should not override env var")
	}
}

func TestApplyProfileNotFound(t *testing.T) {
	viper.Reset()
	viper.Set("profile", "nonexistent-profile")

	cmd := newTestCmd()
	err := ApplyProfile(cmd)
	if err == nil {
		t.Fatal("expected error for missing profile, got nil")
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
