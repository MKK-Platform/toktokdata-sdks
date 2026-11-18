package toktokdata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Version is the SDK version, sent as User-Agent: toktokdata-go/<Version>.
const Version = "0.1.0"

// DefaultBaseURL is the API origin used when neither WithBaseURL nor the
// TOKTOKDATA_BASE_URL environment variable is set.
const DefaultBaseURL = "https://api.toktokdata.live"

// DefaultTimeout is the per-request timeout used when WithTimeout is not given.
const DefaultTimeout = 30 * time.Second

// DefaultMaxRetries is the number of retries used when WithMaxRetries is not given.
const DefaultMaxRetries = 2

// DefaultPendingTimeout is how long the 202-aware methods keep asking again
// before they give up with ErrPendingTimeout.
const DefaultPendingTimeout = 180 * time.Second

const (
	maxRetryAfter   = 60 * time.Second
	minPendingDelay = time.Second
	maxPendingDelay = 120 * time.Second
	maxBackoffShift = 10
)

// Client calls the toktokdata.live API. Create it with NewClient; it is safe
// for concurrent use by multiple goroutines.
type Client struct {
	apiKey         string
	baseURL        string
	httpClient     *http.Client
	timeout        time.Duration
	maxRetries     int
	pendingTimeout time.Duration
	userAgent      string

	// sleep, now and jitter are replaced in tests.
	sleep  func(ctx context.Context, d time.Duration) error
	now    func() time.Time
	jitter func() float64

	mu            sync.Mutex
	lastRateLimit *RateLimit

	// Rankings wraps /v1/rankings: official boards, history, day bars.
	Rankings *RankingsService

	// Creators wraps /v1/creators: search, profile, followers, gifters.
	Creators *CreatorsService

	// Agencies wraps /v1/agencies.
	Agencies *AgenciesService

	// Insights wraps /v1/insights: best time to go LIVE and presence.
	Insights *InsightsService

	// Videos wraps /v1/videos.
	Videos *VideosService

	// Trends wraps /v1/trends.
	Trends *TrendsService

	// Gifts wraps /v1/gifts (the gift catalog).
	Gifts *GiftsService

	// Gifters wraps /v1/gifters (the gifter leaderboard).
	Gifters *GiftersService

	// Live wraps /v1/live: recorded sessions, performance, playback, comments.
	Live *LiveService

	// Alerts wraps /v1/alerts: rules, fires, deliveries, stats.
	Alerts *AlertsService

	// Exports wraps /v1/exports.
	Exports *ExportsService

	// Account wraps /v1/entitlements and /v1/usage.
	Account *AccountService
}

type config struct {
	apiKey         string
	baseURL        string
	httpClient     *http.Client
	timeout        time.Duration
	maxRetries     int
	pendingTimeout time.Duration
}

// Option configures a Client. Pass options to NewClient.
type Option func(*config)

// WithAPIKey sets the API key (keys look like ttk_live_<id>_<secret>).
// Without this option the key is read from TOKTOKDATA_API_KEY.
func WithAPIKey(key string) Option {
	return func(c *config) {
		c.apiKey = key
	}
}

// WithBaseURL overrides the API origin (default https://api.toktokdata.live,
// or TOKTOKDATA_BASE_URL when set). Paths such as /v1/rankings are appended
// to it.
func WithBaseURL(baseURL string) Option {
	return func(c *config) {
		c.baseURL = baseURL
	}
}

// WithHTTPClient sets the *http.Client used to send requests, for example to
// add a proxy or instrumentation. The client's own Timeout still applies on
// top of WithTimeout.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *config) {
		c.httpClient = hc
	}
}

// WithTimeout sets the timeout of each HTTP attempt (default 30 s). Zero or a
// negative value disables the per-request timeout; the caller's context
// still applies.
func WithTimeout(d time.Duration) Option {
	return func(c *config) {
		c.timeout = d
	}
}

// WithMaxRetries sets how many times a failed request is retried (default 2).
// Zero disables retries; negative values count as zero.
func WithMaxRetries(n int) Option {
	return func(c *config) {
		c.maxRetries = n
	}
}

// WithPendingTimeout sets how long the 202-aware methods (Creators.Get,
// Creators.Followers, Creators.Following, Videos.Comments, Live.Playback)
// keep waiting for collected data before returning ErrPendingTimeout
// (default 180 s).
func WithPendingTimeout(d time.Duration) Option {
	return func(c *config) {
		c.pendingTimeout = d
	}
}

