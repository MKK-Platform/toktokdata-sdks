package toktokdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testKey = "ttk_live_test_secret"

// call is one request seen by the fake API.
type call struct {
	method string
	path   string
	query  string
	header http.Header
	body   string
}

// fakeAPI is an httptest server that records requests and answers through
// handler; n is the 1-based number of the request.
type fakeAPI struct {
	mu      sync.Mutex
	calls   []call
	handler func(n int, w http.ResponseWriter, r *http.Request)
	server  *httptest.Server
}

func newFakeAPI(t *testing.T, handler func(n int, w http.ResponseWriter, r *http.Request)) *fakeAPI {
	t.Helper()
	f := &fakeAPI{handler: handler}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.calls = append(f.calls, call{
			method: r.Method,
			path:   r.URL.EscapedPath(),
			query:  r.URL.RawQuery,
			header: r.Header.Clone(),
			body:   string(body),
		})
		n := len(f.calls)
		f.mu.Unlock()
		f.handler(n, w, r)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeAPI) Calls() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]call, len(f.calls))
	copy(out, f.calls)
	return out
}

// fakeClock replaces the client's clock and sleep: sleeping only records the
// duration and moves the clock forward, so retry tests run instantly.
type fakeClock struct {
	now    time.Time
	sleeps []time.Duration
}

func newTestClient(t *testing.T, f *fakeAPI, opts ...Option) (*Client, *fakeClock) {
	t.Helper()
	all := []Option{WithAPIKey(testKey), WithBaseURL(f.server.URL)}
	all = append(all, opts...)
	c, err := NewClient(all...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	clock := &fakeClock{now: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)}
	c.now = func() time.Time {
		return clock.now
	}
	c.sleep = func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		clock.sleeps = append(clock.sleeps, d)
		clock.now = clock.now.Add(d)
		return nil
	}
	c.jitter = func() float64 {
		return 0.5
	}
	return c, clock
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func okData(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, `{"data":{"ok":true},"provenance":{"sourceKind":"derived"}}`)
}

func assertSleeps(t *testing.T, clock *fakeClock, want ...time.Duration) {
	t.Helper()
	if len(clock.sleeps) != len(want) {
		t.Fatalf("sleeps = %v, want %v", clock.sleeps, want)
	}
	for i := range want {
		if clock.sleeps[i] != want[i] {
			t.Fatalf("sleeps = %v, want %v", clock.sleeps, want)
		}
	}
}

func assertCalls(t *testing.T, f *fakeAPI, want int) []call {
	t.Helper()
	calls := f.Calls()
	if len(calls) != want {
		t.Fatalf("server saw %d requests, want %d", len(calls), want)
	}
	return calls
}

func TestNewClientRequiresAPIKey(t *testing.T) {
	t.Setenv("TOKTOKDATA_API_KEY", "")
	c, err := NewClient()
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("err = %v, want ErrMissingAPIKey", err)
	}
	if c != nil {
		t.Fatal("client should be nil without a key")
	}
	if !strings.Contains(err.Error(), "TOKTOKDATA_API_KEY") {
		t.Fatalf("error should name the env variable: %v", err)
	}
	if apiErr, ok := AsAPIError(err); !ok || apiErr.Code != "missing_api_key" || apiErr.Status != 0 {
		t.Fatalf("err = %#v, want code missing_api_key", err)
	}
}

func TestNewClientRejectsInvalidBaseURL(t *testing.T) {
	for _, base := range []string{"ftp://example.com", "not a url", "/v1"} {
		if _, err := NewClient(WithAPIKey(testKey), WithBaseURL(base)); err == nil {
			t.Errorf("base URL %q: want an error", base)
		}
	}
}

func TestNewClientReadsEnvironment(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		okData(w)
	})
	t.Setenv("TOKTOKDATA_API_KEY", "ttk_live_env_key")
	t.Setenv("TOKTOKDATA_BASE_URL", f.server.URL+"/")
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.Rankings.Regions(context.Background()); err != nil {
		t.Fatalf("Regions: %v", err)
	}
	calls := assertCalls(t, f, 1)
	if got := calls[0].header.Get("Authorization"); got != "Bearer ttk_live_env_key" {
		t.Fatalf("Authorization = %q", got)
	}
	if calls[0].path != "/v1/rankings/regions" {
		t.Fatalf("path = %q", calls[0].path)
	}
}

func TestRequestHeaders(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		okData(w)
	})
	c, _ := newTestClient(t, f)
	resp, err := c.Rankings.Regions(context.Background())
	if err != nil {
		t.Fatalf("Regions: %v", err)
	}
	h := assertCalls(t, f, 1)[0].header
	if got := h.Get("Authorization"); got != "Bearer "+testKey {
		t.Errorf("Authorization = %q", got)
	}
	if got := h.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
	if got := h.Get("User-Agent"); got != "toktokdata-go/"+Version {
		t.Errorf("User-Agent = %q", got)
	}
	if got := h.Get("Content-Type"); got != "" {
		t.Errorf("Content-Type on a GET = %q, want none", got)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d", resp.StatusCode)
	}
	var data struct {
		OK bool `json:"ok"`
	}
	if err := resp.Decode(&data); err != nil || !data.OK {
		t.Errorf("Decode = %v, %+v", err, data)
	}
	if resp.Provenance["sourceKind"] != "derived" {
		t.Errorf("Provenance = %v", resp.Provenance)
	}
}

