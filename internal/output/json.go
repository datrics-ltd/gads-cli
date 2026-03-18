package output

import (
	"encoding/json"
	"io"
	"time"
)

// JSONFormatter renders rows as a JSON array (or verbose envelope).
type JSONFormatter struct {
	compact bool
	verbose bool
	meta    Meta
}

func (f *JSONFormatter) Format(w io.Writer, headers []string, rows []map[string]interface{}) error {
	// Build output slice preserving only declared header keys.
	data := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		m := make(map[string]interface{}, len(headers))
		for _, h := range headers {
			m[h] = row[h]
		}
		data[i] = m
	}

	var out interface{} = data
	if f.verbose {
		meta := f.meta
		meta.Rows = len(rows)
		if meta.Timestamp == "" {
			meta.Timestamp = time.Now().UTC().Format(time.RFC3339)
		}
		out = map[string]interface{}{
			"meta": meta,
			"data": data,
		}
	}

	enc := json.NewEncoder(w)
	if !f.compact {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(out)
}
