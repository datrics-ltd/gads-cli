package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

var testHeaders = []string{"name", "status", "clicks"}
var testRows = []map[string]interface{}{
	{"name": "Brand Search", "status": "ENABLED", "clicks": float64(1234)},
	{"name": "Generic Display", "status": "PAUSED", "clicks": float64(567)},
}

func TestTableFormatter(t *testing.T) {
	f := New(FormatTable, Options{NoColor: true})
	var buf bytes.Buffer
	if err := f.Format(&buf, testHeaders, testRows); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Brand Search") {
		t.Errorf("expected 'Brand Search' in output:\n%s", out)
	}
	if !strings.Contains(out, "1,234") {
		t.Errorf("expected '1,234' formatted number in output:\n%s", out)
	}
}

func TestTableFormatterTotals(t *testing.T) {
	f := New(FormatTable, Options{NoColor: true})
	var buf bytes.Buffer
	if err := f.Format(&buf, testHeaders, testRows); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// Total of clicks: 1234 + 567 = 1801
	if !strings.Contains(out, "1,801") {
		t.Errorf("expected footer total '1,801' in output:\n%s", out)
	}
}

func TestJSONFormatter(t *testing.T) {
	f := New(FormatJSON, Options{})
	var buf bytes.Buffer
	if err := f.Format(&buf, testHeaders, testRows); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var arr []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &arr); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 rows, got %d", len(arr))
	}
}

func TestJSONFormatterVerbose(t *testing.T) {
	f := New(FormatJSON, Options{Verbose: true, Meta: Meta{CustomerID: "123", Query: "SELECT *"}})
	var buf bytes.Buffer
	if err := f.Format(&buf, testHeaders, testRows); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var env map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := env["meta"]; !ok {
		t.Error("expected 'meta' key in verbose JSON output")
	}
	if _, ok := env["data"]; !ok {
		t.Error("expected 'data' key in verbose JSON output")
	}
}

func TestCSVFormatter(t *testing.T) {
	f := New(FormatCSV, Options{})
	var buf bytes.Buffer
	if err := f.Format(&buf, testHeaders, testRows); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 { // header + 2 data rows
		t.Errorf("expected 3 lines, got %d:\n%s", len(lines), buf.String())
	}
	if lines[0] != "name,status,clicks" {
		t.Errorf("unexpected header line: %q", lines[0])
	}
}

func TestCSVFormatterBOM(t *testing.T) {
	f := New(FormatCSV, Options{BOM: true})
	var buf bytes.Buffer
	if err := f.Format(&buf, testHeaders, testRows); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b := buf.Bytes()
	if len(b) < 3 || b[0] != 0xEF || b[1] != 0xBB || b[2] != 0xBF {
		t.Error("expected UTF-8 BOM at start of CSV output")
	}
}

func TestFormatNumber(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{1234.56, "1,234.56"},
		{-9876, "-9,876"},
	}
	for _, c := range cases {
		got := formatNumber(c.in)
		if got != c.want {
			t.Errorf("formatNumber(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseFormat(t *testing.T) {
	for _, f := range []string{"table", "json", "csv"} {
		if _, err := ParseFormat(f); err != nil {
			t.Errorf("ParseFormat(%q) unexpected error: %v", f, err)
		}
	}
	if _, err := ParseFormat("xml"); err == nil {
		t.Error("ParseFormat('xml') should return error")
	}
}
