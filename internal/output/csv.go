package output

import (
	"encoding/csv"
	"fmt"
	"io"
)

// CSVFormatter renders rows as RFC 4180 CSV.
type CSVFormatter struct {
	bom bool // prepend UTF-8 BOM for Excel compatibility
}

func (f *CSVFormatter) Format(w io.Writer, headers []string, rows []map[string]interface{}) error {
	if f.bom {
		if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
			return err
		}
	}

	cw := csv.NewWriter(w)

	if err := cw.Write(headers); err != nil {
		return err
	}

	for _, row := range rows {
		record := make([]string, len(headers))
		for i, h := range headers {
			v := row[h]
			if v == nil {
				record[i] = ""
			} else if n, ok := toFloat64(v); ok {
				// Raw numbers — no formatting or currency symbols.
				record[i] = fmt.Sprintf("%g", n)
			} else {
				record[i] = fmt.Sprintf("%v", v)
			}
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}
