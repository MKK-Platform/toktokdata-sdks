package toktokdata

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"
)

const (
	webhookSecret = "whsec_test"
	webhookTS     = "1763467200"
	webhookBody   = `{"alertId":"rule-1:event-1","eventType":"gift"}`
)

// webhookSig is HMAC-SHA256("whsec_test", "1763467200.<webhookBody>"),
// computed independently with openssl.
const webhookSig = "sha256=03f2d21e56151aee29abd1ee3894a5c32862e85cec93d5b37cb79600d32ba618"

var webhookNow = time.Unix(1763467200, 0)

func signWebhook(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func webhookHeader(signature, timestamp string) http.Header {
	h := http.Header{}
	if signature != "" {
		h.Set("x-toktokdata-signature", signature)
	}
	if timestamp != "" {
		h.Set("x-toktokdata-timestamp", timestamp)
	}
	return h
}

// webhookCode returns the Code of a *WebhookError, or "" for any other error.
func webhookCode(err error) string {
	var werr *WebhookError
	if errors.As(err, &werr) {
		return werr.Code
	}
	return ""
}

func TestVerifyWebhookKnownVector(t *testing.T) {
	if got := signWebhook(webhookSecret, webhookTS, []byte(webhookBody)); got != webhookSig {
		t.Fatalf("test helper signature = %s", got)
	}
	payload, err := verifyWebhookAt([]byte(webhookBody), webhookHeader(webhookSig, webhookTS), webhookSecret, 0, webhookNow)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if payload["alertId"] != "rule-1:event-1" || payload["eventType"] != "gift" {
		t.Fatalf("payload = %v", payload)
	}
}

func TestVerifyWebhookLowercaseHeaderMap(t *testing.T) {
	h := http.Header{
		"x-toktokdata-signature": {webhookSig},
		"x-toktokdata-timestamp": {webhookTS},
	}
	if _, err := verifyWebhookAt([]byte(webhookBody), h, webhookSecret, 0, webhookNow); err != nil {
		t.Fatalf("verify with non-canonical keys: %v", err)
	}
}

func TestVerifyWebhookRejects(t *testing.T) {
	body := []byte(webhookBody)
	cases := []struct {
		name     string
		body     []byte
		header   http.Header
		secret   string
		now      time.Time
		wantCode string
	}{
		{"wrong secret", body, webhookHeader(webhookSig, webhookTS), "whsec_other", webhookNow, WebhookInvalidSignature},
		{"tampered body", []byte(`{"alertId":"rule-1:event-2","eventType":"gift"}`), webhookHeader(webhookSig, webhookTS), webhookSecret, webhookNow, WebhookInvalidSignature},
		{"bare hex signature", body, webhookHeader(webhookSig[len("sha256="):], webhookTS), webhookSecret, webhookNow, WebhookInvalidSignature},
		{"missing signature", body, webhookHeader("", webhookTS), webhookSecret, webhookNow, WebhookMissingHeader},
		{"missing timestamp", body, webhookHeader(webhookSig, ""), webhookSecret, webhookNow, WebhookMissingHeader},
		{"no headers", body, nil, webhookSecret, webhookNow, WebhookMissingHeader},
		{"malformed timestamp", body, webhookHeader(webhookSig, "+1763467200"), webhookSecret, webhookNow, WebhookInvalidTimestamp},
		{"expired", body, webhookHeader(webhookSig, webhookTS), webhookSecret, webhookNow.Add(301 * time.Second), WebhookTimestampExpired},
		{"from the future", body, webhookHeader(webhookSig, webhookTS), webhookSecret, webhookNow.Add(-301 * time.Second), WebhookTimestampExpired},
	}
	for _, tc := range cases {
		payload, err := verifyWebhookAt(tc.body, tc.header, tc.secret, 0, tc.now)
		if got := webhookCode(err); got != tc.wantCode {
			t.Errorf("%s: err = %v (code %q), want code %q", tc.name, err, got, tc.wantCode)
		}
		if payload != nil {
			t.Errorf("%s: payload must be nil on error", tc.name)
		}
	}
}

func TestVerifyWebhookTolerance(t *testing.T) {
	header := webhookHeader(webhookSig, webhookTS)
	body := []byte(webhookBody)
	if _, err := verifyWebhookAt(body, header, webhookSecret, 0, webhookNow.Add(299*time.Second)); err != nil {
		t.Fatalf("within the default tolerance: %v", err)
	}
	if _, err := verifyWebhookAt(body, header, webhookSecret, 10*time.Minute, webhookNow.Add(9*time.Minute)); err != nil {
		t.Fatalf("within a custom tolerance: %v", err)
	}
	_, err := verifyWebhookAt(body, header, webhookSecret, 10*time.Second, webhookNow.Add(11*time.Second))
	if webhookCode(err) != WebhookTimestampExpired {
		t.Fatalf("outside a custom tolerance: %v", err)
	}
}

func TestVerifyWebhookInvalidJSON(t *testing.T) {
	for _, raw := range []string{"not json", "[1,2]", "null"} {
		body := []byte(raw)
		header := webhookHeader(signWebhook(webhookSecret, webhookTS, body), webhookTS)
		payload, err := verifyWebhookAt(body, header, webhookSecret, 0, webhookNow)
		if webhookCode(err) != WebhookInvalidJSON || payload != nil {
			t.Errorf("body %q: err = %v, want code %s", raw, err, WebhookInvalidJSON)
		}
	}
}

func TestVerifyWebhookEmptySecret(t *testing.T) {
	_, err := verifyWebhookAt([]byte(webhookBody), webhookHeader(webhookSig, webhookTS), "", 0, webhookNow)
	if err == nil || webhookCode(err) != "" {
		t.Fatalf("an empty secret must fail with a plain error: %v", err)
	}
}

func TestVerifyWebhookUsesCurrentTime(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(`{"alertId":"rule-1:event-3"}`)
	header := webhookHeader(signWebhook(webhookSecret, ts, body), ts)
	payload, err := VerifyWebhook(body, header, webhookSecret, 0)
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if payload["alertId"] != "rule-1:event-3" {
		t.Fatalf("payload = %v", payload)
	}
	_, err = VerifyWebhook(body, webhookHeader(webhookSig, webhookTS), webhookSecret, 0)
	if webhookCode(err) != WebhookTimestampExpired {
		t.Fatalf("an old delivery must be rejected: %v", err)
	}
}
