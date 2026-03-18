// Package output provides formatters for rendering command results.
// Supported formats: table (default), json, csv.
package output

import (
	"io"
	"os"
)

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

// Options configures formatter behaviour.
type Options struct {
	NoColor bool // disable ANSI colour output (also honoured via NO_COLOR env var)
	Compact bool // JSON: single-line output
	BOM     bool // CSV: prepend UTF-8 BOM for Excel compatibility
	Verbose bool // JSON: include metadata envelope
	Meta    Meta // JSON metadata (used when Verbose is true)
}

// Meta is the metadata envelope emitted by the JSON formatter in verbose mode.
type Meta struct {
	CustomerID string `json:"customer_id,omitempty"`
	Query      string `json:"query,omitempty"`
	Rows       int    `json:"rows"`
	Timestamp  string `json:"timestamp"`
}

// New returns a Formatter for the given format and options.
func New(format Format, opts Options) Formatter {
	switch format {
	case FormatJSON:
		return &JSONFormatter{compact: opts.Compact, verbose: opts.Verbose, meta: opts.Meta}
	case FormatCSV:
		return &CSVFormatter{bom: opts.BOM}
	default:
		noColor := opts.NoColor || os.Getenv("NO_COLOR") != ""
		return &TableFormatter{noColor: noColor}
	}
}
