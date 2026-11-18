package toktokdata

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// WebhookSignatureHeader carries sha256=<hex HMAC-SHA256> of an alert webhook.
const WebhookSignatureHeader = "X-Toktokdata-Signature"

// WebhookTimestampHeader carries the Unix time (seconds) the delivery was
// signed at.
const WebhookTimestampHeader = "X-Toktokdata-Timestamp"

// DefaultWebhookTolerance is the maximum age (and clock skew) of a webhook
// timestamp accepted by VerifyWebhook when tolerance is 0.
const DefaultWebhookTolerance = 300 * time.Second

// Codes of a *WebhookError, the same in every toktokdata SDK.
const (
	WebhookMissingHeader    = "webhook_missing_header"
	WebhookInvalidTimestamp = "webhook_invalid_timestamp"
	WebhookTimestampExpired = "webhook_timestamp_expired"
	WebhookInvalidSignature = "webhook_invalid_signature"
	WebhookInvalidJSON      = "webhook_invalid_json"
)

// WebhookError is returned by VerifyWebhook when a delivery must be rejected.
// Read it with errors.As and switch on Code.
type WebhookError struct {
	// Code is WebhookMissingHeader, WebhookInvalidTimestamp,
	// WebhookTimestampExpired, WebhookInvalidSignature or WebhookInvalidJSON.
	Code string

	// Message describes the failure.
	Message string

	// Err is the underlying error (the JSON error for WebhookInvalidJSON), or
	// nil.
	Err error
}

// Error implements the error interface.
func (e *WebhookError) Error() string {
	return "toktokdata: " + e.Code + ": " + e.Message
}

// Unwrap returns the underlying error, if any.
func (e *WebhookError) Unwrap() error {
	return e.Err
}

// VerifyWebhook checks an alert webhook delivery and returns its parsed JSON
// payload. body must be the raw request body, exactly as received; secret is
// the whsec_... value shown when the webhook channel was created. It checks
//
//	x-toktokdata-signature: sha256=<hex HMAC-SHA256(secret, "<x-toktokdata-timestamp>.<body>")>
//
// with a constant-time compare, and rejects timestamps further than
// tolerance from now (0 means DefaultWebhookTolerance, 5 minutes). A rejected
// delivery returns a *WebhookError; an empty secret returns a plain error.
func VerifyWebhook(body []byte, header http.Header, secret string, tolerance time.Duration) (map[string]any, error) {
	return verifyWebhookAt(body, header, secret, tolerance, time.Now())
}

func verifyWebhookAt(body []byte, header http.Header, secret string, tolerance time.Duration, now time.Time) (map[string]any, error) {
	if secret == "" {
		return nil, errors.New("toktokdata: webhook secret is empty")
	}
	if tolerance <= 0 {
		tolerance = DefaultWebhookTolerance
	}
	signature := strings.TrimSpace(headerValue(header, WebhookSignatureHeader))
	timestamp := strings.TrimSpace(headerValue(header, WebhookTimestampHeader))
	if signature == "" {
		return nil, &WebhookError{Code: WebhookMissingHeader, Message: "missing x-toktokdata-signature header"}
	}
	if timestamp == "" {
		return nil, &WebhookError{Code: WebhookMissingHeader, Message: "missing x-toktokdata-timestamp header"}
	}
	if !isDigits(timestamp) {
		return nil, &WebhookError{Code: WebhookInvalidTimestamp, Message: "x-toktokdata-timestamp is not a Unix time in seconds"}
	}
	signedAt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, &WebhookError{Code: WebhookInvalidTimestamp, Message: "x-toktokdata-timestamp is not a Unix time in seconds", Err: err}
	}
	age := now.Unix() - signedAt
	if age < 0 {
		age = -age
	}
	if float64(age) > tolerance.Seconds() {
		return nil, &WebhookError{
			Code:    WebhookTimestampExpired,
			Message: "signed at " + timestamp + ", now " + strconv.FormatInt(now.Unix(), 10) + ": outside the tolerance",
		}
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return nil, &WebhookError{Code: WebhookInvalidSignature, Message: "x-toktokdata-signature does not match the body"}
	}

	var payload map[string]any
	if jerr := json.Unmarshal(body, &payload); jerr != nil {
		return nil, &WebhookError{Code: WebhookInvalidJSON, Message: "the body is not a JSON object", Err: jerr}
	}
	if payload == nil {
		return nil, &WebhookError{Code: WebhookInvalidJSON, Message: "the body is not a JSON object"}
	}
	return payload, nil
}

// headerValue reads a header whatever the case of its key, so a plain
// map[string][]string built by hand works too.
func headerValue(header http.Header, name string) string {
	if header == nil {
		return ""
	}
	if v := header.Get(name); v != "" {
		return v
	}
	for key, values := range header {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func isDigits(s string) bool {
	if s == "" || len(s) > 18 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
