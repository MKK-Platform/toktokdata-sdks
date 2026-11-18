# Ruby examples (plain `net/http`)

There is no Ruby SDK yet. These scripts use only the standard library (`net/http`, `json`,
`openssl`, `socket`) and show everything a client needs:

| File | What it shows | Scope |
|---|---|---|
| `lib/toktokdata_http.rb` | `Client#get`: Bearer auth from `TOKTOKDATA_API_KEY`, query encoding, the `202` wait loop (`retryAfterSeconds` / `Retry-After`, clamped 1–120 s, 180 s deadline), retries of `429` (after `Retry-After`, except `daily_quota_exceeded`) and retryable `5xx`, the error envelope as `ApiError`; `verify_webhook`; `exit_with_error` for `402` / `403` / `429` | |
| `leaderboard.rb [REGION] [BOARD]` | `GET /v1/rankings/official`, top 10 (default `VN` `hourly`) | `rank:read` |
| `creator.rb [HANDLE]` | `GET /v1/creators/{uid}`, waiting through `202` | `creator:read` |
| `sessions.rb [HANDLE]` | `GET /v1/live/sessions?creatorUniqueId=...` as an Enumerator that follows `data.nextCursor` (first 50 sessions) | `live:read` |
| `webhook_server.rb [PORT]` | Tiny local server that verifies alert webhooks: HMAC-SHA256 of `"<x-toktokdata-timestamp>.<raw body>"`, `OpenSSL.secure_compare` against `x-toktokdata-signature: sha256=<hex>`, 300 s tolerance | none; needs `TOKTOKDATA_WEBHOOK_SECRET` |

## Setup and run

Ruby 3.0+; nothing to install.

```bash
cd examples/ruby
export TOKTOKDATA_API_KEY=ttk_live_...   # Settings -> API keys at https://toktokdata.live
ruby leaderboard.rb VN hourly
ruby creator.rb tiktok
ruby sessions.rb tiktok
TOKTOKDATA_WEBHOOK_SECRET=... ruby webhook_server.rb 8080
```

In a Rack, Rails or Sinatra app, call `TokTokDataHttp.verify_webhook(raw_body, timestamp, signature, secret)`
with the raw request body (`request.body.read` in Rack/Sinatra, `request.raw_post` in Rails) instead of
running `webhook_server.rb`.

## Output (shape)

```
hourly board, VN, period 2026-11-18T09
  1  Creator One @creator.one                        99,000  LIVE
 11  (masked, plan pro)                              41,000

  202 resolving, asking again in 5 s
TikTok (@tiktok) [verified]
  region US, 90,000,000 followers, 1,200 videos, 3,000,000,000 likes

  2026-11-17T12:00:00.000Z   120 min  peak   2,000 viewers      5,000 diamonds
```

An error prints `HTTP <status> <code>: <message> (requestId ...)`; on `402 credit_cap_exceeded`
also when credits reset. Numbers above are illustrative.
