package api

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// apiVersion is the Google Ads API version used for all requests.
const apiVersion = "v23"

// searchStreamResponse is one chunk from the googleAds:searchStream endpoint.
type searchStreamResponse struct {
	Results   []map[string]interface{} `json:"results"`
	FieldMask string                   `json:"fieldMask"`
	RequestID string                   `json:"requestId"`
}

// ExecuteGAQL runs a GAQL query against the search stream endpoint and returns
// ordered headers and flattened result rows. The customerID parameter overrides
// the client's default if non-empty.
func (c *Client) ExecuteGAQL(customerID, query string) ([]string, []map[string]interface{}, error) {
	if customerID == "" {
		customerID = c.customerID
	}
	if customerID == "" {
		return nil, nil, fmt.Errorf("customer ID is required — set via --customer-id or `gads config set default_customer_id <id>`")
	}
	// Normalize: strip dashes (123-456-7890 → 1234567890).
	customerID = strings.ReplaceAll(customerID, "-", "")

	path := fmt.Sprintf("/%s/customers/%s/googleAds:searchStream", apiVersion, customerID)

	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return nil, nil, fmt.Errorf("encoding query: %w", err)
	}

	resp, err := c.Post(path, body, nil)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading response: %w", err)
	}

	chunks, err := parseStreamResponse(rawBody)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing stream response: %w", err)
	}

	// Collect rows and the field mask from the first chunk that has one.
	var allRows []map[string]interface{}
	var fieldMask string
	for _, chunk := range chunks {
		if fieldMask == "" && chunk.FieldMask != "" {
			fieldMask = chunk.FieldMask
		}
		for _, result := range chunk.Results {
			allRows = append(allRows, flattenResult(result))
		}
	}

	headers := headersFromMask(fieldMask, allRows)
	return headers, allRows, nil
}

// parseStreamResponse handles both a JSON array of stream chunks and a bare
// single-object response (defensive fallback).
func parseStreamResponse(body []byte) ([]searchStreamResponse, error) {
	// Try JSON array first (normal streaming response).
	var chunks []searchStreamResponse
	if err := json.Unmarshal(body, &chunks); err == nil {
		return chunks, nil
	}
	// Fall back to a single JSON object.
	var single searchStreamResponse
	if err := json.Unmarshal(body, &single); err != nil {
		return nil, fmt.Errorf("response is neither a JSON array nor a JSON object: %w", err)
	}
	return []searchStreamResponse{single}, nil
}

// headersFromMask returns an ordered slice of field names. If fieldMask is set
// it is used directly; otherwise headers are derived from the first row's keys.
func headersFromMask(fieldMask string, rows []map[string]interface{}) []string {
	if fieldMask != "" {
		return strings.Split(fieldMask, ",")
	}
	if len(rows) == 0 {
		return nil
	}
	headers := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		headers = append(headers, k)
	}
	return headers
}

// flattenResult converts a nested result map to a flat map with dot-separated keys.
// e.g. {"campaign": {"name": "Foo"}} → {"campaign.name": "Foo"}
func flattenResult(m map[string]interface{}) map[string]interface{} {
	flat := make(map[string]interface{})
	flattenInto(flat, m, "")
	return flat
}

func flattenInto(dst map[string]interface{}, src map[string]interface{}, prefix string) {
	for k, v := range src {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := v.(map[string]interface{}); ok {
			flattenInto(dst, nested, key)
		} else {
			dst[key] = v
		}
	}
}
