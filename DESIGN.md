# SDK design contract

Every SDK in this repo implements the same contract, so a developer who knows one knows all of
them. The source of truth for routes, parameters and examples is `openapi/openapi.json`
(the `x-public` operations of the toktokdata.live API). This file says how SDKs wrap it.

## Client

| | |
|---|---|
| Base URL | `https://api.toktokdata.live` (paths start with `/v1/`). Override with an option or `TOKTOKDATA_BASE_URL`. |
| Auth | `Authorization: Bearer <key>`. Key from an option, else env `TOKTOKDATA_API_KEY`; missing key fails at construction with a clear message. Keys look like `ttk_live_<id>_<secret>`. Never log the key. |
| Headers | `Accept: application/json`, `User-Agent: toktokdata-<lang>/<version>`, `Content-Type: application/json` on bodies. |
| Timeout | 30 s per request by default, configurable. |
| Retries | `maxRetries` default 2. Retry network errors, `429` (except code `daily_quota_exceeded`), and `5xx` whose envelope says `retryable: true` (or has no envelope). Wait `Retry-After` seconds when present (cap 60 s), else exponential backoff 0.5 s, 1 s, 2 s ... with jitter. Never retry `402` or other `4xx`. A `429` is retried for every method (the limiter answers before the handler runs); `5xx` and network errors are retried for GET only. |
| Responses | Return the parsed JSON body unchanged (reads are `{ "data": ..., "provenance": {...} }`; exports and alert rules are flat objects). Types may be loose (`Record<string, unknown>` / `dict` / `map[string]any`) plus light typed helpers where cheap. `204` returns nothing/null. `GET /v1/exports/{id}/download` returns the file as text/bytes. |
| Rate-limit info | Expose the last response's `X-RateLimit-*` headers (`limit`, `remaining`, `minuteLimit`, `minuteRemaining`, `dailyLimit`, `dailyRemaining`) via something like `client.lastRateLimit` / `client.last_rate_limit` / `client.LastRateLimit()`. |

## Errors

One error type: JS/TS `TokTokDataError`, Python `TokTokDataError`, Go `*APIError`.
Fields: `status` (HTTP), `code`, `message`, `requestId`, `retryable`, `retryAfter` (seconds, from
the header, optional), `details` (the whole envelope, which may carry `resetAt`).
Envelope: `{"code","message","requestId","retryable", ...extra}`. Non-JSON error bodies still
produce the error with `code: "http_error"`.
Helpers: `isRateLimited` (429), `isPaymentRequired` (402, code `credit_cap_exceeded`),
`isForbidden` (403: `insufficient_scope`, `module_not_entitled`), `isUnauthorized` (401),
`isNotFound` (404). Plus a distinct timeout error for 202 polling (`PendingTimeoutError`).

## 202 "not ready yet"

Some reads collect fresh data and answer `202` with
`{"status": "resolving"|"saturated", "jobId", "retryAfterSeconds"}` and `Retry-After`:
`GET /v1/creators/{uid}`, `.../followers`, `.../following`, `GET /v1/videos/{id}/comments`,
`GET /v1/live/creators/{uid}/playback`.
Default: these methods **wait** — sleep `retryAfterSeconds` (or `Retry-After`, clamped 1–120 s)
and ask again until a 200 arrives or 180 s pass (`PendingTimeoutError`). A 202 is never billed
beyond a cached read. Callers can opt out (`wait: false` / `wait=False` / Go option) and get the
202 body back. `POST /v1/live/creators/{uid}/refresh` (202 queued) and `POST /v1/exports`
(201 ready or 202 pending) return their body as is.

## Pagination

Cursor lists return `data.nextCursor` (null at the end) and accept `cursor` + `limit`:
creators followers / following, live sessions, live performance, rankings history,
video comments. Each SDK offers an iterator over pages or items (JS async iterator
`client.paginate(...)` or `for await (const page of client.live.iterSessions(...))`; Python
generator `iter_*`; Go `Next()`-style iterator or callback). Items live under
`data.entries`, `data.items` or `data.comments` depending on the route; the
iterator yields whole pages, so it works for all of them.

## Resources and methods

Names: camelCase in JS, snake_case in Python, PascalCase in Go. Path params are positional;
query params and bodies go in one options/params object (Go: a params struct per method).