func TestQueryEncoding(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		okData(w)
	})
	c, _ := newTestClient(t, f)
	ctx := context.Background()

	if _, err := c.Rankings.Official(ctx, "hourly", &RankingsOfficialParams{Region: "VN", Limit: 50, LiveOnly: Bool(false)}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Agencies.List(ctx, &AgenciesListParams{Q: "star agency", PublicOnly: Bool(true), Offset: 20}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Live.Compare(ctx, []string{"s1", "s2", "s3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Gifts.List(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Creators.Changes(ctx, "@linh.live", &CreatorChangesParams{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Videos.Get(ctx, "a/b c"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Alerts.Deliveries(ctx, &AlertDeliveriesParams{RuleID: "r1", Status: "failed", Since: "2026-11-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"/v1/rankings/official?board=hourly&limit=50&liveOnly=false&region=VN",
		"/v1/agencies?offset=20&publicOnly=true&q=star+agency",
		"/v1/live/sessions/compare?ids=s1%2Cs2%2Cs3",
		"/v1/gifts",
		"/v1/creators/linh.live/changes",
		"/v1/videos/a%2Fb%20c",
		"/v1/alerts/deliveries?ruleId=r1&since=2026-11-01T00%3A00%3A00Z&status=failed",
	}
	calls := assertCalls(t, f, len(want))
	for i, wantURL := range want {
		got := calls[i].path
		if calls[i].query != "" {
			got += "?" + calls[i].query
		}
		if got != wantURL {
			t.Errorf("request %d = %q, want %q", i, got, wantURL)
		}
	}
}

func TestRequiredParameters(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		okData(w)
	})
	c, _ := newTestClient(t, f)
	ctx := context.Background()
	checks := map[string]error{}
	_, checks["Rankings.Official"] = c.Rankings.Official(ctx, "", nil)
	_, checks["Rankings.History"] = c.Rankings.History(ctx, " ", nil)
	_, checks["Creators.Get"] = c.Creators.Get(ctx, "", nil)
	_, checks["Creators.Get(@)"] = c.Creators.Get(ctx, "@", nil)
	_, checks["Live.Compare(nil)"] = c.Live.Compare(ctx, nil)
	_, checks["Live.Compare(1)"] = c.Live.Compare(ctx, []string{"s1"})
	_, checks["Live.Compare(5)"] = c.Live.Compare(ctx, []string{"s1", "s2", "s3", "s4", "s5"})
	_, checks["Live.Compare(empty id)"] = c.Live.Compare(ctx, []string{"s1", ""})
	_, checks["Exports.Create(nil)"] = c.Exports.Create(ctx, nil)
	_, checks["Exports.Create(no format)"] = c.Exports.Create(ctx, &ExportCreateParams{Dataset: "rankings"})
	_, checks["Alerts.CreateRule"] = c.Alerts.CreateRule(ctx, nil)
	checks["Alerts.DeleteRule"] = c.Alerts.DeleteRule(ctx, "")
	for name, err := range checks {
		if err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	assertCalls(t, f, 0)
}

func TestErrorMappingJSONEnvelope(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusForbidden, `{"code":"module_not_entitled","message":"The plan does not include gifters.","requestId":"req-123","retryable":false}`)
	})
	c, clock := newTestClient(t, f)
	_, err := c.Gifters.List(context.Background(), nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Status != 403 || apiErr.Code != "module_not_entitled" || apiErr.RequestID != "req-123" || apiErr.Retryable {
		t.Fatalf("unexpected error fields: %+v", apiErr)
	}
	if apiErr.Message != "The plan does not include gifters." {
		t.Fatalf("Message = %q", apiErr.Message)
	}
	if apiErr.Details["code"] != "module_not_entitled" {
		t.Fatalf("Details = %v", apiErr.Details)
	}
	if !IsForbidden(err) || IsRateLimited(err) || IsNotFound(err) || IsUnauthorized(err) || IsPaymentRequired(err) {
		t.Fatal("helpers disagree with status 403")
	}
	if msg := err.Error(); !strings.Contains(msg, "module_not_entitled") || !strings.Contains(msg, "req-123") || strings.Contains(msg, testKey) {
		t.Fatalf("Error() = %q", msg)
	}
	assertCalls(t, f, 1)
	assertSleeps(t, clock)
}

