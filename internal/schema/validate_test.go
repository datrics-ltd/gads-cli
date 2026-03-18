package schema

import (
	"strings"
	"testing"
)

func TestValidateGAQL_ValidQuery(t *testing.T) {
	// A straightforward valid query that uses known selectable/filterable fields.
	q := `SELECT campaign.name, campaign.status, metrics.clicks, metrics.impressions
FROM campaign
WHERE campaign.status = 'ENABLED'
ORDER BY metrics.clicks DESC
LIMIT 100`
	if err := ValidateGAQL(q); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateGAQL_UnknownResource(t *testing.T) {
	// Unknown resource — should pass validation (skip to avoid false positives).
	q := `SELECT some_new_resource.field FROM some_new_resource`
	if err := ValidateGAQL(q); err != nil {
		t.Errorf("unknown resource should not fail validation, got: %v", err)
	}
}

func TestValidateGAQL_UnknownField(t *testing.T) {
	// Field not in embedded schema — should pass (API handles unknown fields).
	q := `SELECT campaign.name, campaign.unknown_future_field FROM campaign`
	if err := ValidateGAQL(q); err != nil {
		t.Errorf("unknown field should not fail validation, got: %v", err)
	}
}

func TestValidateGAQL_WrongResource(t *testing.T) {
	// ad_group.id is an ATTRIBUTE of ad_group, used in a FROM campaign query.
	q := `SELECT campaign.name, ad_group.id FROM campaign`
	err := ValidateGAQL(q)
	if err == nil {
		t.Fatal("expected validation error for cross-resource field, got nil")
	}
	if !strings.Contains(err.Error(), "ad_group.id") {
		t.Errorf("expected error to mention ad_group.id, got: %v", err)
	}
}

func TestValidateGAQL_NonSelectableField(t *testing.T) {
	// Verify that a field known to be non-selectable triggers an error.
	// campaign.resource_name is selectable, so let's find a non-selectable one.
	// We'll inject a fake field into byName for this test.
	load()
	byName["test.nonselectable"] = Field{
		Name:       "test.nonselectable",
		DataType:   "STRING",
		Category:   "ATTRIBUTE",
		Resource:   "campaign",
		Selectable: false,
		Filterable: false,
	}
	defer delete(byName, "test.nonselectable")

	q := `SELECT campaign.name, test.nonselectable FROM campaign`
	err := ValidateGAQL(q)
	if err == nil {
		t.Fatal("expected validation error for non-selectable field, got nil")
	}
	if !strings.Contains(err.Error(), "not selectable") {
		t.Errorf("expected 'not selectable' in error, got: %v", err)
	}
}

func TestValidateGAQL_NonFilterableFieldInWhere(t *testing.T) {
	load()
	byName["test.nonfilterable"] = Field{
		Name:       "test.nonfilterable",
		DataType:   "STRING",
		Category:   "ATTRIBUTE",
		Resource:   "campaign",
		Selectable: true,
		Filterable: false,
	}
	defer delete(byName, "test.nonfilterable")

	q := `SELECT campaign.name FROM campaign WHERE test.nonfilterable = 'x'`
	err := ValidateGAQL(q)
	if err == nil {
		t.Fatal("expected validation error for non-filterable field in WHERE, got nil")
	}
	if !strings.Contains(err.Error(), "not filterable") {
		t.Errorf("expected 'not filterable' in error, got: %v", err)
	}
}

func TestValidateGAQL_NonSortableFieldInOrderBy(t *testing.T) {
	load()
	byName["test.nonsortable"] = Field{
		Name:       "test.nonsortable",
		DataType:   "STRING",
		Category:   "ATTRIBUTE",
		Resource:   "campaign",
		Selectable: true,
		Filterable: true,
		Sortable:   false,
	}
	defer delete(byName, "test.nonsortable")

	q := `SELECT campaign.name FROM campaign ORDER BY test.nonsortable ASC`
	err := ValidateGAQL(q)
	if err == nil {
		t.Fatal("expected validation error for non-sortable field in ORDER BY, got nil")
	}
	if !strings.Contains(err.Error(), "not sortable") {
		t.Errorf("expected 'not sortable' in error, got: %v", err)
	}
}

func TestValidateGAQL_MissingFrom(t *testing.T) {
	// No FROM clause — not parseable, so no validation error (API will catch).
	q := `SELECT campaign.name`
	if err := ValidateGAQL(q); err != nil {
		t.Errorf("unparseable query should not fail validation, got: %v", err)
	}
}

func TestValidateGAQL_MultipleIssues(t *testing.T) {
	load()
	byName["test.bad1"] = Field{Name: "test.bad1", Category: "ATTRIBUTE", Resource: "campaign", Selectable: false}
	byName["test.bad2"] = Field{Name: "test.bad2", Category: "ATTRIBUTE", Resource: "campaign", Filterable: false}
	defer func() {
		delete(byName, "test.bad1")
		delete(byName, "test.bad2")
	}()

	q := `SELECT campaign.name, test.bad1 FROM campaign WHERE test.bad2 = 'x'`
	err := ValidateGAQL(q)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Issues) < 2 {
		t.Errorf("expected at least 2 issues, got %d: %v", len(ve.Issues), ve.Issues)
	}
}

func TestValidateGAQL_ComplexWhereWithAnd(t *testing.T) {
	// Multiple WHERE conditions — all valid fields.
	q := `SELECT campaign.name, metrics.clicks FROM campaign
WHERE campaign.status = 'ENABLED' AND segments.date DURING LAST_7_DAYS`
	if err := ValidateGAQL(q); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestParseFieldList(t *testing.T) {
	cases := []struct {
		input  string
		expect []string
	}{
		{"campaign.name, metrics.clicks, metrics.impressions", []string{"campaign.name", "metrics.clicks", "metrics.impressions"}},
		{"  campaign.id  ,  campaign.status  ", []string{"campaign.id", "campaign.status"}},
		{"campaign.name AS name, metrics.clicks", []string{"campaign.name", "metrics.clicks"}},
	}
	for _, c := range cases {
		got := parseFieldList(c.input)
		if len(got) != len(c.expect) {
			t.Errorf("parseFieldList(%q): got %v, want %v", c.input, got, c.expect)
			continue
		}
		for i := range got {
			if got[i] != c.expect[i] {
				t.Errorf("parseFieldList(%q)[%d]: got %q, want %q", c.input, i, got[i], c.expect[i])
			}
		}
	}
}

func TestParseWhereFields(t *testing.T) {
	s := "campaign.status = 'ENABLED' AND segments.date DURING LAST_7_DAYS AND metrics.clicks > 0"
	got := parseWhereFields(s)
	want := []string{"campaign.status", "segments.date", "metrics.clicks"}
	if len(got) != len(want) {
		t.Errorf("got %v, want %v", got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}