// NewClient returns a configured Client. It fails with ErrMissingAPIKey when
// no key is given through WithAPIKey or TOKTOKDATA_API_KEY, and with an error
// when the base URL is not an absolute http(s) URL.
func NewClient(opts ...Option) (*Client, error) {
	cfg := config{
		timeout:        DefaultTimeout,
		maxRetries:     DefaultMaxRetries,
		pendingTimeout: DefaultPendingTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	key := strings.TrimSpace(cfg.apiKey)
	if key == "" {
		key = strings.TrimSpace(os.Getenv("TOKTOKDATA_API_KEY"))
	}
	if key == "" {
		return nil, ErrMissingAPIKey
	}

	base := strings.TrimSpace(cfg.baseURL)
	if base == "" {
		base = strings.TrimSpace(os.Getenv("TOKTOKDATA_BASE_URL"))
	}
	if base == "" {
		base = DefaultBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("toktokdata: invalid base URL %q: want an absolute http(s) URL", base)
	}

	hc := cfg.httpClient
	if hc == nil {
		hc = &http.Client{}
	}
	if cfg.maxRetries < 0 {
		cfg.maxRetries = 0
	}
	if cfg.pendingTimeout <= 0 {
		cfg.pendingTimeout = DefaultPendingTimeout
	}

	c := &Client{
		apiKey:         key,
		baseURL:        strings.TrimRight(base, "/"),
		httpClient:     hc,
		timeout:        cfg.timeout,
		maxRetries:     cfg.maxRetries,
		pendingTimeout: cfg.pendingTimeout,
		userAgent:      "toktokdata-go/" + Version,
		sleep:          sleepContext,
		now:            time.Now,
		jitter:         rand.Float64,
	}
	c.Rankings = &RankingsService{c: c}
	c.Creators = &CreatorsService{c: c}
	c.Agencies = &AgenciesService{c: c}
	c.Insights = &InsightsService{c: c}
	c.Videos = &VideosService{c: c}
	c.Trends = &TrendsService{c: c}
	c.Gifts = &GiftsService{c: c}
	c.Gifters = &GiftersService{c: c}
	c.Live = &LiveService{c: c}
	c.Alerts = &AlertsService{c: c}
	c.Exports = &ExportsService{c: c}
	c.Account = &AccountService{c: c}
	return c, nil
}

// RateLimit is the rate-limit state reported by the X-RateLimit-* headers of
// a response. Limit and Remaining describe the per-second window of the key,
// MinuteLimit and MinuteRemaining the per-minute window shared by the
// workspace, DailyLimit and DailyRemaining the key's daily quota. A field is
// -1 when the response did not carry that header.
type RateLimit struct {
	Limit           int
	Remaining       int
	MinuteLimit     int
	MinuteRemaining int
	DailyLimit      int
	DailyRemaining  int
}

// LastRateLimit returns a copy of the rate-limit headers of the most recent
// response that carried any X-RateLimit-* header, or nil when none has been
// seen yet.
func (c *Client) LastRateLimit() *RateLimit {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastRateLimit == nil {
		return nil
	}
	rl := *c.lastRateLimit
	return &rl
}

func (c *Client) recordRateLimit(h http.Header) {
	rl := parseRateLimit(h)
	if rl == nil {
		return
	}
	c.mu.Lock()
	c.lastRateLimit = rl
	c.mu.Unlock()
}

func parseRateLimit(h http.Header) *RateLimit {
	rl := &RateLimit{
		Limit:           -1,
		Remaining:       -1,
		MinuteLimit:     -1,
		MinuteRemaining: -1,
		DailyLimit:      -1,
		DailyRemaining:  -1,
	}
	found := false
	read := func(name string, dst *int) {
		v := strings.TrimSpace(h.Get(name))
		if v == "" {
			return
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return
		}
		*dst = n
		found = true
	}
	read("X-RateLimit-Limit", &rl.Limit)
	read("X-RateLimit-Remaining", &rl.Remaining)
	read("X-RateLimit-Minute-Limit", &rl.MinuteLimit)
	read("X-RateLimit-Minute-Remaining", &rl.MinuteRemaining)
	read("X-RateLimit-Daily-Limit", &rl.DailyLimit)
	read("X-RateLimit-Daily-Remaining", &rl.DailyRemaining)
	if !found {
		return nil
	}
	return rl
}

// get sends a GET request with retries.
func (c *Client) get(ctx context.Context, path string, query url.Values) (*Response, error) {
	return c.do(ctx, http.MethodGet, path, query, nil)
}

// do sends one API call, retrying it as DESIGN.md allows, and returns the
// 2xx response or an error (*APIError for non-2xx answers).
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) (*Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("toktokdata: encode request body: %w", err)
		}
		payload = encoded
	}
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resp, err := c.send(ctx, method, target, payload)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			// A network error may hide a request the server already
			// processed, so only idempotent GETs are sent again.
			if method == http.MethodGet && attempt < c.maxRetries {
				if serr := c.sleep(ctx, c.backoff(attempt)); serr != nil {
					return nil, serr
				}
				continue
			}
			return nil, fmt.Errorf("toktokdata: %s %s: %w", method, path, err)
		}
		c.recordRateLimit(resp.Header)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		apiErr := newAPIError(resp.StatusCode, resp.Header, resp.Body, c.now())
		if attempt < c.maxRetries && shouldRetry(method, apiErr) {
			wait := apiErr.RetryAfter
			if wait <= 0 {
				wait = c.backoff(attempt)
			}
			if wait > maxRetryAfter {
				wait = maxRetryAfter
			}
			if serr := c.sleep(ctx, wait); serr != nil {
				return nil, serr
			}
			continue
		}
		return nil, apiErr
	}
}

