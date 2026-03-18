package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/datrics-ltd/gads-cli/internal/api"
	"github.com/datrics-ltd/gads-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var apiCmd = &cobra.Command{
	Use:   "api <METHOD> <path>",
	Short: "Make authenticated requests to the Google Ads API",
	Long: `Make authenticated HTTP requests directly to the Google Ads API.

Auth headers (developer-token and Authorization: Bearer) are injected automatically.
{customer_id} in the path is replaced with the configured default customer ID.
If the path starts with /, https://googleads.googleapis.com is prepended.

Examples:
  # GET a resource
  gads api GET /v18/customers/{customer_id}/campaigns/789

  # POST a mutation with inline body
  gads api POST /v18/customers/{customer_id}/campaigns:mutate -d '{"operations": [...]}'

  # POST with body from file
  gads api POST /v18/customers/{customer_id}/adGroups:mutate -d @payload.json

  # POST with body from stdin
  cat mutation.json | gads api POST /v18/customers/{customer_id}/campaigns:mutate

  # Dry run — show request without sending
  gads api POST /v18/customers/{customer_id}/campaigns:mutate -d @payload.json --dry-run

  # Custom headers
  gads api GET /v18/customers/{customer_id}/campaigns -H "x-custom: value"`,
	Args: cobra.ExactArgs(2),
	RunE: runAPICommand,
}

func runAPICommand(cmd *cobra.Command, args []string) error {
	method := strings.ToUpper(args[0])
	if method != "GET" && method != "POST" {
		return fmt.Errorf("unsupported method %q — use GET or POST", method)
	}

	path := args[1]

	// Replace {customer_id} with the configured customer ID (dashes stripped).
	customerID := viper.GetString("customer_id")
	normalizedID := strings.ReplaceAll(customerID, "-", "")
	path = strings.ReplaceAll(path, "{customer_id}", normalizedID)

	// Resolve full URL (BaseURL is prepended when path starts with /).
	resolvedURL := path
	if strings.HasPrefix(path, "/") {
		resolvedURL = api.BaseURL + path
	}

	// Read request body.
	body, err := readBody(cmd, method)
	if err != nil {
		return err
	}

	// Parse custom headers (-H "key: value").
	customHeaders, err := parseHeaders(cmd)
	if err != nil {
		return err
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		return printDryRun(method, resolvedURL, body, customHeaders)
	}

	client, err := buildAPIClient()
	if err != nil {
		return err
	}

	// Execute the request.
	var rawBody []byte
	switch method {
	case "GET":
		r, err := client.Get(path, customHeaders)
		if err != nil {
			return err
		}
		defer r.Body.Close()
		rawBody, err = io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}
	case "POST":
		r, err := client.Post(path, body, customHeaders)
		if err != nil {
			return err
		}
		defer r.Body.Close()
		rawBody, err = io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}
	}

	rawFlag, _ := cmd.Flags().GetBool("raw")
	if rawFlag {
		_, err = os.Stdout.Write(rawBody)
		return err
	}

	// If --output was explicitly set to json or csv, try to route through a formatter.
	fmtStr := viper.GetString("output")
	outputExplicit := cmd.Root().PersistentFlags().Changed("output")
	if outputExplicit && fmtStr != "table" {
		if err := formatAPIResponse(rawBody, fmtStr, customerID); err == nil {
			return nil
		}
		// Fall through to pretty-print if structured formatting fails.
	}

	// Default: pretty-print JSON.
	return prettyPrintJSON(rawBody)
}

// readBody reads the request body from -d flag, @file, or stdin (POST only).
func readBody(cmd *cobra.Command, method string) ([]byte, error) {
	data, _ := cmd.Flags().GetString("data")
	if data != "" {
		if strings.HasPrefix(data, "@") {
			filename := data[1:]
			b, err := os.ReadFile(filename)
			if err != nil {
				return nil, fmt.Errorf("reading body file %q: %w", filename, err)
			}
			return b, nil
		}
		return []byte(data), nil
	}

	// For POST with no -d flag, read from stdin if it's not a terminal.
	if method == "POST" {
		stat, err := os.Stdin.Stat()
		if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, fmt.Errorf("reading stdin: %w", err)
			}
			return b, nil
		}
	}
	return nil, nil
}

// parseHeaders parses -H "key: value" flags into a map.
func parseHeaders(cmd *cobra.Command) (map[string]string, error) {
	headerStrs, _ := cmd.Flags().GetStringArray("header")
	if len(headerStrs) == 0 {
		return nil, nil
	}
	headers := make(map[string]string, len(headerStrs))
	for _, h := range headerStrs {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header %q — expected \"Key: Value\"", h)
		}
		headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return headers, nil
}

// printDryRun shows what would be sent without actually sending it.
func printDryRun(method, url string, body []byte, headers map[string]string) error {
	fmt.Printf("[DRY RUN] %s %s\n", method, url)
	fmt.Println("[DRY RUN] Headers:")
	fmt.Println("[DRY RUN]   developer-token: ***")
	fmt.Println("[DRY RUN]   Authorization: Bearer ***")
	if method == "POST" {
		fmt.Println("[DRY RUN]   Content-Type: application/json")
	}
	for k, v := range headers {
		fmt.Printf("[DRY RUN]   %s: %s\n", k, v)
	}
	if len(body) > 0 {
		// Pretty-print the body if it's valid JSON.
		var parsed interface{}
		if err := json.Unmarshal(body, &parsed); err == nil {
			pretty, _ := json.MarshalIndent(parsed, "[DRY RUN]   ", "  ")
			fmt.Printf("[DRY RUN] Body:\n[DRY RUN]   %s\n", pretty)
		} else {
			fmt.Printf("[DRY RUN] Body: %s\n", body)
		}
	} else {
		fmt.Println("[DRY RUN] Body: (empty)")
	}
	return nil
}

// formatAPIResponse attempts to parse rawBody as a JSON array of objects and
// route through the requested output formatter.
func formatAPIResponse(rawBody []byte, fmtStr, customerID string) error {
	var rows []map[string]interface{}
	if err := json.Unmarshal(rawBody, &rows); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	// Derive headers from first row's keys.
	headers := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		headers = append(headers, k)
	}

	format, err := output.ParseFormat(fmtStr)
	if err != nil {
		return err
	}

	opts := output.Options{
		NoColor: viper.GetBool("no_color"),
		Compact: viper.GetBool("compact"),
		BOM:     viper.GetBool("bom"),
		Verbose: viper.GetBool("verbose"),
		Meta: output.Meta{
			CustomerID: customerID,
			Rows:       len(rows),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		},
	}

	formatter := output.New(format, opts)
	return formatter.Format(os.Stdout, headers, rows)
}

// prettyPrintJSON writes rawBody to stdout as indented JSON.
// If rawBody is not valid JSON it is printed as-is.
func prettyPrintJSON(rawBody []byte) error {
	var parsed interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		_, err = os.Stdout.Write(rawBody)
		return err
	}
	pretty, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		_, err = os.Stdout.Write(rawBody)
		return err
	}
	fmt.Println(string(pretty))
	return nil
}

func init() {
	apiCmd.Flags().StringP("data", "d", "", `Request body: inline JSON, @file.json for file, or omit to read from stdin (POST only)`)
	apiCmd.Flags().StringArrayP("header", "H", nil, `Custom header "Key: Value" (repeatable)`)
	apiCmd.Flags().Bool("dry-run", false, "Show the full request without sending it")
	apiCmd.Flags().Bool("raw", false, "Print raw response without JSON pretty-printing")
	rootCmd.AddCommand(apiCmd)
}
