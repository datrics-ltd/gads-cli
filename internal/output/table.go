package output

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"golang.org/x/term"
)

// TableFormatter renders rows as an aligned text table.
type TableFormatter struct {
	noColor bool
}

func (f *TableFormatter) Format(w io.Writer, headers []string, rows []map[string]interface{}) error {
	if len(headers) == 0 {
		return nil
	}

	maxWidth := termWidth()

	table := tablewriter.NewWriter(w)
	table.Configure(func(cfg *tablewriter.Config) {
		cfg.MaxWidth = maxWidth
		cfg.Header.Formatting.AutoFormat = tw.Off
		cfg.Row.Formatting.AutoWrap = tw.WrapNone
		cfg.Header.Alignment.Global = tw.AlignLeft
		cfg.Row.Alignment.Global = tw.AlignLeft
		cfg.Footer.Alignment.Global = tw.AlignLeft
	})
	table.Options(
		tablewriter.WithRendererSettings(tw.Settings{
			Separators: tw.Separators{
				ShowHeader:     tw.On,
				ShowFooter:     tw.Off,
				BetweenRows:    tw.Off,
				BetweenColumns: tw.Off,
			},
			Lines: tw.Lines{
				ShowHeaderLine: tw.On,
				ShowFooterLine: tw.Off,
			},
		}),
	)

	// Convert headers to interface slice for Header()
	headerIface := make([]interface{}, len(headers))
	for i, h := range headers {
		headerIface[i] = h
	}
	table.Header(headerIface...)

	// Track which columns are purely numeric (for footer totals).
	// Exclude ID-like columns since summing IDs is meaningless.
	colNumeric := make([]bool, len(headers))
	totals := make([]float64, len(headers))
	hasAnyRow := false

	for _, row := range rows {
		cells := make([]interface{}, len(headers))
		for i, h := range headers {
			v := row[h]
			cells[i] = f.formatCell(v)

			if !isIDColumn(h) {
				if n, ok := toFloat64(v); ok {
					if !hasAnyRow {
						colNumeric[i] = true
					}
					if colNumeric[i] {
						totals[i] += n
					}
				} else {
					colNumeric[i] = false
				}
			}
		}
		hasAnyRow = true
		if err := table.Append(cells...); err != nil {
			return err
		}
	}

	// Emit a footer with column totals when there are multiple rows and numeric data.
	if len(rows) > 1 {
		footer := make([]interface{}, len(headers))
		anyTotal := false
		for i, isNum := range colNumeric {
			if isNum {
				footer[i] = formatNumber(totals[i])
				anyTotal = true
			} else {
				footer[i] = ""
			}
		}
		if anyTotal {
			table.Footer(footer...)
		}
	}

	return table.Render()
}

// formatCell converts a value to a display string, applying colour for known status values.
func (f *TableFormatter) formatCell(v interface{}) string {
	if v == nil {
		return ""
	}

	// Format numeric values with comma separators.
	if n, ok := toFloat64(v); ok {
		return formatNumber(n)
	}

	s := fmt.Sprintf("%v", v)

	if !f.noColor {
		switch strings.ToUpper(strings.TrimSpace(s)) {
		case "ENABLED":
			return color.GreenString(s)
		case "PAUSED":
			return color.YellowString(s)
		case "REMOVED":
			return color.RedString(s)
		}
	}

	return s
}

// isIDColumn returns true if the column name looks like an identifier column.
func isIDColumn(name string) bool {
	lower := strings.ToLower(name)
	return lower == "id" ||
		strings.HasSuffix(lower, "_id") ||
		strings.HasSuffix(lower, " id")
}

// toFloat64 extracts a float64 from numeric interface values.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}

// formatNumber formats a float64 with comma separators.
// Whole numbers are rendered without a decimal point; others get two decimal places.
func formatNumber(f float64) string {
	neg := f < 0
	abs := math.Abs(f)

	if abs == math.Trunc(abs) && abs < 1e15 {
		s := commaInt(int64(abs))
		if neg {
			return "-" + s
		}
		return s
	}

	// Two decimal places.
	intPart := int64(abs)
	frac := math.Round((abs-float64(intPart))*100)
	if frac >= 100 {
		intPart++
		frac -= 100
	}
	s := fmt.Sprintf("%s.%02d", commaInt(intPart), int(frac))
	if neg {
		return "-" + s
	}
	return s
}

// commaInt formats a non-negative integer with comma separators.
func commaInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	start := len(s) % 3
	if start == 0 {
		start = 3
	}
	b.WriteString(s[:start])
	for i := start; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// termWidth returns the current terminal width, falling back to 0 (unlimited).
func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			return n
		}
	}
	return 0
}