func TestErrorMappingNonJSONBody(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "<html>bad gateway</html>")
	})
	c, _ := newTestClient(t, f, WithMaxRetries(0))
	_, err := c.Trends.List(context.Background(), nil)
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Status != http.StatusBadGateway || apiErr.Code != "http_error" || apiErr.Message != "Bad Gateway" {
		t.Fatalf("unexpected error fields: %+v", apiErr)
	}
	if !apiErr.Retryable || apiErr.Details != nil {
		t.Fatalf("Retryable = %v, Details = %v", apiErr.Retryable, apiErr.Details)
	}
	if string(apiErr.Body) != "<html>bad gateway</html>" {
		t.Fatalf("Body = %q", apiErr.Body)
	}
}

func TestErrorHelpers(t *testing.T) {
	cases := []struct {
		status int
		body   string
		check  func(error) bool
	}{
		{401, `{"code":"unauthorized","message":"Missing API key.","retryable":false}`, IsUnauthorized},
		{402, `{"code":"credit_cap_exceeded","message":"Out of credits.","retryable":false,"resetAt":"2026-12-01T00:00:00Z"}`, IsPaymentRequired},
		{403, `{"code":"insufficient_scope","message":"Needs rank:read.","retryable":false}`, IsForbidden},
		{404, `{"code":"not_found","message":"No such creator.","retryable":false}`, IsNotFound},
		{429, `{"code":"rate_limited","message":"Slow down.","retryable":true}`, IsRateLimited},
	}
	for _, tc := range cases {
		status, body := tc.status, tc.body
		f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
			writeJSON(w, status, body)
		})
		c, _ := newTestClient(t, f, WithMaxRetries(0))
		_, err := c.Creators.Search(context.Background(), &CreatorsSearchParams{Q: "linh"})
		if !tc.check(err) {
			t.Errorf("status %d: helper returned false for %v", tc.status, err)
		}
		if !tc.check(fmt.Errorf("wrapped: %w", err)) {
			t.Errorf("status %d: helper does not see through wrapping", tc.status)
		}
		if tc.status == 402 {
			apiErr, _ := AsAPIError(err)
			if apiErr == nil || apiErr.Details["resetAt"] != "2026-12-01T00:00:00Z" {
				t.Errorf("402 Details = %+v", apiErr)
			}
		}
	}
	if IsNotFound(nil) || IsNotFound(errors.New("plain")) {
		t.Error("helpers must be false for nil and non-API errors")
	}
}

func TestRetryOn429HonoursRetryAfter(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 1 {
			w.Header().Set("Retry-After", "3")
			writeJSON(w, http.StatusTooManyRequests, `{"code":"rate_limited","message":"Too many requests.","retryable":true}`)
			return
		}
		okData(w)
	})
	c, clock := newTestClient(t, f)
	resp, err := c.Rankings.Regions(context.Background())
	if err != nil {
		t.Fatalf("Regions: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d", resp.StatusCode)
	}
	assertCalls(t, f, 2)
	assertSleeps(t, clock, 3*time.Second)
}

func TestRetryAfterIsCapped(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 1 {
			w.Header().Set("Retry-After", "600")
			writeJSON(w, http.StatusTooManyRequests, `{"code":"minute_rate_limit_exceeded","message":"Slow down.","retryable":true}`)
			return
		}
		okData(w)
	})
	c, clock := newTestClient(t, f)
	if _, err := c.Rankings.Regions(context.Background()); err != nil {
		t.Fatalf("Regions: %v", err)
	}
	assertSleeps(t, clock, 60*time.Second)
}

func TestRetryBackoffWithoutRetryAfter(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		switch n {
		case 1:
			writeJSON(w, http.StatusServiceUnavailable, `{"code":"upstream_unavailable","message":"Try again.","retryable":true}`)
		case 2:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			okData(w)
		}
	})
	c, clock := newTestClient(t, f)
	if _, err := c.Insights.BestTime(context.Background(), nil); err != nil {
		t.Fatalf("BestTime: %v", err)
	}
	assertCalls(t, f, 3)
	assertSleeps(t, clock, 500*time.Millisecond, time.Second)
}

func TestRetriesExhausted(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusTooManyRequests, `{"code":"rate_limited","message":"Slow down.","retryable":true}`)
	})
	c, clock := newTestClient(t, f)
	_, err := c.Rankings.Regions(context.Background())
	if !IsRateLimited(err) {
		t.Fatalf("err = %v, want a 429", err)
	}
	apiErr, _ := AsAPIError(err)
	if apiErr.RetryAfter != time.Second {
		t.Fatalf("RetryAfter = %v", apiErr.RetryAfter)
	}
	assertCalls(t, f, 3)
	assertSleeps(t, clock, time.Second, time.Second)
}

