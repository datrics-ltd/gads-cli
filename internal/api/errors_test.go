package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func makeErrorResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestParseAPIErrorHTTP401Unstructured(t *testing.T) {
	resp := makeErrorResponse(401, "Unauthorized")
	err := ParseAPIError(resp)
	if err == nil {
		t.Fatal("expected non-nil APIError")
	}
	if err.HTTPStatus != 401 {
		t.Errorf("HTTPStatus: got %d, want 401", err.HTTPStatus)
	}
	if !strings.Contains(strings.ToLower(err.Message), "authentication") {
		t.Errorf("expected 'authentication' in message, got: %q", err.Message)
	}
}

func TestParseAPIErrorHTTP403Unstructured(t *testing.T) {
	resp := makeErrorResponse(403, "Forbidden")
	err := ParseAPIError(resp)
	if err == nil {
		t.Fatal("expected non-nil APIError")
	}
	if !strings.Contains(strings.ToLower(err.Message), "not authorized") {
		t.Errorf("expected 'not authorized' in message, got: %q", err.Message)
	}
}

func TestParseAPIErrorHTTP429(t *testing.T) {
	resp := makeErrorResponse(429, "Too Many Requests")
	err := ParseAPIError(resp)
	if err == nil {
		t.Fatal("expected non-nil APIError")
	}
	if !strings.Contains(strings.ToLower(err.Message), "rate limit") {
		t.Errorf("expected 'rate limit' in message, got: %q", err.Message)
	}
}

func TestParseAPIErrorHTTP500(t *testing.T) {
	resp := makeErrorResponse(500, "Internal Server Error")
	err := ParseAPIError(resp)
	if err == nil {
		t.Fatal("expected non-nil APIError")
	}
	if !strings.Contains(strings.ToLower(err.Message), "server error") {
		t.Errorf("expected 'server error' in message, got: %q", err.Message)
	}
}

func TestParseAPIErrorQueryError(t *testing.T) {
	body := `{
		"error": {
			"code": 400,
			"message": "Request contains an invalid argument.",
			"details": [{
				"@type": "type.googleapis.com/google.ads.googleads.v18.errors.GoogleAdsFailure",
				"errors": [{
					"errorCode": {"queryError": "UNRECOGNIZED_FIELD"},
					"message": "Error in field 'unknown_field': Field not found."
				}]
			}]
		}
	}`
	resp := makeErrorResponse(400, body)
	err := ParseAPIError(resp)
	if err == nil {
		t.Fatal("expected non-nil APIError")
	}
	if !strings.Contains(err.Message, "GAQL syntax error") {
		t.Errorf("expected 'GAQL syntax error' in message, got: %q", err.Message)
	}
	if len(err.Details) == 0 {
		t.Error("expected at least one error detail")
	}
	if err.Details[0].ErrorCode == "" {
		t.Error("expected non-empty ErrorCode in detail")
	}
}

func TestParseAPIErrorAuthenticationError(t *testing.T) {
	body := `{
		"error": {
			"code": 401,
			"message": "Token expired.",
			"details": [{
				"@type": "type.googleapis.com/google.ads.googleads.v18.errors.GoogleAdsFailure",
				"errors": [{
					"errorCode": {"authenticationError": "OAUTH_TOKEN_EXPIRED"},
					"message": "Your OAuth token has expired."
				}]
			}]
		}
	}`
	resp := makeErrorResponse(401, body)
	err := ParseAPIError(resp)
	if err == nil {
		t.Fatal("expected non-nil APIError")
	}
	if !strings.Contains(strings.ToLower(err.Message), "authentication") {
		t.Errorf("expected 'authentication' in message, got: %q", err.Message)
	}
}

func TestParseAPIErrorAuthorizationError(t *testing.T) {
	body := `{
		"error": {
			"code": 403,
			"message": "User not allowed to access resource.",
			"details": [{
				"@type": "type.googleapis.com/google.ads.googleads.v18.errors.GoogleAdsFailure",
				"errors": [{
					"errorCode": {"authorizationError": "USER_PERMISSION_DENIED"},
					"message": "User does not have permission to access this resource."
				}]
			}]
		}
	}`
	resp := makeErrorResponse(403, body)
	err := ParseAPIError(resp)
	if err == nil {
		t.Fatal("expected non-nil APIError")
	}
	if !strings.Contains(strings.ToLower(err.Message), "not authorized") {
		t.Errorf("expected 'not authorized' in message, got: %q", err.Message)
	}
}

func TestParseAPIErrorQuotaError(t *testing.T) {
	body := `{
		"error": {
			"code": 429,
			"message": "Resource has been exhausted.",
			"details": [{
				"@type": "type.googleapis.com/google.ads.googleads.v18.errors.GoogleAdsFailure",
				"errors": [{
					"errorCode": {"quotaError": "RESOURCE_EXHAUSTED"},
					"message": "Rate limit exceeded."
				}]
			}]
		}
	}`
	resp := makeErrorResponse(429, body)
	err := ParseAPIError(resp)
	if err == nil {
		t.Fatal("expected non-nil APIError")
	}
	if !strings.Contains(strings.ToLower(err.Message), "quota") {
		t.Errorf("expected 'quota' in message, got: %q", err.Message)
	}
}

func TestParseAPIErrorRequestError(t *testing.T) {
	body := `{
		"error": {
			"code": 400,
			"message": "Bad request.",
			"details": [{
				"@type": "type.googleapis.com/google.ads.googleads.v18.errors.GoogleAdsFailure",
				"errors": [{
					"errorCode": {"requestError": "INVALID_INPUT"},
					"message": "The request is malformed."
				}]
			}]
		}
	}`
	resp := makeErrorResponse(400, body)
	err := ParseAPIError(resp)
	if err == nil {
		t.Fatal("expected non-nil APIError")
	}
	if !strings.Contains(strings.ToLower(err.Message), "invalid request") {
		t.Errorf("expected 'invalid request' in message, got: %q", err.Message)
	}
}

func TestParseAPIErrorPreservesRawBody(t *testing.T) {
	rawBody := `{"error": {"code": 500, "message": "server failure"}}`
	resp := makeErrorResponse(500, rawBody)
	err := ParseAPIError(resp)
	if err == nil {
		t.Fatal("expected non-nil APIError")
	}
	if err.RawBody != rawBody {
		t.Errorf("RawBody: got %q, want %q", err.RawBody, rawBody)
	}
}

func TestExtractErrorCode(t *testing.T) {
	rawJSON := []byte(`{"queryError": "UNRECOGNIZED_FIELD"}`)
	code := extractErrorCode(rawJSON)
	if !strings.Contains(code, "queryError") {
		t.Errorf("extractErrorCode: got %q, expected to contain 'queryError'", code)
	}
	if !strings.Contains(code, "UNRECOGNIZED_FIELD") {
		t.Errorf("extractErrorCode: got %q, expected to contain 'UNRECOGNIZED_FIELD'", code)
	}
}
