# toktokdata Go SDK

Go client for the [toktokdata.live](https://toktokdata.live) API: official TikTok LIVE rankings,
creators, recorded LIVE sessions, alerts and exports. Standard library only, Go 1.22+.

```sh
go get github.com/MKK-Platform/toktokdata-sdks/go
```

Releases are tagged per module in this monorepo, so Go module tags look like `go/v0.1.0`
(`go get github.com/MKK-Platform/toktokdata-sdks/go@v0.1.0` resolves that tag).

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"log"

	toktokdata "github.com/MKK-Platform/toktokdata-sdks/go"
)

func main() {
	// Reads TOKTOKDATA_API_KEY (and TOKTOKDATA_BASE_URL when set).
	client, err := toktokdata.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Rankings.Official(context.Background(), "hourly",
		&toktokdata.RankingsOfficialParams{Region: "VN", Limit: 10})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(resp.Data))             // the "data" member, raw JSON
	fmt.Println(resp.Provenance["sourceKind"]) // "official" or "derived"
}
```

### Client options

| Option | Default |
|---|---|
| `WithAPIKey(key)` | env `TOKTOKDATA_API_KEY`; `NewClient` fails with `ErrMissingAPIKey` (code `missing_api_key`) when both are empty |
| `WithBaseURL(url)` | env `TOKTOKDATA_BASE_URL`, else `https://api.toktokdata.live` |
| `WithHTTPClient(hc)` | a new `*http.Client` |
| `WithTimeout(d)` | 30 s per HTTP attempt (0 disables; the context still applies) |
| `WithMaxRetries(n)` | 2 |
| `WithPendingTimeout(d)` | 180 s, see [202 waiting](#202-not-ready-yet) |

Every method takes a `context.Context` first. Retries, 202 waits and export polling stop as soon as
the context is done.

### Responses

Every call returns a `*toktokdata.Response` that keeps the JSON body unchanged:

| Field / method | |
|---|---|
| `StatusCode`, `Header`, `Body` | the HTTP status, headers and raw body |
| `Data` (`json.RawMessage`) | the `data` member of read responses (nil for flat objects such as exports and alert rules) |
| `Provenance` (`map[string]any`) | how fresh and complete the data is, `official` or `derived`, the time window |
| `Decode(&v)` | unmarshals `data`, or the whole body when there is no `data` member |
| `DecodeBody(&v)` | unmarshals the whole body |
| `NextCursor()`, `Items()` | the cursor and the items of a paged list |
| `IsPending()`, `PendingInfo()` | a 202 answer and its `status` / `jobId` / `retryAfterSeconds` |

### Parameters

Path parameters are positional; query parameters go in a params struct per method (pass `nil` for
the API defaults). String and int fields are sent only when they are not `""` / `0`. Booleans are
`*bool` so `false` can be sent: `toktokdata.Bool(false)`; `nil` leaves them out. Creator uids accept
a leading `@`, which is dropped.

## One example per service

```go
ctx := context.Background()

// Rankings: official boards, regions, history, day bars, masked rows.
client.Rankings.Official(ctx, "daily", &toktokdata.RankingsOfficialParams{Region: "VN", LiveOnly: toktokdata.Bool(true)})
client.Rankings.Regions(ctx)
client.Rankings.History(ctx, "league_a1", &toktokdata.RankingsHistoryParams{Region: "VN", Limit: 24})
client.Rankings.DayBars(ctx, &toktokdata.RankingsDayBarsParams{Region: "VN", Days: 30})
client.Rankings.Masked(ctx, "maskedRefFromTheBoard")

// Creators: search, profile (202 aware), analysis, changes, followers/following, gifters, insights, videos.
client.Creators.Search(ctx, &toktokdata.CreatorsSearchParams{Q: "linh", Limit: 5})
client.Creators.Get(ctx, "linh.live", nil)
client.Creators.Followers(ctx, "linh.live", &toktokdata.CreatorFollowParams{Limit: 50})
client.Creators.Gifters(ctx, "linh.live", &toktokdata.CreatorGiftersParams{Window: "30d"})

// Agencies.
client.Agencies.List(ctx, &toktokdata.AgenciesListParams{Region: "VN", Sort: "revenue"})
client.Agencies.Get(ctx, "7000000000000000000")

// Insights.
client.Insights.BestTime(ctx, &toktokdata.InsightsBestTimeParams{Region: "VN"})
client.Insights.Presence(ctx, &toktokdata.InsightsPresenceParams{Region: "VN", Hours: 24})

// Videos.
client.Videos.Get(ctx, "7300000000000000001")
client.Videos.Comments(ctx, "7300000000000000001", &toktokdata.VideoCommentsParams{Limit: 50})

// Trends, gifts, gifters.
client.Trends.List(ctx, &toktokdata.TrendsListParams{Region: "US", Type: "hashtag"})
client.Gifts.List(ctx, &toktokdata.GiftsListParams{MinCoins: 100, Sort: "coins"})
client.Gifters.List(ctx, &toktokdata.GiftersListParams{Window: "7d", Segment: "whale", Region: "VN"})

// Live: sessions, one session, timeline, compare, performance, refresh, playback, chat.
client.Live.Sessions(ctx, &toktokdata.LiveSessionsParams{CreatorUniqueID: "linh.live"})
client.Live.Session(ctx, "session-id")
client.Live.Timeline(ctx, "session-id")
client.Live.Compare(ctx, []string{"session-a", "session-b"})
client.Live.Performance(ctx, "linh.live", nil)
client.Live.Refresh(ctx, "linh.live")                 // 202 queued, returned as is
client.Live.Playback(ctx, "linh.live", nil)           // needs live:stream, 202 aware
client.Live.Comments(ctx, "linh.live", &toktokdata.LiveCommentsParams{After: "cursor"})

// Alerts.
client.Alerts.CreateRule(ctx, &toktokdata.AlertRuleParams{
	Name:            "Big gifts",
	CreatorUniqueID: toktokdata.String("linh.live"),
	EventTypes:      []string{"gift"},
	MinDiamondCount: toktokdata.Int(1000),
	Channels:        []toktokdata.AlertChannel{{ChannelType: "webhook", Target: "https://example.com/hook"}},
})
client.Alerts.UpdateRule(ctx, "rule-id", map[string]any{"enabled": false, "creatorUniqueId": nil})
client.Alerts.DeleteRule(ctx, "rule-id")
client.Alerts.Fires(ctx, &toktokdata.AlertFiresParams{Since: "2026-11-01T00:00:00Z"})
client.Alerts.Stats(ctx)

// Exports.
created, _ := client.Exports.Create(ctx, &toktokdata.ExportCreateParams{Dataset: "live_sessions", Format: "csv", RangeDays: 7})
var job toktokdata.ExportJob
created.Decode(&job)
ready, _ := client.Exports.WaitUntilReady(ctx, job.ID)  // polls every 2 s, up to 300 s
ready.Decode(&job)                                       // job.Status is "ready" or "failed"
file, _ := client.Exports.Download(ctx, job.ID)          // file.Body holds the CSV / XLSX bytes

// Account.
client.Account.Entitlements(ctx, nil)
client.Account.Usage(ctx, &toktokdata.AccountUsageParams{Period: "2026-11-01"})
```

(Errors are ignored above for brevity; check them in real code.)

## Errors

Every non-2xx answer is a `*toktokdata.APIError` with `Status`, `Code`, `Message`, `RequestID`,
`Retryable`, `RetryAfter` (from the header), `Details` (the whole envelope, which may carry
`resetAt`) and the raw `Body`. A body that is not a JSON envelope still gives an `*APIError` with
`Code: "http_error"`.

```go
_, err := client.Gifters.List(ctx, nil)
switch {
case toktokdata.IsRateLimited(err):     // 429
case toktokdata.IsPaymentRequired(err): // 402, e.g. credit_cap_exceeded (Details["resetAt"])
case toktokdata.IsForbidden(err):       // 403, insufficient_scope or module_not_entitled
case toktokdata.IsUnauthorized(err):    // 401
case toktokdata.IsNotFound(err):        // 404
case err != nil:
	var apiErr *toktokdata.APIError
	if errors.As(err, &apiErr) {
		log.Printf("%d %s: %s (request %s)", apiErr.Status, apiErr.Code, apiErr.Message, apiErr.RequestID)
	}
}
```

## Retries

Up to `WithMaxRetries` (default 2) retries:

- `429` for every method (the rate limiter answers before the request runs), except code
  `daily_quota_exceeded`;
- `5xx` and network errors for `GET` only, and a `5xx` only when its envelope does not say
  `retryable: false`;
- never `402` or any other `4xx`.

The wait is `Retry-After` when present (capped at 60 s), else exponential backoff 0.5 s, 1 s, 2 s ...
with jitter.

## 202 "not ready yet"

`Creators.Get`, `Creators.Followers`, `Creators.Following`, `Videos.Comments` and `Live.Playback`
may answer `202` while fresh data is collected. By default they sleep `retryAfterSeconds` (clamped to
1–120 s) and ask again until a `200` arrives; after 180 s (`WithPendingTimeout`) they return an error
matching `toktokdata.ErrPendingTimeout` (a `*toktokdata.PendingTimeoutError` holding the last 202
response). Opt out per call with `Wait: toktokdata.Bool(false)` to get the 202 body back:

```go
resp, err := client.Creators.Get(ctx, "linh.live", &toktokdata.CreatorGetParams{Wait: toktokdata.Bool(false)})
if err == nil && resp.IsPending() {
	info := resp.PendingInfo()
	fmt.Println(info.Status, info.JobID, info.RetryAfterSeconds)
}
```

`Live.Refresh` (202 queued) and `Exports.Create` (201 ready or 202 pending) return their body as is.

## Pagination

Cursor-paged lists have an `Iter` variant that yields whole pages: `Live.SessionsIter`,
`Live.PerformanceIter`, `Rankings.HistoryIter`, `Creators.FollowersIter`, `Creators.FollowingIter`,
`Videos.CommentsIter`.

```go
it := client.Live.SessionsIter(ctx, &toktokdata.LiveSessionsParams{CreatorUniqueID: "linh.live", Limit: 100})
for it.Next() {
	page := it.Page()
	for _, raw := range page.Items() { // data.items, data.entries, data.sessions or data.comments
		var session map[string]any
		_ = json.Unmarshal(raw, &session)
	}
}
if err := it.Err(); err != nil {
	log.Fatal(err)
}
```

Iteration stops when `data.nextCursor` is `null`. `it.Cursor()` gives the cursor to resume from.

## Webhooks

Alert webhooks are signed with `x-toktokdata-signature: sha256=<hex HMAC-SHA256(secret,
"<x-toktokdata-timestamp>.<raw body>")>`. `VerifyWebhook` checks the signature in constant time,
rejects timestamps more than the tolerance (default 5 minutes) away, and returns the parsed payload:

```go
secret := os.Getenv("TOKTOKDATA_WEBHOOK_SECRET") // the whsec_... value of the channel

http.HandleFunc("/hooks/toktokdata", func(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // the raw bytes, before any JSON decoding
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	payload, err := toktokdata.VerifyWebhook(body, r.Header, secret, 5*time.Minute)
	if err != nil {
		var werr *toktokdata.WebhookError
		if errors.As(err, &werr) {
			log.Println("rejected:", werr.Code)
		}
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	log.Println("alert", payload["alertId"]) // deduplicate on alertId / x-toktokdata-alert-id
	w.WriteHeader(http.StatusNoContent)
})
```

A rejected delivery returns a `*toktokdata.WebhookError` whose `Code` is one of
`webhook_missing_header`, `webhook_invalid_timestamp`, `webhook_timestamp_expired`,
`webhook_invalid_signature` or `webhook_invalid_json`.

## Rate-limit info

`client.LastRateLimit()` returns the `X-RateLimit-*` headers of the latest response that carried
them (or `nil`): `Limit` / `Remaining` (per second, per key), `MinuteLimit` / `MinuteRemaining`
(per minute, per workspace), `DailyLimit` / `DailyRemaining` (the key's daily quota). A field is
`-1` when its header was absent.

```go
if rl := client.LastRateLimit(); rl != nil && rl.MinuteRemaining == 0 {
	time.Sleep(time.Minute)
}
```

## Development

```sh
cd go
gofmt -l .
go vet ./...
go test -race ./...
```

Tests run offline against `net/http/httptest`; no test calls the real API.
