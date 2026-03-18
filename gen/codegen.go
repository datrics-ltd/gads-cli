//go:build ignore

// codegen parses Google Ads API v18 proto definitions and writes
// internal/schema/data/schema.json, which is embedded in the binary
// at build time by the internal/schema package.
//
// Run after gen/proto_fetch.sh:
//
//	go run gen/codegen.go
//	go run gen/codegen.go v19   # for a different API version
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ---- types (must match internal/schema/metadata.go) ----------------------

// Field represents a single Google Ads API field entry.
type Field struct {
	Name        string   `json:"name"`
	DataType    string   `json:"data_type"`
	Category    string   `json:"category"`
	Resource    string   `json:"resource"`
	Selectable  bool     `json:"selectable"`
	Filterable  bool     `json:"filterable"`
	Sortable    bool     `json:"sortable"`
	IsRepeated  bool     `json:"is_repeated"`
	Description string   `json:"description"`
	EnumValues  []string `json:"enum_values,omitempty"`
}

// Schema is the top-level document written to schema.json.
type Schema struct {
	APIVersion  string  `json:"api_version"`
	GeneratedAt string  `json:"generated_at"`
	Fields      []Field `json:"fields"`
}

// ---- proto type → data type mapping -------------------------------------

var protoToDataType = map[string]string{
	"string":             "STRING",
	"int32":              "INT32",
	"int64":              "INT64",
	"uint32":             "UINT32",
	"uint64":             "UINT64",
	"double":             "DOUBLE",
	"float":              "FLOAT",
	"bool":               "BOOLEAN",
	"bytes":              "MESSAGE",
	"google.type.Date":   "DATE",
	"google.type.Money":  "MESSAGE",
}

func dataType(raw string) string {
	raw = strings.TrimSpace(raw)
	if dt, ok := protoToDataType[raw]; ok {
		return dt
	}
	// Enum types end with something like CampaignStatusEnum.CampaignStatus
	if strings.Contains(raw, "Enum.") || strings.HasSuffix(raw, "Enum") {
		return "ENUM"
	}
	return "MESSAGE"
}

// ---- proto parsing -------------------------------------------------------

// fieldLine matches proto field declarations, capturing:
//
//	group 1: last single-line comment before the field (may be empty)
//	group 2: optional "repeated" or "optional" modifier
//	group 3: proto type
//	group 4: field name
var fieldLine = regexp.MustCompile(
	`(?m)//\s*(.*?)\n\s*(repeated\s+|optional\s+)?(\S+)\s+(\w+)\s*=\s*\d+`,
)

// enumValues extracts enum value names from an enum block found in content.
var enumBlockRe = regexp.MustCompile(`(?s)enum\s+\w+\s*\{([^}]+)\}`)
var enumValueRe = regexp.MustCompile(`(?m)^\s+(\w+)\s*=\s*\d+`)

// enumsInFile returns a map from enum type suffix (e.g. "CampaignStatus") to
// its string values — parsed from enum proto files in the same directory tree.
func enumsFromDir(enumDir string) map[string][]string {
	result := map[string][]string{}
	entries, err := os.ReadDir(enumDir)
	if err != nil {
		return result
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".proto") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(enumDir, e.Name()))
		if err != nil {
			continue
		}
		for _, block := range enumBlockRe.FindAllStringSubmatch(string(data), -1) {
			body := block[1]
			// Determine enum name from "enum FooEnum { ..."
			nameRe := regexp.MustCompile(`enum\s+(\w+)\s*\{`)
			nameM := nameRe.FindStringSubmatch(block[0])
			if nameM == nil {
				continue
			}
			enumName := nameM[1]
			var vals []string
			for _, vm := range enumValueRe.FindAllStringSubmatch(body, -1) {
				vals = append(vals, vm[1])
			}
			result[enumName] = vals
		}
	}
	return result
}

