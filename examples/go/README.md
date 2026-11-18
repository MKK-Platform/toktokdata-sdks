# Go examples (`github.com/MKK-Platform/toktokdata-sdks/go`)

| Command | What it does | Scopes |
|---|---|---|
| `cmd/leaderboard` | Top 10 of an official board (`hourly` by default) for a region | `rank:read` |
| `cmd/creator-report` | Profile (waits through `202` while the creator is being looked up), content analysis, and up to 30 recent LIVE sessions read with the `Live.SessionsIter` page iterator | `creator:read`, `live:read` |
| `cmd/export-csv` | Creates a `rankings` CSV export, waits until it is ready (bounded by the context), downloads it to a file | `export` |
| `cmd/webhook-server` | `net/http` server that verifies alert webhook signatures with `VerifyWebhook` and prints each alert | none (the alert rule needs `webhook:manage`) |

## Setup

Go 1.22 or newer. `go.mod` replaces the SDK module with the source in this repo (`../../go`);
once the SDK is tagged, delete the `replace` line and `go get github.com/MKK-Platform/toktokdata-sdks/go@v0.1.0`.

```bash
cd examples/go
export TOKTOKDATA_API_KEY=ttk_live_...   # Settings -> API keys at https://toktokdata.live
```

## Run

```bash
go run ./cmd/leaderboard VN hourly
go run ./cmd/creator-report tiktok
go run ./cmd/export-csv VN daily          # writes the file named by the export job
TOKTOKDATA_WEBHOOK_SECRET=... go run ./cmd/webhook-server 8080
```

## What they print

```
hourly board, VN, period 2026-11-18T09
  1  Creator One @creator.one                        99000  LIVE
  2  Creator Two @creator.two                        98000
 ...
 11  (masked, plan pro)                              41000
```

`creator-report` prints the profile, the analysis (plays, engagement rate, posts per week and any
`limits` notes) and one line per LIVE session. Numbers depend on live data.

SDK calls return a `*toktokdata.Response`; the commands decode its body into small typed structs
with `DecodeBody` to keep the printing code readable. Errors are `*toktokdata.APIError` (`Status`,
`Code`, `Message`, `RequestID`, `RetryAfter`, `Details`), printed by `sample.Exit`; the SDK already
retries `429` (after `Retry-After`) and retryable `5xx`.

## Webhooks

Create an alert rule with a `webhook` channel (dashboard, or `client.Alerts.CreateRule` with the
`webhook:manage` scope). The signing secret is shown once when the channel is created; put it in
`TOKTOKDATA_WEBHOOK_SECRET`. The server answers `401` when `x-toktokdata-signature` does not match
or `x-toktokdata-timestamp` is older than 300 s. Expose it with a tunnel while testing: webhook
targets must be publicly reachable (private and loopback addresses are refused).