func TestNoRetry(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"402 credit cap", 402, `{"code":"credit_cap_exceeded","message":"Out of credits.","retryable":false}`},
		{"429 daily quota", 429, `{"code":"daily_quota_exceeded","message":"Daily quota used.","retryable":false}`},
		{"500 not retryable", 500, `{"code":"internal_error","message":"Bug.","retryable":false}`},
		{"400 validation", 400, `{"code":"validation_error","message":"limit must be an integer.","retryable":false}`},
	}
	for _, tc := range cases {
		status, body := tc.status, tc.body
		f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "1")
			writeJSON(w, status, body)
		})
		c, clock := newTestClient(t, f)
		_, err := c.Rankings.Regions(context.Background())
		apiErr, ok := AsAPIError(err)
		if !ok || apiErr.Status != tc.status {
			t.Errorf("%s: err = %v", tc.name, err)
			continue
		}
		if got := len(f.Calls()); got != 1 {
			t.Errorf("%s: %d requests, want 1", tc.name, got)
		}
		if len(clock.sleeps) != 0 {
			t.Errorf("%s: slept %v", tc.name, clock.sleeps)
		}
	}
}

func TestPostIsRetriedOnlyOn429(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusServiceUnavailable, `{"code":"upstream_unavailable","message":"Try again.","retryable":true}`)
	})
	c, clock := newTestClient(t, f)
	if _, err := c.Live.Refresh(context.Background(), "linh.live"); err == nil {
		t.Fatal("want an error")
	}
	assertCalls(t, f, 1)
	assertSleeps(t, clock)

	g := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 1 {
			w.Header().Set("Retry-After", "2")
			writeJSON(w, http.StatusTooManyRequests, `{"code":"rate_limited","message":"Slow down.","retryable":true}`)
			return
		}
		writeJSON(w, http.StatusAccepted, `{"status":"queued"}`)
	})
	c2, clock2 := newTestClient(t, g)
	resp, err := c2.Live.Refresh(context.Background(), "linh.live")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d", resp.StatusCode)
	}
	calls := assertCalls(t, g, 2)
	if calls[0].method != http.MethodPost || calls[0].path != "/v1/live/creators/linh.live/refresh" {
		t.Fatalf("request = %s %s", calls[0].method, calls[0].path)
	}
	if calls[0].body != "" || calls[0].header.Get("Content-Type") != "" {
		t.Fatalf("a POST without body sent %q with Content-Type %q", calls[0].body, calls[0].header.Get("Content-Type"))
	}
	assertSleeps(t, clock2, 2*time.Second)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestNetworkErrorRetriedForGETOnly(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		okData(w)
	})
	var attempts atomic.Int32
	flaky := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if attempts.Add(1) == 1 {
			return nil, errors.New("connection reset by peer")
		}
		return http.DefaultTransport.RoundTrip(r)
	})
	c, clock := newTestClient(t, f, WithHTTPClient(&http.Client{Transport: flaky}))
	if _, err := c.Rankings.Regions(context.Background()); err != nil {
		t.Fatalf("Regions: %v", err)
	}
	assertCalls(t, f, 1)
	assertSleeps(t, clock, 500*time.Millisecond)

	broken := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection reset by peer")
	})
	c2, clock2 := newTestClient(t, f, WithHTTPClient(&http.Client{Transport: broken}))
	_, err := c2.Exports.Create(context.Background(), &ExportCreateParams{Dataset: "rankings", Format: "csv"})
	if err == nil {
		t.Fatal("want a network error")
	}
	if _, ok := AsAPIError(err); ok {
		t.Fatalf("a network error must not be an *APIError: %v", err)
	}
	assertSleeps(t, clock2)
}

func TestPerRequestTimeout(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		okData(w)
	})
	c, _ := newTestClient(t, f, WithTimeout(50*time.Millisecond), WithMaxRetries(0))
	_, err := c.Rankings.Regions(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestPendingWaitLoop(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n <= 2 {
			w.Header().Set("Retry-After", "5")
			writeJSON(w, http.StatusAccepted, `{"status":"resolving","jobId":"job-1","handle":"linh.live","retryAfterSeconds":5}`)
			return
		}
		writeJSON(w, http.StatusOK, `{"data":{"creator":{"uniqueId":"linh.live"}},"provenance":{"sourceKind":"derived"}}`)
	})
	c, clock := newTestClient(t, f)
	resp, err := c.Creators.Get(context.Background(), "linh.live", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != http.StatusOK || resp.IsPending() {
		t.Fatalf("StatusCode = %d", resp.StatusCode)
	}
	var data struct {
		Creator struct {
			UniqueID string `json:"uniqueId"`
		} `json:"creator"`
	}
	if err := resp.Decode(&data); err != nil || data.Creator.UniqueID != "linh.live" {
		t.Fatalf("Decode = %v, %+v", err, data)
	}
	if resp.Provenance["sourceKind"] != "derived" {
		t.Fatalf("Provenance = %v", resp.Provenance)
	}
	calls := assertCalls(t, f, 3)
	if calls[2].path != "/v1/creators/linh.live" {
		t.Fatalf("path = %q", calls[2].path)
	}
	assertSleeps(t, clock, 5*time.Second, 5*time.Second)
}