| Namespace | Method | Route |
|---|---|---|
| rankings | `official(board, {region, period, limit, liveOnly, leagueTier})` | GET /v1/rankings/official |
| | `regions()` | GET /v1/rankings/regions |
| | `history(board, {region, cursor, limit, leagueTier})` | GET /v1/rankings/history |
| | `dayBars({region, board, days})` | GET /v1/rankings/day-bars |
| | `masked(ref)` | GET /v1/rankings/masked/{ref} |
| creators | `search({q, limit})` | GET /v1/creators |
| | `get(uid, {wait})` | GET /v1/creators/{uid} (202 aware) |
| | `analysis(uid)` | GET /v1/creators/{uid}/analysis |
| | `changes(uid, {limit})` | GET /v1/creators/{uid}/changes |
| | `followers(uid, {cursor, limit, wait})` | GET /v1/creators/{uid}/followers (202 aware, paged) |
| | `following(uid, {cursor, limit, wait})` | GET /v1/creators/{uid}/following (202 aware, paged) |
| | `gifters(uid, {window, limit})` | GET /v1/creators/{uid}/gifters |
| | `insights(uid)` | GET /v1/creators/{uid}/insights |
| | `videos(uid)` | GET /v1/creators/{uid}/videos |
| agencies | `list({q, region, publicOnly, sort, limit, offset})` | GET /v1/agencies |
| | `get(agencyId)` | GET /v1/agencies/{agency_id} |
| insights | `bestTime({region})` | GET /v1/insights/best-time |
| | `presence({region, hours})` | GET /v1/insights/presence |
| videos | `get(videoId)` | GET /v1/videos/{video_id} |
| | `comments(videoId, {parentCommentId, cursor, limit, wait})` | GET /v1/videos/{video_id}/comments (202 aware, paged) |
| trends | `list({region, type})` | GET /v1/trends |
| gifts | `list({q, minCoins, maxCoins, region, sort, limit})` | GET /v1/gifts |
| gifters | `list({window, segment, region, limit})` | GET /v1/gifters |
| live | `sessions({creatorUniqueId, cursor, limit})` | GET /v1/live/sessions (paged) |
| | `session(sessionId)` | GET /v1/live/sessions/{session_id} |
| | `timeline(sessionId)` | GET /v1/live/sessions/{session_id}/timeline |
| | `compare(ids[])` | GET /v1/live/sessions/compare (`ids` comma-joined, 2–4) |
| | `performance(uid, {cursor, limit})` | GET /v1/live/creators/{uid}/performance (paged) |
| | `refresh(uid)` | POST /v1/live/creators/{uid}/refresh |
| | `playback(uid, {wait})` | GET /v1/live/creators/{uid}/playback (202 aware) |
| | `comments(uid, {after})` | GET /v1/live/creators/{uid}/comments |
| alerts | `listRules()` / `createRule(body)` / `updateRule(ruleId, body)` / `deleteRule(ruleId)` / `testRule(ruleId)` | /v1/alerts/rules... |
| | `fires({ruleId, since, limit})` / `deliveries({ruleId, status, since, limit})` / `stats()` | /v1/alerts/... |
| exports | `list()` / `create({dataset, board, region, leagueTier, format, rangeDays})` / `get(jobId)` / `download(jobId)` | /v1/exports... |
| | `waitUntilReady(jobId, {timeout})` | polls `get` every 2 s (honour `Retry-After`) until `status` is `ready` or `failed` |
| account | `entitlements({period})` / `usage({period, limit})` | GET /v1/entitlements, GET /v1/usage |

Check every query parameter name, enum and required flag against `openapi/openapi.json`
(for example `board` is required on `official` and `history`; `ids` on `compare`).
Omit `undefined`/`None`/zero-value params from the query string; booleans as `true`/`false`.

## Webhooks

Each SDK ships a signature helper for alert webhooks:
`verifyWebhook(rawBody, headers, secret, {toleranceSeconds = 300})` checks
`x-toktokdata-signature: sha256=<hex HMAC-SHA256(secret, "<x-toktokdata-timestamp>.<raw body>")>`
with a constant-time compare and rejects timestamps older than the tolerance. Returns the parsed
payload or raises/returns an error.

## Testing

Unit tests run offline against a mocked HTTP layer: auth header, query encoding, error mapping,
retry on 429 with `Retry-After`, no retry on 402, 202 wait loop, pagination, webhook signature.
No test calls the real API. An optional smoke script may call production when
`TOKTOKDATA_API_KEY` is set.

## Versioning

SDKs start at `0.1.0` and follow semver. The API path `/v1` only grows compatibly.
