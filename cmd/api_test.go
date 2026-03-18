package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestAPICmd returns a cobra.Command with the same flags as apiCmd for testing.
func newTestAPICmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().StringP("data", "d", "", "Request body")
	cmd.Flags().StringArrayP("header", "H", nil, "Custom header")
	cmd.Flags().Bool("dry-run", false, "Dry run")
	cmd.Flags().Bool("raw", false, "Raw output")
	return cmd
}

func TestParseHeaders(t *testing.T) {
	cmd := newTestAPICmd()
	if err := cmd.Flags().Set("header", "X-Custom: custom-value"); err != nil {
		t.Fatalf("setting header flag: %v", err)
	}
	if err := cmd.Flags().Set("header", "X-Other: other-value"); err != nil {
		t.Fatalf("setting header flag: %v", err)
	}

	headers, err := parseHeaders(cmd)
	if err != nil {
		t.Fatalf("parseHeaders: %v", err)
	}
	if headers["X-Custom"] != "custom-value" {
		t.Errorf("X-Custom: got %q, want %q", headers["X-Custom"], "custom-value")
	}
	if headers["X-Other"] != "other-value" {
		t.Errorf("X-Other: got %q, want %q", headers["X-Other"], "other-value")
	}
}

func TestParseHeadersEmpty(t *testing.T) {
	cmd := newTestAPICmd()

	headers, err := parseHeaders(cmd)
	if err != nil {
		t.Fatalf("parseHeaders: %v", err)
	}
	if headers != nil {
		t.Errorf("expected nil headers when no -H flags set, got %v", headers)
	}
}

func TestParseHeadersInvalid(t *testing.T) {
	cmd := newTestAPICmd()
	if err := cmd.Flags().Set("header", "InvalidNoColon"); err != nil {
		t.Fatalf("setting header flag: %v", err)
	}

	_, err := parseHeaders(cmd)
	if err == nil {
		t.Error("expected error for header without colon")
	}
}

func TestParseHeadersWhitespaceTrimmed(t *testing.T) {
	cmd := newTestAPICmd()
	if err := cmd.Flags().Set("header", "  X-Key  :  some value  "); err != nil {
		t.Fatalf("setting header flag: %v", err)
	}

	headers, err := parseHeaders(cmd)
	if err != nil {
		t.Fatalf("parseHeaders: %v", err)
	}
	if headers["X-Key"] != "some value" {
		t.Errorf("X-Key: got %q, want %q", headers["X-Key"], "some value")
	}
}

func TestPrettyPrintJSONValid(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := prettyPrintJSON([]byte(`{"key":"value","num":42}`))

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("prettyPrintJSON: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"key": "value"`) {
		t.Errorf("expected pretty-printed JSON, got:\n%s", out)
	}
	if !strings.Contains(out, `"num": 42`) {
		t.Errorf("expected number field in output, got:\n%s", out)
	}
}

func TestPrettyPrintJSONInvalid(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	raw := []byte("not valid json")
	err := prettyPrintJSON(raw)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("prettyPrintJSON: %v", err)
	}
	// Raw bytes should be written as-is.
	if !strings.Contains(buf.String(), "not valid json") {
		t.Errorf("expected raw bytes in output, got: %q", buf.String())
	}
}

func TestPrintDryRunGET(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printDryRun("GET", "https://googleads.googleapis.com/v18/customers/123/campaigns", nil, nil)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("printDryRun: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[DRY RUN] GET https://googleads.googleapis.com/v18/customers/123/campaigns") {
		t.Errorf("expected dry-run method/URL line, got:\n%s", out)
	}
	if !strings.Contains(out, "developer-token: ***") {
		t.Errorf("expected redacted developer-token, got:\n%s", out)
	}
	if !strings.Contains(out, "Authorization: Bearer ***") {
		t.Errorf("expected redacted Authorization header, got:\n%s", out)
	}
}

func TestPrintDryRunPOSTWithBody(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	body := []byte(`{"operations": [{"update": {"status": "PAUSED"}}]}`)
	err := printDryRun("POST", "https://googleads.googleapis.com/v18/customers/123/campaigns:mutate", body, nil)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("printDryRun: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[DRY RUN] POST") {
		t.Errorf("expected POST in dry-run output, got:\n%s", out)
	}
	if !strings.Contains(out, "Content-Type: application/json") {
		t.Errorf("expected Content-Type header in POST dry-run, got:\n%s", out)
	}
	if !strings.Contains(out, "operations") {
		t.Errorf("expected body content in dry-run output, got:\n%s", out)
	}
}

func TestPrintDryRunWithCustomHeaders(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printDryRun("GET", "https://example.com/test", nil, map[string]string{
		"X-Custom": "custom-value",
	})

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("printDryRun: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "X-Custom: custom-value") {
		t.Errorf("expected custom header in dry-run output, got:\n%s", out)
	}
}

func TestPathSubstitution(t *testing.T) {
	cases := []struct {
		path       string
		customerID string
		want       string
	}{
		{
			path:       "/v18/customers/{customer_id}/campaigns",
			customerID: "123-456-7890",
			want:       "/v18/customers/1234567890/campaigns",
		},
		{
			path:       "/v18/customers/{customer_id}/campaigns/{customer_id}",
			customerID: "111-222-3333",
			want:       "/v18/customers/1112223333/campaigns/1112223333",
		},
		{
			path:       "/v18/customers/explicit-id/campaigns",
			customerID: "123-456-7890",
			want:       "/v18/customers/explicit-id/campaigns",
		},
	}
	for _, c := range cases {
		// Reproduce the same normalization logic as runAPICommand.
		normalized := strings.ReplaceAll(c.customerID, "-", "")
		got := strings.ReplaceAll(c.path, "{customer_id}", normalized)
		if got != c.want {
			t.Errorf("path %q with customerID %q: got %q, want %q",
				c.path, c.customerID, got, c.want)
		}
	}
}

func TestReadBodyFromFlag(t *testing.T) {
	cmd := newTestAPICmd()
	if err := cmd.Flags().Set("data", `{"key": "value"}`); err != nil {
		t.Fatalf("setting data flag: %v", err)
	}

	body, err := readBody(cmd, "POST")
	if err != nil {
		t.Fatalf("readBody: %v", err)
	}
	if string(body) != `{"key": "value"}` {
		t.Errorf("body: got %q, want %q", body, `{"key": "value"}`)
	}
}

func TestReadBodyFromFile(t *testing.T) {
	tmp := t.TempDir()
	bodyFile := tmp + "/body.json"
	if err := os.WriteFile(bodyFile, []byte(`{"from": "file"}`), 0o644); err != nil {
		t.Fatalf("writing body file: %v", err)
	}

	cmd := newTestAPICmd()
	if err := cmd.Flags().Set("data", "@"+bodyFile); err != nil {
		t.Fatalf("setting data flag: %v", err)
	}

	body, err := readBody(cmd, "POST")
	if err != nil {
		t.Fatalf("readBody: %v", err)
	}
	if string(body) != `{"from": "file"}` {
		t.Errorf("body: got %q, want %q", body, `{"from": "file"}`)
	}
}

func TestReadBodyFromFileMissing(t *testing.T) {
	cmd := newTestAPICmd()
	if err := cmd.Flags().Set("data", "@/nonexistent/path/body.json"); err != nil {
		t.Fatalf("setting data flag: %v", err)
	}

	_, err := readBody(cmd, "POST")
	if err == nil {
		t.Error("expected error for missing body file")
	}
}