// parseResourceProto parses a single resource .proto file and returns Fields.
// resourceName is the snake_case resource name (e.g. "campaign").
// enums is the global enum map used to populate enum_values.
func parseResourceProto(path, resourceName string, enums map[string][]string) []Field {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot read %s: %v\n", path, err)
		return nil
	}
	content := string(data)

	// Skip non-resource proto files (services, etc.)
	if !strings.Contains(content, "option (google.api.resource)") &&
		!strings.Contains(content, "message "+snakeToCamel(resourceName)+" {") {
		return nil
	}

	var fields []Field
	seen := map[string]bool{}

	for _, m := range fieldLine.FindAllStringSubmatch(content, -1) {
		comment := strings.TrimSpace(m[1])
		modifier := strings.TrimSpace(m[2])
		pType := strings.TrimSpace(m[3])
		fieldName := m[4]

		// Skip proto keywords that look like fields
		skip := []string{"option", "message", "enum", "reserved", "oneof", "map", "service", "rpc"}
		bad := false
		for _, s := range skip {
			if pType == s || fieldName == s {
				bad = true
				break
			}
		}
		if bad {
			continue
		}

		isRepeated := modifier == "repeated"
		dt := dataType(pType)

		// Build dot-notation field name
		qualName := resourceName + "." + fieldName
		if seen[qualName] {
			continue
		}
		seen[qualName] = true

		// Populate enum values when possible
		var enumVals []string
		if dt == "ENUM" {
			// pType ends with "Enum.SomeValue"; key is the inner type name
			parts := strings.SplitN(pType, ".", -1)
			innerName := parts[len(parts)-1]
			if v, ok := enums[innerName]; ok {
				enumVals = v
			} else if v, ok := enums[innerName+"Enum"]; ok {
				enumVals = v
			}
		}

		sortable := !isRepeated && dt != "MESSAGE"

		fields = append(fields, Field{
			Name:        qualName,
			DataType:    dt,
			Category:    "ATTRIBUTE",
			Resource:    resourceName,
			Selectable:  true,
			Filterable:  dt != "MESSAGE",
			Sortable:    sortable,
			IsRepeated:  isRepeated,
			Description: comment,
			EnumValues:  enumVals,
		})
	}

	return fields
}

// snakeToCamel converts snake_case to CamelCase (e.g. "ad_group" → "AdGroup").
func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

// ---- main ---------------------------------------------------------------

func main() {
	version := "v18"
	if len(os.Args) > 1 {
		version = os.Args[1]
	}

	protoBase := filepath.Join("gen", "proto", "google", "ads", "googleads", version)
	resourceDir := filepath.Join(protoBase, "resources")
	enumDir := filepath.Join(protoBase, "enums")
	outputPath := filepath.Join("internal", "schema", "data", "schema.json")

	if _, err := os.Stat(resourceDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: proto directory not found: %s\n", resourceDir)
		fmt.Fprintf(os.Stderr, "run gen/proto_fetch.sh first\n")
		os.Exit(1)
	}

	fmt.Printf("Parsing protos for API %s...\n", version)

	enums := enumsFromDir(enumDir)
	fmt.Printf("  Loaded %d enum types from %s\n", len(enums), enumDir)

	entries, err := os.ReadDir(resourceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", resourceDir, err)
		os.Exit(1)
	}

	schema := Schema{
		APIVersion:  version,
		GeneratedAt: time.Now().UTC().Format("2006-01-02"),
	}

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".proto") {
			continue
		}
		resourceName := strings.TrimSuffix(e.Name(), ".proto")
		path := filepath.Join(resourceDir, e.Name())
		fields := parseResourceProto(path, resourceName, enums)
		if len(fields) > 0 {
			fmt.Printf("  %-40s %d fields\n", resourceName, len(fields))
			schema.Fields = append(schema.Fields, fields...)
		}
	}

	// Sort for deterministic output
	sort.Slice(schema.Fields, func(i, j int) bool {
		return schema.Fields[i].Name < schema.Fields[j].Name
	})

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output dir: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Printf("\nWrote %s (%d fields)\n", outputPath, len(schema.Fields))
}
