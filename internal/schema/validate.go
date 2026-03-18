package schema

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationIssue represents a single validation problem found in a GAQL query.
type ValidationIssue struct {
	Field   string // the field name that caused the issue (empty for resource-level issues)
	Message string
}

func (v ValidationIssue) Error() string {
	return v.Message
}

// ValidationResult holds the outcome of ValidateGAQL.
type ValidationResult struct {
	Resource string
	Issues   []ValidationIssue
}

// Valid returns true when there are no validation issues.
func (r ValidationResult) Valid() bool {
	return len(r.Issues) == 0
}

// ValidationError is returned by ValidateGAQL when issues are found.
type ValidationError struct {
	Issues []ValidationIssue
}

func (e *ValidationError) Error() string {
	if len(e.Issues) == 1 {
		return "GAQL validation: " + e.Issues[0].Message
	}
	lines := make([]string, len(e.Issues))
	for i, iss := range e.Issues {
		lines[i] = "  • " + iss.Message
	}
	return "GAQL validation errors:\n" + strings.Join(lines, "\n")
}

// field-name pattern: one or more dot-separated lowercase identifier segments.
var fieldPattern = regexp.MustCompile(`\b([a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)+)\b`)

// clauseRE matches major GAQL clauses for splitting. GAQL is case-insensitive by convention.
var (
	reSelect  = regexp.MustCompile(`(?i)\bSELECT\b`)
	reFrom    = regexp.MustCompile(`(?i)\bFROM\b`)
	reWhere   = regexp.MustCompile(`(?i)\bWHERE\b`)
	reOrderBy = regexp.MustCompile(`(?i)\bORDER\s+BY\b`)
	reLimit   = regexp.MustCompile(`(?i)\bLIMIT\b`)
	reParams  = regexp.MustCompile(`(?i)\bPARAMETERS\b`)
)

// ValidateGAQL checks a GAQL query string against the embedded schema.
//
// Only issues that can be confirmed using the embedded schema are reported —
// fields absent from the schema are skipped to avoid false positives (the
// embedded schema covers a subset of the full API surface).
//
// Returns nil if validation passes or if the query cannot be parsed (the API
// will report a more precise error in that case).
func ValidateGAQL(gaql string) error {
	result := validateGAQL(gaql)
	if !result.Valid() {
		return &ValidationError{Issues: result.Issues}
	}
	return nil
}