func TestPendingDelayIsClamped(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		switch n {
		case 1:
			w.Header().Set("Retry-After", "0")
			writeJSON(w, http.StatusAccepted, `{"status":"saturated","handle":"linh.live"}`)
		case 2:
			writeJSON(w, http.StatusAccepted, `{"status":"collecting","jobId":"job-2","retryAfterSeconds":500}`)
		default:
			writeJSON(w, http.StatusOK, `{"data":{"entries":[],"nextCursor":null}}`)
		}
	})
	c, clock := newTestClient(t, f)
	if _, err := c.Creators.Followers(context.Background(), "linh.live", &CreatorFollowParams{Limit: 10}); err != nil {
		t.Fatalf("Followers: %v", err)
	}
	assertSleeps(t, clock, time.Second, 120*time.Second)
}

func TestPendingWithoutWaiting(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusAccepted, `{"status":"resolving","jobId":"job-9","retryAfterSeconds":3}`)
	})
	c, clock := newTestClient(t, f)
	resp, err := c.Live.Playback(context.Background(), "linh.live", &LivePlaybackParams{Wait: Bool(false)})
	if err != nil {
		t.Fatalf("Playback: %v", err)
	}
	if !resp.IsPending() {
		t.Fatalf("StatusCode = %d, want 202", resp.StatusCode)
	}
	info := resp.PendingInfo()
	if info == nil || info.JobID != "job-9" || info.Status != "resolving" || info.RetryAfterSeconds != 3 {
		t.Fatalf("PendingInfo = %+v", info)
	}
	assertCalls(t, f, 1)
	assertSleeps(t, clock)
}

func TestPendingTimeout(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusAccepted, `{"status":"collecting","jobId":"job-3","retryAfterSeconds":60}`)
	})
	c, clock := newTestClient(t, f)
	resp, err := c.Videos.Comments(context.Background(), "7300000000000000001", nil)
	if resp != nil {
		t.Fatalf("resp = %+v, want nil", resp)
	}
	if !errors.Is(err, ErrPendingTimeout) {
		t.Fatalf("err = %v, want ErrPendingTimeout", err)
	}
	var pending *PendingTimeoutError
	if !errors.As(err, &pending) {
		t.Fatalf("err = %T, want *PendingTimeoutError", err)
	}
	if pending.Path != "/v1/videos/7300000000000000001/comments" || pending.Timeout != DefaultPendingTimeout {
		t.Fatalf("PendingTimeoutError = %+v", pending)
	}
	if pending.Last == nil || pending.Last.PendingInfo().JobID != "job-3" {
		t.Fatalf("Last = %+v", pending.Last)
	}
	assertCalls(t, f, 4)
	assertSleeps(t, clock, time.Minute, time.Minute, time.Minute)
}

func TestPendingTimeoutOption(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusAccepted, `{"status":"resolving","retryAfterSeconds":5}`)
	})
	c, clock := newTestClient(t, f, WithPendingTimeout(10*time.Second))
	_, err := c.Creators.Following(context.Background(), "linh.live", nil)
	if !errors.Is(err, ErrPendingTimeout) {
		t.Fatalf("err = %v, want ErrPendingTimeout", err)
	}
	assertCalls(t, f, 3)
	assertSleeps(t, clock, 5*time.Second, 5*time.Second)
}

func TestPagination(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "":
			writeJSON(w, http.StatusOK, `{"data":{"items":[{"id":"s1"},{"id":"s2"}],"nextCursor":"c2"},"provenance":{}}`)
		case "c2":
			writeJSON(w, http.StatusOK, `{"data":{"items":[{"id":"s3"},{"id":"s4"}],"nextCursor":"c3"},"provenance":{}}`)
		case "c3":
			writeJSON(w, http.StatusOK, `{"data":{"items":[{"id":"s5"}],"nextCursor":null},"provenance":{}}`)
		default:
			writeJSON(w, http.StatusBadRequest, `{"code":"invalid_cursor","message":"Bad cursor."}`)
		}
	})
	c, _ := newTestClient(t, f)
	params := &LiveSessionsParams{CreatorUniqueID: "linh.live", Limit: 2}
	it := c.Live.SessionsIter(context.Background(), params)
	var ids []string
	pages := 0
	for it.Next() {
		pages++
		for _, raw := range it.Page().Items() {
			var item struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &item); err != nil {
				t.Fatalf("item: %v", err)
			}
			ids = append(ids, item.ID)
		}
	}
	if err := it.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if pages != 3 || strings.Join(ids, ",") != "s1,s2,s3,s4,s5" {
		t.Fatalf("pages = %d, ids = %v", pages, ids)
	}
	if it.Next() {
		t.Fatal("Next after the last page must be false")
	}
	if params.Cursor != "" {
		t.Fatalf("the caller's params were changed: %+v", params)
	}
	calls := assertCalls(t, f, 3)
	wantQueries := []string{
		"creatorUniqueId=linh.live&limit=2",
		"creatorUniqueId=linh.live&cursor=c2&limit=2",
		"creatorUniqueId=linh.live&cursor=c3&limit=2",
	}
	for i, want := range wantQueries {
		if calls[i].query != want {
			t.Errorf("page %d query = %q, want %q", i+1, calls[i].query, want)
		}
	}
}

