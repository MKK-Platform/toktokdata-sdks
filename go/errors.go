package toktokdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrMissingAPIKey is returned by NewClient when no API key was given and
// TOKTOKDATA_API_KEY is empty. It is an *APIError with Status 0 and Code
// "missing_api_key"; compare it with errors.Is.
var ErrMissingAPIKey = &APIError{
	Code:    "missing_api_key",
	Message: "no API key: pass toktokdata.WithAPIKey(key) or set TOKTOKDATA_API_KEY",
}

// ErrPendingTimeout matches (with errors.Is) the error returned when a
// 202-aware read or Exports.WaitUntilReady is still not ready after its
// timeout. The concrete error is a *PendingTimeoutError.
var ErrPendingTimeout = errors.New("toktokdata: still pending when the wait timed out")

// APIError is returned for every non-2xx answer of the API (and, with Status
// 0, for ErrMissingAPIKey). Use errors.As to read it, or the helpers
// IsRateLimited, IsPaymentRequired, IsForbidden, IsUnauthorized and
// IsNotFound.
type APIError struct {
	// Status is the HTTP status code, or 0 for an error raised before any
	// request (ErrMissingAPIKey).
	Status int

	// Code is the machine-readable error code of the envelope, for example
	// "credit_cap_exceeded" or "insufficient_scope". It is "http_error" when
	// the body was not a JSON error envelope.
	Code string

	// Message is the human-readable message of the envelope, or the HTTP
	// status text.
	Message string

	// RequestID identifies the request in support conversations.
	RequestID string

	// Retryable is the envelope's retryable flag. Without an envelope it is
	// true for 429 and 5xx.
	Retryable bool

	// RetryAfter is the Retry-After header of the answer, or 0 when absent.
	RetryAfter time.Duration

	// Details is the whole decoded JSON envelope (it may carry extra fields
	// such as resetAt), or nil when the body was not a JSON object.
	Details map[string]any

	// Body is the raw response body.
	Body []byte

	hasRetryable bool
}

// Error implements the error interface. The message never contains the API key.
func (e *APIError) Error() string {
	if e.Status == 0 {
		return "toktokdata: " + e.Code + ": " + e.Message
	}
	msg := fmt.Sprintf("toktokdata: HTTP %d %s: %s", e.Status, e.Code, e.Message)
	if e.RequestID != "" {
		msg += " (request " + e.RequestID + ")"
	}
	return msg
}

// AsAPIError returns the *APIError wrapped in err, if any.
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return apiErr, true
	}
	return nil, false
}

func hasStatus(err error, status int) bool {
	apiErr, ok := AsAPIError(err)
	return ok && apiErr.Status == status
}

// IsRateLimited reports whether err is a 429 answer (per-second, per-minute
// or daily limit; check Code for "daily_quota_exceeded").
func IsRateLimited(err error) bool {
	return hasStatus(err, http.StatusTooManyRequests)
}

// IsPaymentRequired reports whether err is a 402 answer (any code), for
// example "credit_cap_exceeded": the workspace's credit wallet cannot cover
// the call (the envelope's resetAt, in Details, says when credits reset).
func IsPaymentRequired(err error) bool {
	return hasStatus(err, http.StatusPaymentRequired)
}

// IsForbidden reports whether err is a 403 answer: the key lacks the scope
// ("insufficient_scope") or the plan lacks the feature ("module_not_entitled").
func IsForbidden(err error) bool {
	return hasStatus(err, http.StatusForbidden)
}

// IsUnauthorized reports whether err is a 401 answer: missing, unknown,
// revoked or expired key.
func IsUnauthorized(err error) bool {
	return hasStatus(err, http.StatusUnauthorized)
}

// IsNotFound reports whether err is a 404 answer.
func IsNotFound(err error) bool {
	return hasStatus(err, http.StatusNotFound)
}

// PendingTimeoutError is returned when a 202-aware read (or
// Exports.WaitUntilReady) was still pending after its timeout. It matches
// ErrPendingTimeout with errors.Is.
type PendingTimeoutError struct {
	// Path is the API path that was polled.
	Path string

	// Timeout is the wait budget that ran out.
	Timeout time.Duration

	// Last is the last pending response (its body holds status, jobId and
	// retryAfterSeconds, or the export job).
	Last *Response
}

// Error implements the error interface.
func (e *PendingTimeoutError) Error() string {
	return fmt.Sprintf("toktokdata: %s was still pending after %s", e.Path, e.Timeout)
}

// Unwrap lets errors.Is(err, ErrPendingTimeout) match.
func (e *PendingTimeoutError) Unwrap() error {
	return ErrPendingTimeout
}

// newAPIError maps a non-2xx answer to an *APIError.
func newAPIError(status int, header http.Header, body []byte, now time.Time) *APIError {
	e := &APIError{Status: status, Body: body}
	if d, ok := parseRetryAfter(header.Get("Retry-After"), now); ok {
		e.RetryAfter = d
	}
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err == nil && envelope != nil {
		e.Details = envelope
		e.Code, _ = envelope["code"].(string)
		e.Message, _ = envelope["message"].(string)
		e.RequestID, _ = envelope["requestId"].(string)
		if retryable, ok := envelope["retryable"].(bool); ok {
			e.Retryable = retryable
			e.hasRetryable = true
		}
		if e.Message == "" {
			e.Message, _ = envelope["detail"].(string)
		}
	}
	if e.Code == "" {
		e.Code = "http_error"
	}
	if e.Message == "" {
		e.Message = http.StatusText(status)
	}
	if e.Message == "" {
		e.Message = "HTTP " + strconv.Itoa(status)
	}
	if e.RequestID == "" {
		e.RequestID = header.Get("X-Request-Id")
	}
	if !e.hasRetryable {
		e.Retryable = status == http.StatusTooManyRequests || status >= 500
	}
	return e
}

// parseRetryAfter reads a Retry-After value given in seconds or as an HTTP date.
func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(value); err == nil {
		if secs < 0 {
			return 0, false
		}
		if secs > 86400 {
			secs = 86400
		}
		return time.Duration(secs) * time.Second, true
	}
	if at, err := http.ParseTime(value); err == nil {
		d := at.Sub(now)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}