func validateGAQL(gaql string) ValidationResult {
	load()

	q := strings.TrimSpace(gaql)

	// ---- locate clause boundaries ------------------------------------------

	fromIdx := reFrom.FindStringIndex(q)
	if fromIdx == nil {
		// Can't parse; let the API handle it.
		return ValidationResult{}
	}

	selectIdx := reSelect.FindStringIndex(q)
	whereIdx := reWhere.FindStringIndex(q)
	orderByIdx := reOrderBy.FindStringIndex(q)
	limitIdx := reLimit.FindStringIndex(q)
	paramsIdx := reParams.FindStringIndex(q)

	// ---- extract FROM resource ---------------------------------------------

	// FROM ends at the next clause keyword or end-of-string.
	fromEnd := len(q)
	for _, idx := range [][]int{whereIdx, orderByIdx, limitIdx, paramsIdx} {
		if idx != nil && idx[0] < fromEnd {
			fromEnd = idx[0]
		}
	}
	resourceStr := strings.TrimSpace(q[fromIdx[1]:fromEnd])
	// Resource name may contain underscores; trim any trailing whitespace/comment.
	resource := strings.Fields(resourceStr)
	var resourceName string
	if len(resource) > 0 {
		resourceName = strings.ToLower(resource[0])
	}

	result := ValidationResult{Resource: resourceName}

	// Validate resource exists in schema (only if we have ATTRIBUTE fields for it).
	knownResources := map[string]bool{}
	for _, f := range doc.Fields {
		if f.Category == "ATTRIBUTE" {
			knownResources[f.Resource] = true
		}
	}
	resourceKnown := knownResources[resourceName]
	if resourceName != "" && !resourceKnown {
		// Unknown resource — skip further validation to avoid false positives.
		return result
	}

	// ---- extract SELECT fields ---------------------------------------------

	if selectIdx != nil {
		selectEnd := fromIdx[0]
		selectStr := q[selectIdx[1]:selectEnd]
		for _, fieldName := range parseFieldList(selectStr) {
			if f, ok := byName[fieldName]; ok {
				if !f.Selectable {
					result.Issues = append(result.Issues, ValidationIssue{
						Field:   fieldName,
						Message: fmt.Sprintf("field %q is not selectable", fieldName),
					})
				}
				if resourceKnown && f.Category == "ATTRIBUTE" && f.Resource != resourceName {
					result.Issues = append(result.Issues, ValidationIssue{
						Field:   fieldName,
						Message: fmt.Sprintf("field %q belongs to resource %q, not %q", fieldName, f.Resource, resourceName),
					})
				}
			}
			// Unknown fields: skip — the API will validate them.
		}
	}

	// ---- extract WHERE fields ----------------------------------------------

	if whereIdx != nil {
		whereEnd := len(q)
		for _, idx := range [][]int{orderByIdx, limitIdx, paramsIdx} {
			if idx != nil && idx[0] < whereEnd {
				whereEnd = idx[0]
			}
		}
		whereStr := q[whereIdx[1]:whereEnd]
		for _, fieldName := range parseWhereFields(whereStr) {
			if f, ok := byName[fieldName]; ok {
				if !f.Filterable {
					result.Issues = append(result.Issues, ValidationIssue{
						Field:   fieldName,
						Message: fmt.Sprintf("field %q is not filterable (cannot be used in WHERE clause)", fieldName),
					})
				}
				if resourceKnown && f.Category == "ATTRIBUTE" && f.Resource != resourceName {
					result.Issues = append(result.Issues, ValidationIssue{
						Field:   fieldName,
						Message: fmt.Sprintf("field %q belongs to resource %q, not %q (incompatible with FROM %s)", fieldName, f.Resource, resourceName, resourceName),
					})
				}
			}
		}
	}

	// ---- extract ORDER BY fields -------------------------------------------

	if orderByIdx != nil {
		orderEnd := len(q)
		for _, idx := range [][]int{limitIdx, paramsIdx} {
			if idx != nil && idx[0] < orderEnd {
				orderEnd = idx[0]
			}
		}
		orderStr := q[orderByIdx[1]:orderEnd]
		for _, fieldName := range parseOrderByFields(orderStr) {
			if f, ok := byName[fieldName]; ok {
				if !f.Sortable {
					result.Issues = append(result.Issues, ValidationIssue{
						Field:   fieldName,
						Message: fmt.Sprintf("field %q is not sortable (cannot be used in ORDER BY clause)", fieldName),
					})
				}
			}
		}
	}

	return result
}

// parseFieldList splits a comma-separated SELECT list into field names.
// Handles multi-line and extra whitespace.
func parseFieldList(s string) []string {
	parts := strings.Split(s, ",")
	var fields []string
	for _, p := range parts {
		name := strings.TrimSpace(p)
		// Strip any alias (AS keyword)
		if i := strings.Index(strings.ToUpper(name), " AS "); i >= 0 {
			name = strings.TrimSpace(name[:i])
		}
		if name != "" && strings.Contains(name, ".") {
			fields = append(fields, strings.ToLower(name))
		}
	}
	return fields
}

// parseWhereFields extracts field names (left-hand side of each condition)
// from a WHERE clause body. Field names are in dot-notation.
func parseWhereFields(s string) []string {
	// Tokenize by AND/OR (case-insensitive).
	conditions := regexp.MustCompile(`(?i)\b(?:AND|OR)\b`).Split(s, -1)
	seen := map[string]bool{}
	var fields []string
	for _, cond := range conditions {
		cond = strings.TrimSpace(cond)
		// The field name is the first dot-notation token.
		matches := fieldPattern.FindString(cond)
		if matches != "" {
			name := strings.ToLower(matches)
			if !seen[name] {
				seen[name] = true
				fields = append(fields, name)
			}
		}
	}
	return fields
}

// parseOrderByFields extracts field names from an ORDER BY clause body.
// Format: "field1 [ASC|DESC], field2 [ASC|DESC], ..."
func parseOrderByFields(s string) []string {
	parts := strings.Split(s, ",")
	var fields []string
	for _, p := range parts {
		tokens := strings.Fields(p)
		if len(tokens) == 0 {
			continue
		}
		name := strings.ToLower(tokens[0])
		if strings.Contains(name, ".") {
			fields = append(fields, name)
		}
	}
	return fields
}