// send performs a single HTTP attempt and reads the whole body.
func (c *Client) send(ctx context.Context, method, target string, payload []byte) (*Response, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return newResponse(res.StatusCode, res.Header, data), nil
}

// shouldRetry reports whether a non-2xx answer may be sent again.
// 429 means the request was throttled before it ran, so it is retried for
// every method (except the daily quota, which will not reset soon); a 5xx is
// retried for GET only, unless the envelope says retryable: false.
func shouldRetry(method string, e *APIError) bool {
	switch {
	case e.Status == http.StatusTooManyRequests:
		return e.Code != "daily_quota_exceeded"
	case e.Status >= 500:
		if method != http.MethodGet {
			return false
		}
		if e.hasRetryable {
			return e.Retryable
		}
		return true
	default:
		return false
	}
}

// backoff returns 0.5 s, 1 s, 2 s ... (capped at 60 s) with +/-25 % jitter.
func (c *Client) backoff(attempt int) time.Duration {
	if attempt > maxBackoffShift {
		attempt = maxBackoffShift
	}
	base := 500 * time.Millisecond
	for i := 0; i < attempt; i++ {
		base *= 2
	}
	if base > maxRetryAfter {
		base = maxRetryAfter
	}
	factor := 0.75 + 0.5*c.jitter()
	return time.Duration(float64(base) * factor)
}

// getPending sends a GET to a 202-aware route. Unless wait is false it sleeps
// through 202 answers until a 200 arrives or the pending timeout passes.
func (c *Client) getPending(ctx context.Context, path string, query url.Values, wait *bool) (*Response, error) {
	deadline := c.now().Add(c.pendingTimeout)
	for {
		resp, err := c.get(ctx, path, query)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusAccepted || (wait != nil && !*wait) {
			return resp, nil
		}
		delay := pendingDelay(resp, c.now())
		if c.now().Add(delay).After(deadline) {
			return nil, &PendingTimeoutError{Path: path, Timeout: c.pendingTimeout, Last: resp}
		}
		if serr := c.sleep(ctx, delay); serr != nil {
			return nil, serr
		}
	}
}

// pendingDelay reads retryAfterSeconds from a 202 body (or the Retry-After
// header) and clamps it to 1-120 s.
func pendingDelay(resp *Response, now time.Time) time.Duration {
	secs := 0.0
	var body PendingBody
	if err := json.Unmarshal(resp.Body, &body); err == nil {
		secs = body.RetryAfterSeconds
	}
	if secs <= 0 {
		if ra, ok := parseRetryAfter(resp.Header.Get("Retry-After"), now); ok {
			secs = ra.Seconds()
		}
	}
	if secs < minPendingDelay.Seconds() {
		secs = minPendingDelay.Seconds()
	}
	if secs > maxPendingDelay.Seconds() {
		secs = maxPendingDelay.Seconds()
	}
	return time.Duration(secs * float64(time.Second))
}

// sleepContext waits for d or until ctx is done.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
