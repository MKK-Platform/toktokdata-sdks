# Java examples (plain HTTP)

There is no Java SDK yet. This Maven project calls the API with `java.net.http.HttpClient` and
Jackson, and shows everything a client needs:

| File | What it shows |
|---|---|
| `TokTokDataHttp.java` | Bearer auth from `TOKTOKDATA_API_KEY`, query encoding, the `202` wait loop (`retryAfterSeconds` / `Retry-After`, clamped 1–120 s, 180 s deadline), retries of `429` (after `Retry-After`, except `daily_quota_exceeded`) and retryable `5xx`, and the error envelope as `ApiException` |
| `Webhooks.java` | Alert webhook signature check: HMAC-SHA256 of `"<x-toktokdata-timestamp>.<raw body>"`, constant-time compare with `x-toktokdata-signature: sha256=<hex>`, 300 s tolerance |
| `Main.java` | The commands below, and how to react to `402`, `403` and `429` |

| Command | Calls | Scope |
|---|---|---|
| `leaderboard [REGION] [BOARD]` | `GET /v1/rankings/official` (top 10, default `VN` `hourly`) | `rank:read` |
| `creator [HANDLE]` | `GET /v1/creators/{uid}`, waiting through `202` | `creator:read` |
| `sessions [HANDLE]` | `GET /v1/live/sessions?creatorUniqueId=...`, following `data.nextCursor` (up to 50 sessions) | `live:read` |
| `webhook [PORT]` | Local server (`com.sun.net.httpserver`) that verifies and prints alert webhooks | none; needs `TOKTOKDATA_WEBHOOK_SECRET` |

## Setup and run

Java 17+ and Maven 3.9+.

```bash
cd examples/java
export TOKTOKDATA_API_KEY=ttk_live_...   # Settings -> API keys at https://toktokdata.live
mvn -q compile exec:java -Dexec.args="leaderboard VN hourly"
mvn -q compile exec:java -Dexec.args="creator tiktok"
mvn -q compile exec:java -Dexec.args="sessions tiktok"
TOKTOKDATA_WEBHOOK_SECRET=... mvn -q compile exec:java -Dexec.args="webhook 8080"
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