func TestPaginationStopsOnError(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 1 {
			writeJSON(w, http.StatusOK, `{"data":{"entries":[1,2],"nextCursor":"p2"}}`)
			return
		}
		writeJSON(w, http.StatusNotFound, `{"code":"not_found","message":"Gone."}`)
	})
	c, _ := newTestClient(t, f)
	it := c.Rankings.HistoryIter(context.Background(), "daily", &RankingsHistoryParams{Region: "VN"})
	pages := 0
	for it.Next() {
		pages++
	}
	if pages != 1 || !IsNotFound(it.Err()) || it.Page() != nil {
		t.Fatalf("pages = %d, err = %v", pages, it.Err())
	}
}

func TestPaginationStopsOnRepeatedCursor(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"data":{"items":[1],"nextCursor":"same"}}`)
	})
	c, _ := newTestClient(t, f)
	it := c.Live.PerformanceIter(context.Background(), "linh.live", nil)
	pages := 0
	for it.Next() {
		pages++
	}
	if pages != 2 || it.Err() != nil {
		t.Fatalf("pages = %d, err = %v", pages, it.Err())
	}
	assertCalls(t, f, 2)
}

func TestIteratorRoutes(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"data":{"comments":[{"id":"x"}],"nextCursor":null}}`)
	})
	c, _ := newTestClient(t, f)
	ctx := context.Background()
	iters := []*PageIterator{
		c.Creators.FollowersIter(ctx, "linh.live", nil),
		c.Creators.FollowingIter(ctx, "linh.live", &CreatorFollowParams{Cursor: "start"}),
		c.Videos.CommentsIter(ctx, "v1", &VideoCommentsParams{ParentCommentID: "c1"}),
		c.Rankings.HistoryIter(ctx, "daily", nil),
		c.Live.PerformanceIter(ctx, "linh.live", &LivePerformanceParams{Limit: 5}),
	}
	for i, it := range iters {
		pages := 0
		for it.Next() {
			pages++
			if len(it.Page().Items()) != 1 {
				t.Errorf("iterator %d: items = %v", i, it.Page().Items())
			}
		}
		if pages != 1 || it.Err() != nil {
			t.Errorf("iterator %d: pages = %d, err = %v", i, pages, it.Err())
		}
	}
	want := []string{
		"/v1/creators/linh.live/followers",
		"/v1/creators/linh.live/following?cursor=start",
		"/v1/videos/v1/comments?parentCommentId=c1",
		"/v1/rankings/history?board=daily",
		"/v1/live/creators/linh.live/performance?limit=5",
	}
	calls := assertCalls(t, f, len(want))
	for i, wantURL := range want {
		got := calls[i].path
		if calls[i].query != "" {
			got += "?" + calls[i].query
		}
		if got != wantURL {
			t.Errorf("request %d = %q, want %q", i, got, wantURL)
		}
	}
}

func TestContextCanceledBeforeRequest(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		okData(w)
	})
	c, _ := newTestClient(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Rankings.Regions(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	assertCalls(t, f, 0)
}

func TestContextCanceledDuringRetryWait(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		writeJSON(w, http.StatusTooManyRequests, `{"code":"rate_limited","message":"Slow down.","retryable":true}`)
	})
	c, _ := newTestClient(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.sleep = func(sleepCtx context.Context, d time.Duration) error {
		cancel()
		return sleepContext(sleepCtx, d)
	}
	start := time.Now()
	_, err := c.Rankings.Regions(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("the retry wait ignored the cancellation")
	}
	assertCalls(t, f, 1)
}

func TestContextCanceledDuringPendingWait(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusAccepted, `{"status":"resolving","retryAfterSeconds":30}`)
	})
	c, _ := newTestClient(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.sleep = func(sleepCtx context.Context, d time.Duration) error {
		cancel()
		return sleepContext(sleepCtx, d)
	}
	_, err := c.Creators.Get(ctx, "linh.live", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	assertCalls(t, f, 1)
}

func TestSleepContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := sleepContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("sleepContext ignored the cancellation")
	}
	if err := sleepContext(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestBackoffSchedule(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		okData(w)
	})
	c, _ := newTestClient(t, f)
	want := map[int]time.Duration{
		0:  500 * time.Millisecond,
		1:  time.Second,
		2:  2 * time.Second,
		20: 60 * time.Second,
	}
	for attempt, d := range want {
		if got := c.backoff(attempt); got != d {
			t.Errorf("backoff(%d) = %v, want %v", attempt, got, d)
		}
	}
	c.jitter = func() float64 {
		return 0
	}
	if got := c.backoff(0); got != 375*time.Millisecond {
		t.Errorf("backoff with jitter 0 = %v", got)
	}
	c.jitter = func() float64 {
		return 1
	}
	if got := c.backoff(0); got != 625*time.Millisecond {
		t.Errorf("backoff with jitter 1 = %v", got)
	}
}

