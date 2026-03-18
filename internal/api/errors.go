package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// APIError represents a parsed Google Ads API error response.
type APIError struct {
	HTTPStatus int
	Message    string
	Details    []ErrorDetail
	RawBody    string
}

func (e *APIError) Error() string {
	return e.Message
}

// ErrorDetail holds a single error entry from the Google Ads error response.
type ErrorDetail struct {
	ErrorCode string
	Message   string
}

// googleAdsFailure matches the REST error envelope from the Google Ads API.
type googleAdsFailure struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
		Details []struct {
			Type   string `json:"@type"`
			Errors []struct {
				ErrorCode json.RawMessage `json:"errorCode"`
				Message   string          `json:"message"`
			} `json:"errors"`
			Reason string `json:"reason"` // quota errors
		} `json:"details"`
	} `json:"error"`
}

// ParseAPIError reads an error response body and returns a structured APIError.
// It maps known Google Ads error codes to human-readable messages.
func ParseAPIError(resp *http.Response) *APIError {
	body, _ := io.ReadAll(resp.Body)

	apiErr := &APIError{
		HTTPStatus: resp.StatusCode,
		RawBody:    string(body),
	}

	var failure googleAdsFailure
	if err := json.Unmarshal(body, &failure); err != nil || failure.Error.Code == 0 {
		// Not a structured error — fall back to status text.
		apiErr.Message = mapHTTPStatus(resp.StatusCode, string(body))
		return apiErr
	}

	// Collect detailed error entries.
	for _, detail := range failure.Error.Details {
		for _, e := range detail.Errors {
			code := extractErrorCode(e.ErrorCode)
			apiErr.Details = append(apiErr.Details, ErrorDetail{
				ErrorCode: code,
				Message:   e.Message,
			})
		}
	}

	apiErr.Message = buildMessage(resp.StatusCode, &failure)
	return apiErr
}

// mapHTTPStatus returns a human-readable message for common HTTP error codes.
func mapHTTPStatus(status int, body string) string {
	switch status {
	case 401:
		return "authentication failed — run `gads auth login`"
	case 403:
		return "not authorized for this resource — check your developer token and customer ID"
	case 429:
		return "API rate limit exceeded — try again later"
	case 500, 503:
		return fmt.Sprintf("Google Ads API server error (%d) — try again shortly", status)
	default:
		if len(body) > 200 {
			body = body[:200] + "..."
		}
		return fmt.Sprintf("unexpected API response (%d): %s", status, body)
	}
}

// buildMessage constructs a human-readable message from a structured failure.
func buildMessage(status int, failure *googleAdsFailure) string {
	// Check for known error types in details.
	for _, detail := range failure.Error.Details {
		for _, e := range detail.Errors {
			code := strings.ToLower(extractErrorCode(e.ErrorCode))
			switch {
			case strings.Contains(code, "authentication"):
				return fmt.Sprintf("authentication failed — run `gads auth login` (%s)", e.Message)
			case strings.Contains(code, "authorization"):
				return fmt.Sprintf("not authorized — %s", e.Message)
			case strings.Contains(code, "quota"):
				return "API quota exceeded — try again later"
			case strings.Contains(code, "query"):
				return fmt.Sprintf("GAQL syntax error: %s", e.Message)
			case strings.Contains(code, "request"):
				return fmt.Sprintf("invalid request: %s", e.Message)
			}
		}
		// Quota error at detail level.
		if strings.Contains(strings.ToLower(detail.Type), "quota") && detail.Reason != "" {
			return fmt.Sprintf("API quota exceeded (%s) — try again later", detail.Reason)
		}
	}

	// Fall back to the top-level message.
	if failure.Error.Message != "" {
		return mapStatusCode(status, failure.Error.Message)
	}
	return mapHTTPStatus(status, "")
}

func mapStatusCode(status int, msg string) string {
	switch status {
	case 401:
		return fmt.Sprintf("authentication failed: %s — run `gads auth login`", msg)
	case 403:
		return fmt.Sprintf("not authorized: %s", msg)
	case 429:
		return "API rate limit exceeded — try again later"
	default:
		return msg
	}
}

// extractErrorCode pulls the first key from an errorCode JSON object.
// Google Ads error codes are objects like {"queryError": "UNRECOGNIZED_FIELD"}.
func extractErrorCode(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return string(raw)
	}
	for k, v := range m {
		return fmt.Sprintf("%s.%v", k, v)
	}
	return ""
}
