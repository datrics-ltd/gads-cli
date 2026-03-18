// Package output provides formatters for rendering command results.
// Supported formats: table (default), json, csv.
// Full implementations will be added in Phase 1 — Foundation.
package output

import "io"

// Format represents an output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatCSV   Format = "csv"
)

// ParseFormat parses a format string, returning an error for unknown values.
func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case FormatTable, FormatJSON, FormatCSV:
		return Format(s), nil
	default:
		return "", &UnknownFormatError{Value: s}
	}
}

// UnknownFormatError is returned when an unrecognised format string is provided.
type UnknownFormatError struct {
	Value string
}

func (e *UnknownFormatError) Error() string {
	return "unknown output format: " + e.Value + " (valid: table, json, csv)"
}

// Formatter is the interface implemented by all output formatters.
type Formatter interface {
	// Format writes rows to w. headers is an ordered list of column names;
	// rows is a slice of maps keyed by header name.
	Format(w io.Writer, headers []string, rows []map[string]interface{}) error
}