func TestRateLimitInfo(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-RateLimit-Limit", "20")
		h.Set("X-RateLimit-Remaining", "19")
		h.Set("X-RateLimit-Minute-Limit", "1200")
		h.Set("X-RateLimit-Minute-Remaining", "1187")
		okData(w)
	})
	c, _ := newTestClient(t, f)
	if c.LastRateLimit() != nil {
		t.Fatal("LastRateLimit before any call must be nil")
	}
	if _, err := c.Rankings.Regions(context.Background()); err != nil {
		t.Fatalf("Regions: %v", err)
	}
	rl := c.LastRateLimit()
	want := RateLimit{Limit: 20, Remaining: 19, MinuteLimit: 1200, MinuteRemaining: 1187, DailyLimit: -1, DailyRemaining: -1}
	if rl == nil || *rl != want {
		t.Fatalf("LastRateLimit = %+v, want %+v", rl, want)
	}
	rl.Limit = 0
	if c.LastRateLimit().Limit != 20 {
		t.Fatal("LastRateLimit must return a copy")
	}
}

func TestAlertRules(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			writeJSON(w, http.StatusCreated, `{"id":"rule-1","name":"Big gifts","enabled":false}`)
		case http.MethodPatch:
			writeJSON(w, http.StatusOK, `{"id":"rule-1","name":"Big gifts","enabled":true}`)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			okData(w)
		}
	})
	c, _ := newTestClient(t, f)
	ctx := context.Background()

	created, err := c.Alerts.CreateRule(ctx, &AlertRuleParams{
		Name:            "Big gifts",
		CreatorUniqueID: String("linh.live"),
		EventTypes:      []string{"gift"},
		MinDiamondCount: Int(1000),
		Enabled:         Bool(false),
		Channels:        []AlertChannel{{ChannelType: "webhook", Target: "https://example.com/hooks/toktokdata"}},
	})
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	var rule map[string]any
	if err := created.Decode(&rule); err != nil || rule["id"] != "rule-1" || created.StatusCode != http.StatusCreated {
		t.Fatalf("CreateRule = %d %v %v", created.StatusCode, rule, err)
	}
	if _, err := c.Alerts.UpdateRule(ctx, "rule-1", map[string]any{"creatorUniqueId": nil, "enabled": true}); err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	if err := c.Alerts.DeleteRule(ctx, "rule-1"); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}

	calls := assertCalls(t, f, 3)
	if calls[0].method != http.MethodPost || calls[0].path != "/v1/alerts/rules" || calls[0].header.Get("Content-Type") != "application/json" {
		t.Fatalf("create request = %s %s %q", calls[0].method, calls[0].path, calls[0].header.Get("Content-Type"))
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(calls[0].body), &sent); err != nil {
		t.Fatalf("create body %q: %v", calls[0].body, err)
	}
	if sent["name"] != "Big gifts" || sent["creatorUniqueId"] != "linh.live" || sent["enabled"] != false || sent["minDiamondCount"] != float64(1000) {
		t.Fatalf("create body = %s", calls[0].body)
	}
	if _, ok := sent["scope"]; ok {
		t.Fatalf("empty scope must be omitted: %s", calls[0].body)
	}
	if _, ok := sent["region"]; ok {
		t.Fatalf("nil region must be omitted: %s", calls[0].body)
	}
	if calls[1].method != http.MethodPatch || calls[1].path != "/v1/alerts/rules/rule-1" || calls[1].body != `{"creatorUniqueId":null,"enabled":true}` {
		t.Fatalf("update request = %s %s %s", calls[1].method, calls[1].path, calls[1].body)
	}
	if calls[2].method != http.MethodDelete || calls[2].path != "/v1/alerts/rules/rule-1" || calls[2].body != "" {
		t.Fatalf("delete request = %s %s %q", calls[2].method, calls[2].path, calls[2].body)
	}
}

