# PHP examples (plain curl extension)

There is no PHP SDK yet. These scripts call the API with the `curl` extension and no other
dependency, and show everything a client needs:

| File | What it shows | Scope |
|---|---|---|
| `TokTokData.php` | `Client::get()`: Bearer auth from `TOKTOKDATA_API_KEY`, query encoding, the `202` wait loop (`retryAfterSeconds` / `Retry-After`, clamped 1–120 s, 180 s deadline), retries of `429` (after `Retry-After`, except `daily_quota_exceeded`) and retryable `5xx`, the error envelope as `ApiException`; `verifyWebhook()`; `exitWithError()` for `402` / `403` / `429` | |
| `leaderboard.php [REGION] [BOARD]` | `GET /v1/rankings/official`, top 10 (default `VN` `hourly`) | `rank:read` |
| `creator.php [HANDLE]` | `GET /v1/creators/{uid}`, waiting through `202` | `creator:read` |
| `sessions.php [HANDLE]` | `GET /v1/live/sessions?creatorUniqueId=...`, following `data.nextCursor` (up to 50 sessions) | `live:read` |
| `webhook.php` | Verifies an alert webhook: HMAC-SHA256 of `"<x-toktokdata-timestamp>.<raw body>"`, `hash_equals` against `x-toktokdata-signature: sha256=<hex>`, 300 s tolerance | none; needs `TOKTOKDATA_WEBHOOK_SECRET` |

## Setup and run

PHP 8.1+ with the `curl` and `json` extensions (`composer check-platform-reqs` checks them; there is
nothing to install).

```bash
cd examples/php
export TOKTOKDATA_API_KEY=ttk_live_...   # Settings -> API keys at https://toktokdata.live
php leaderboard.php VN hourly
php creator.php tiktok
php sessions.php tiktok
TOKTOKDATA_WEBHOOK_SECRET=... php -S localhost:8080 webhook.php
```

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
