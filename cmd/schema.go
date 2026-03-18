package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/datrics-ltd/gads-cli/internal/schema"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var schemaCmd = &cobra.Command{
	Use:   "schema [resource]",
	Short: "Show API field schema for a resource",
	Long: `Show all available fields for a Google Ads API resource.

Without a resource argument, lists all known resources in the embedded schema.
Use --selectable or --filterable to filter which fields are shown.
Use --live to fetch current field data directly from GoogleAdsFieldService.`,
	Example: `  gads schema                      # List all resources
  gads schema campaign              # All fields for campaign
  gads schema campaign --selectable # Only GAQL-selectable fields
  gads schema campaign --filterable # Only GAQL-filterable fields
  gads schema campaign --live       # Fetch from live GoogleAdsFieldService`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSchema,
}

func init() {
	schemaCmd.Flags().Bool("selectable", false, "Only show fields usable in GAQL SELECT")
	schemaCmd.Flags().Bool("filterable", false, "Only show fields usable in GAQL WHERE")
	schemaCmd.Flags().Bool("live", false, "Fetch field data from GoogleAdsFieldService instead of embedded schema")
	rootCmd.AddCommand(schemaCmd)
}

func runSchema(cmd *cobra.Command, args []string) error {
	selectable, _ := cmd.Flags().GetBool("selectable")
	filterable, _ := cmd.Flags().GetBool("filterable")
	live, _ := cmd.Flags().GetBool("live")

	// No resource argument: list available resources
	if len(args) == 0 {
		return runSchemaListResources()
	}

	resource := strings.ToLower(strings.TrimSpace(args[0]))

	var fields []schema.Field
	var err error

	if live {
		fields, err = fetchLiveFields(resource)
		if err != nil {
			return err
		}
		if len(fields) == 0 {
			return fmt.Errorf("no fields returned from GoogleAdsFieldService for resource %q", resource)
		}
	} else {
		fields = schema.GetFields(resource)
		if len(fields) == 0 {
			return fmt.Errorf("no schema found for resource %q — try `gads schema` to list available resources", resource)
		}
	}

	if selectable {
		fields = schemaFilterFields(fields, func(f schema.Field) bool { return f.Selectable })
	} else if filterable {
		fields = schemaFilterFields(fields, func(f schema.Field) bool { return f.Filterable })
	}

	if len(fields) == 0 {
		fmt.Fprintf(os.Stderr, "No fields match the specified filter for resource %q.\n", resource)
		return nil
	}

	return printSchemaFields(fields)
}

func runSchemaListResources() error {
	resources := schema.ListResources()
	if len(resources) == 0 {
		fmt.Fprintln(os.Stderr, "No resources found in embedded schema.")
		return nil
	}

	fmtStr := viper.GetString("output")

	headers := []string{"resource"}
	rows := make([]map[string]interface{}, len(resources))
	for i, r := range resources {
		rows[i] = map[string]interface{}{"resource": r}
	}

	if fmtStr == "table" && !viper.GetBool("quiet") {
		fmt.Fprintf(os.Stderr, "Embedded schema: API %s, generated %s\n\n",
			schema.APIVersion(), schema.GeneratedAt())
	}

	return renderOutput(headers, rows, "", "")
}

func printSchemaFields(fields []schema.Field) error {
	headers := []string{"field", "type", "category", "selectable", "filterable", "sortable", "description"}
	rows := make([]map[string]interface{}, len(fields))
	for i, f := range fields {
		row := map[string]interface{}{
			"field":       f.Name,
			"type":        f.DataType,
			"category":    f.Category,
			"selectable":  f.Selectable,
			"filterable":  f.Filterable,
			"sortable":    f.Sortable,
			"description": f.Description,
		}
		if len(f.EnumValues) > 0 {
			row["enum_values"] = strings.Join(f.EnumValues, "|")
		}
		rows[i] = row
	}

	return renderOutput(headers, rows, "", "")
}

func schemaFilterFields(fields []schema.Field, keep func(schema.Field) bool) []schema.Field {
	var out []schema.Field
	for _, f := range fields {
		if keep(f) {
			out = append(out, f)
		}
	}
	return out
}

// fetchLiveFields queries GoogleAdsFieldService for field metadata for the given resource.
func fetchLiveFields(resource string) ([]schema.Field, error) {
	client, err := buildAPIClient()
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(
		"SELECT name, category, data_type, selectable, filterable, sortable, is_repeated, description, enum_values "+
			"WHERE name LIKE '%s.%%'",
		resource,
	)

	reqBody, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	resp, err := client.Post("/v18/googleAdsFields:search", reqBody, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching live schema: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result struct {
		Results []struct {
			Name        string   `json:"name"`
			Category    string   `json:"category"`
			DataType    string   `json:"dataType"`
			Selectable  bool     `json:"selectable"`
			Filterable  bool     `json:"filterable"`
			Sortable    bool     `json:"sortable"`
			IsRepeated  bool     `json:"isRepeated"`
			Description string   `json:"description"`
			EnumValues  []string `json:"enumValues"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing GoogleAdsFieldService response: %w", err)
	}

	fields := make([]schema.Field, len(result.Results))
	for i, r := range result.Results {
		fields[i] = schema.Field{
			Name:        r.Name,
			DataType:    r.DataType,
			Category:    r.Category,
			Resource:    resource,
			Selectable:  r.Selectable,
			Filterable:  r.Filterable,
			Sortable:    r.Sortable,
			IsRepeated:  r.IsRepeated,
			Description: r.Description,
			EnumValues:  r.EnumValues,
		}
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})

	return fields, nil
}
