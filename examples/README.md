# toktokdata.live API examples

Small runnable projects that call the [toktokdata.live](https://toktokdata.live) public API
(`https://api.toktokdata.live/v1/...`, described in [`../openapi/openapi.json`](../openapi/openapi.json)).
Every sample reads the API key from the `TOKTOKDATA_API_KEY` environment variable. None of
them contains a key, and none should: never commit or paste one.

| Folder | Uses | Covers |
|---|---|---|
| [`curl/`](curl/) | curl (+ jq if present), PowerShell | official board, creator profile with a `202` wait loop, entitlements |
| [`typescript/`](typescript/) | `@toktokdata/sdk` | leaderboard, creator report (profile + analysis + LIVE sessions via the paging iterator), CSV export, webhook receiver |
| [`python/`](python/) | `toktokdata` | same four scripts as TypeScript |
| [`go/`](go/) | `github.com/MKK-Platform/toktokdata-sdks/go` | same four commands as TypeScript |
| [`java/`](java/) | plain `java.net.http` + Jackson | leaderboard, creator with `202` retry, cursor paging over LIVE sessions, error envelope (`429`, `402`), webhook signature check |
| [`csharp/`](csharp/) | plain `HttpClient` + `System.Text.Json` (.NET 8) | same as Java |
| [`php/`](php/) | plain curl extension (PHP 8.1) | same as Java |
| [`ruby/`](ruby/) | plain `net/http` + `json` (Ruby 3) | same as Java |

Java, C#, PHP and Ruby have no SDK yet, so their samples show the raw HTTP contract:
that is also the reference if you write your own client.

## Get an API key

1. Sign in at <https://toktokdata.live> and open the dashboard.
2. Go to **Settings → API keys**, create a key and tick only the scopes you need.
3. Copy the key (`ttk_live_...`); it is shown once. Export it in your shell:

```bash
export TOKTOKDATA_API_KEY=ttk_live_...          # bash / zsh
$env:TOKTOKDATA_API_KEY = 'ttk_live_...'        # PowerShell
```

If a key leaks, revoke it in the same screen and create a new one.

`TOKTOKDATA_BASE_URL` overrides `https://api.toktokdata.live` (every sample honours it).
The webhook receivers read the rule's signing secret from `TOKTOKDATA_WEBHOOK_SECRET`.

## Scopes

Each operation needs one scope on the key (the `x-scope` of the operation in the spec):

| Scope | Operations |
|---|---|
| `rank:read` | `/v1/rankings/*` (official boards, regions, history, day bars, masked entries) |
| `creator:read` | `/v1/creators`, `/v1/creators/{uid}` and its `analysis`, `changes`, `followers`, `following`, `insights`; `/v1/agencies`; `/v1/insights/*` |
| `gifter:read` | `/v1/gifters`, `/v1/creators/{uid}/gifters` |
| `content:read` | `/v1/videos/*`, `/v1/creators/{uid}/videos` |
| `live:read` | `/v1/live/sessions*`, `/v1/live/creators/{uid}/performance`, `.../refresh`, `.../comments`, `/v1/gifts` |
| `live:stream` | `/v1/live/creators/{uid}/playback` |
| `trend:read` | `/v1/trends` |
| `shop:read` | the `shopping_daily` / `shopping_weekly` boards |
| `export` | `/v1/exports*` |
| `webhook:manage` | `/v1/alerts/*` (alert rules, fires, deliveries, stats) |
| `keys:manage` | `/v1/entitlements`, `/v1/usage` |

## Things every client should handle

- **Errors** come as `{"code", "message", "requestId", "retryable"}`. Quote `requestId` when you
  contact support.
- **`429`**: too many requests. Wait `Retry-After` seconds and try again (except
  `daily_quota_exceeded`, which lasts until the next day).
- **`402 credit_cap_exceeded`**: the workspace's credits are spent; `resetAt` says when they
  come back. Do not retry.
- **`403`**: `insufficient_scope` (add the scope to the key) or `module_not_entitled` (the plan
  does not include the feature).
- **`202`** on creator lookups (`/v1/creators/{uid}`, `followers`, `following`), video comments
  and LIVE playback: the data is still being collected. The body has `status`
  (`resolving` or `saturated`), `jobId` and `retryAfterSeconds`; ask again after that many seconds.
- **Paging**: lists answer `data.nextCursor`; pass it back as `cursor` until it is `null`.
- **Webhooks**: alert webhooks carry `x-toktokdata-timestamp` and
  `x-toktokdata-signature: sha256=<hex HMAC-SHA256(secret, "<timestamp>.<raw body>")>`. Verify it on
  the raw bytes with a constant-time compare and reject timestamps older than 300 seconds.
