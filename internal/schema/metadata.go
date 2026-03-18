// Package schema provides access to embedded Google Ads API field metadata.
//
// The metadata is generated from proto definitions by gen/codegen.go and
// embedded into the binary at build time via go:embed. Run gen/proto_fetch.sh
// then go run gen/codegen.go to regenerate after an API version upgrade.
package schema

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed data/schema.json
var schemaJSON []byte

// Field describes a single Google Ads API field.
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

// schemaDoc is the top-level structure of schema.json.
type schemaDoc struct {
	APIVersion  string  `json:"api_version"`
	GeneratedAt string  `json:"generated_at"`
	Fields      []Field `json:"fields"`
}

var (
	once   sync.Once
	doc    schemaDoc
	byName map[string]Field // keyed by field name
)

func load() {
	once.Do(func() {
		if err := json.Unmarshal(schemaJSON, &doc); err != nil {
			panic("schema: failed to parse embedded schema.json: " + err.Error())
		}
		byName = make(map[string]Field, len(doc.Fields))
		for _, f := range doc.Fields {
			byName[f.Name] = f
		}
	})
}

// APIVersion returns the Google Ads API version this schema was built for.
func APIVersion() string {
	load()
	return doc.APIVersion
}

// GeneratedAt returns the date the schema was generated (YYYY-MM-DD).
func GeneratedAt() string {
	load()
	return doc.GeneratedAt
}

// ListResources returns a sorted, deduplicated list of resource names that have
// at least one ATTRIBUTE field in the schema (e.g. "campaign", "ad_group").
// Metric and segment field prefixes ("metrics", "segments") are not included.
func ListResources() []string {
	load()
	seen := map[string]bool{}
	var resources []string
	for _, f := range doc.Fields {
		if f.Category == "ATTRIBUTE" && f.Resource != "" && !seen[f.Resource] {
			seen[f.Resource] = true
			resources = append(resources, f.Resource)
		}
	}
	// Simple insertion-order list is fine; sort for determinism.
	sortStrings(resources)
	return resources
}

// GetField returns the field with the given dot-notation name, if it exists.
func GetField(name string) (Field, bool) {
	load()
	f, ok := byName[name]
	return f, ok
}

// GetFields returns all fields associated with a given resource.
// For resource queries (e.g. "campaign") this includes:
//   - All ATTRIBUTE fields whose Resource == resourceName
//   - All METRIC fields (always available in SELECT)
//   - All SEGMENT fields (always available in SELECT)
func GetFields(resourceName string) []Field {
	load()
	var out []Field
	resourceName = strings.ToLower(strings.TrimSpace(resourceName))
	for _, f := range doc.Fields {
		switch f.Category {
		case "ATTRIBUTE":
			if f.Resource == resourceName {
				out = append(out, f)
			}
		case "METRIC", "SEGMENT":
			out = append(out, f)
		}
	}
	return out
}

// GetSelectableFields returns only the selectable fields for a resource.
func GetSelectableFields(resourceName string) []Field {
	return filterFields(GetFields(resourceName), func(f Field) bool {
		return f.Selectable
	})
}

// GetFilterableFields returns only the filterable fields for a resource.
func GetFilterableFields(resourceName string) []Field {
	return filterFields(GetFields(resourceName), func(f Field) bool {
		return f.Filterable
	})
}

// GetSortableFields returns only the sortable fields for a resource.
func GetSortableFields(resourceName string) []Field {
	return filterFields(GetFields(resourceName), func(f Field) bool {
		return f.Sortable
	})
}

// GetAttributeFields returns only ATTRIBUTE fields (no metrics/segments) for a resource.
func GetAttributeFields(resourceName string) []Field {
	load()
	resourceName = strings.ToLower(strings.TrimSpace(resourceName))
	var out []Field
	for _, f := range doc.Fields {
		if f.Category == "ATTRIBUTE" && f.Resource == resourceName {
			out = append(out, f)
		}
	}
	return out
}

// AllFields returns every field in the embedded schema.
func AllFields() []Field {
	load()
	return doc.Fields
}

// --- helpers ----------------------------------------------------------------

func filterFields(fields []Field, keep func(Field) bool) []Field {
	var out []Field
	for _, f := range fields {
		if keep(f) {
			out = append(out, f)
		}
	}
	return out
}

// sortStrings sorts a string slice in-place (stdlib sort not imported to keep
// the package lightweight — simple insertion sort is fine for small slices).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}