func TestExportsCreateWaitDownload(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			writeJSON(w, http.StatusAccepted, `{"id":"exp-1","dataset":"live_sessions","format":"csv","status":"pending"}`)
		case strings.HasSuffix(r.URL.Path, "/download"):
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", `attachment; filename="toktokdata-live_sessions.csv"`)
			_, _ = io.WriteString(w, "id,creator\ns1,linh.live\n")
		case n == 2:
			writeJSON(w, http.StatusOK, `{"id":"exp-1","status":"pending"}`)
		case n == 3:
			w.Header().Set("Retry-After", "5")
			writeJSON(w, http.StatusOK, `{"id":"exp-1","status":"pending"}`)
		default:
			writeJSON(w, http.StatusOK, `{"id":"exp-1","dataset":"live_sessions","format":"csv","status":"ready","rowCount":2,"downloadUrl":"/v1/exports/exp-1/download","error":null,"provenance":{"sourceKind":"derived"}}`)
		}
	})
	c, clock := newTestClient(t, f)
	ctx := context.Background()

	created, err := c.Exports.Create(ctx, &ExportCreateParams{Dataset: "live_sessions", Format: "csv", RangeDays: 7})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !created.IsPending() {
		t.Fatalf("Create StatusCode = %d, want 202 returned as is", created.StatusCode)
	}
	var job ExportJob
	if err := created.Decode(&job); err != nil || job.ID != "exp-1" || job.Status != "pending" {
		t.Fatalf("Create job = %+v, %v", job, err)
	}

	ready, err := c.Exports.WaitUntilReady(ctx, job.ID)
	if err != nil {
		t.Fatalf("WaitUntilReady: %v", err)
	}
	var done ExportJob
	if err := ready.Decode(&done); err != nil || done.Status != "ready" || done.RowCount != 2 || done.Error != "" {
		t.Fatalf("ready job = %+v, %v", done, err)
	}
	if ready.Provenance["sourceKind"] != "derived" {
		t.Fatalf("Provenance = %v", ready.Provenance)
	}
	assertSleeps(t, clock, 2*time.Second, 5*time.Second)

	file, err := c.Exports.Download(ctx, job.ID)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if string(file.Body) != "id,creator\ns1,linh.live\n" || file.Data != nil {
		t.Fatalf("Download body = %q", file.Body)
	}
	if file.Header.Get("Content-Disposition") == "" {
		t.Fatal("Content-Disposition header missing")
	}

	calls := assertCalls(t, f, 5)
	if calls[0].body != `{"dataset":"live_sessions","format":"csv","rangeDays":7}` {
		t.Fatalf("Create body = %s", calls[0].body)
	}
	if calls[1].path != "/v1/exports/exp-1" || calls[4].path != "/v1/exports/exp-1/download" {
		t.Fatalf("paths = %q, %q", calls[1].path, calls[4].path)
	}
}

func TestExportsWaitUntilReadyFailed(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"id":"exp-2","status":"failed","error":"boom"}`)
	})
	c, clock := newTestClient(t, f)
	resp, err := c.Exports.WaitUntilReady(context.Background(), "exp-2")
	if err != nil {
		t.Fatalf("a failed job is returned without error, got %v", err)
	}
	var job ExportJob
	if err := resp.Decode(&job); err != nil || job.Status != "failed" || job.Error != "boom" {
		t.Fatalf("job = %+v, %v", job, err)
	}
	assertCalls(t, f, 1)
	assertSleeps(t, clock)
}

func TestExportsWaitUntilReadyTimeout(t *testing.T) {
	f := newFakeAPI(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"id":"exp-3","status":"pending"}`)
	})
	c, clock := newTestClient(t, f)
	_, err := c.Exports.WaitUntilReady(context.Background(), "exp-3", WaitTimeout(5*time.Second))
	if !errors.Is(err, ErrPendingTimeout) {
		t.Fatalf("err = %v, want ErrPendingTimeout", err)
	}
	assertCalls(t, f, 3)
	assertSleeps(t, clock, 2*time.Second, 2*time.Second)
}

func TestResponseHelpers(t *testing.T) {
	r := newResponse(http.StatusOK, http.Header{}, []byte(`{"data":{"entries":[{"a":1}],"nextCursor":"abc"},"provenance":{"freshness":"historical"}}`))
	if r.NextCursor() != "abc" || len(r.Items()) != 1 || r.Provenance["freshness"] != "historical" {
		t.Fatalf("paged response helpers: cursor %q, items %v", r.NextCursor(), r.Items())
	}
	flat := newResponse(http.StatusOK, http.Header{}, []byte(`{"id":"exp-1","status":"ready"}`))
	if flat.Data != nil || flat.NextCursor() != "" || flat.Items() != nil || flat.PendingInfo() != nil {
		t.Fatal("flat response must have no data, cursor, items or pending info")
	}
	var body map[string]any
	if err := flat.Decode(&body); err != nil || body["id"] != "exp-1" {
		t.Fatalf("Decode flat = %v, %v", body, err)
	}
	empty := newResponse(http.StatusNoContent, http.Header{}, nil)
	if err := empty.Decode(&body); err != nil {
		t.Fatalf("Decode empty = %v", err)
	}
	csv := newResponse(http.StatusOK, http.Header{}, []byte("a,b\n1,2\n"))
	if csv.Data != nil || csv.Provenance != nil {
		t.Fatal("non-JSON body must not be parsed")
	}
	numeric := newResponse(http.StatusOK, http.Header{}, []byte(`{"data":{"items":[],"nextCursor":42}}`))
	if numeric.NextCursor() != "42" {
		t.Fatalf("numeric cursor = %q", numeric.NextCursor())
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	if d, ok := parseRetryAfter("7", now); !ok || d != 7*time.Second {
		t.Errorf("seconds: %v %v", d, ok)
	}
	if d, ok := parseRetryAfter(now.Add(30*time.Second).Format(http.TimeFormat), now); !ok || d != 30*time.Second {
		t.Errorf("HTTP date: %v %v", d, ok)
	}
	for _, bad := range []string{"", "-1", "soon"} {
		if _, ok := parseRetryAfter(bad, now); ok {
			t.Errorf("%q should not parse", bad)
		}
	}
}
