# @toktokdata.live/sdk

Official JavaScript / TypeScript client for the [toktokdata.live](https://toktokdata.live) API:
official TikTok LIVE rankings, creators, recorded LIVE sessions, gifts, gifters, agencies, trends,
alerts and exports.

- Zero runtime dependencies, uses the global `fetch` (Node 20+, Deno, Bun, Cloudflare Workers and other edge runtimes).
- ESM and CommonJS builds, TypeScript types included.
- Retries, `Retry-After`, 202 "still collecting" waiting, cursor pagination and webhook verification built in.

> **Keep your API key on a server.** Do not ship it in browser or mobile code: anyone who can read
> the bundle can spend your workspace's credits. Call the API from your backend (or an edge function)
> and pass the results to the browser.

## Install

```bash
npm install @toktokdata.live/sdk
```

## Quickstart

Create a key in the dashboard (**Settings → API keys**). Keys look like `ttk_live_<id>_<secret>`.

```ts
import { TokTokData } from "@toktokdata.live/sdk";

const client = new TokTokData(); // reads TOKTOKDATA_API_KEY from the environment

const { data, provenance } = await client.rankings.official("hourly", { region: "VN", limit: 10 });
if (data.board.publishable) {
  for (const entry of data.board.entries) {
    console.log(entry.rank, entry.masked ? "(masked)" : entry.uniqueId, entry.score);
  }
}
console.log(provenance.sourceKind, provenance.observedAt);
```

CommonJS works too: `const { TokTokData } = require("@toktokdata.live/sdk");`

### Options

| Option | Default | |
|---|---|---|
| `apiKey` | `TOKTOKDATA_API_KEY` | Missing key throws `TokTokDataError` with code `missing_api_key`. |
| `baseUrl` | `TOKTOKDATA_BASE_URL`, else `https://api.toktokdata.live` | |
| `timeoutMs` | `30000` | Per request. |
| `maxRetries` | `2` | See [Retries](#retries). |
| `pendingTimeoutMs` | `180000` | How long 202-aware methods wait. |
| `fetch` | global `fetch` | Inject your own (proxies, tests). |

Reads return the JSON body unchanged: `{ data, provenance }`. `provenance` says how fresh and complete
the data is and whether a number is TikTok's own (`sourceKind: "official"`) or computed by
toktokdata.live (`"derived"`). Treat a missing number as unknown, not 0.

## Resources

Path parameters are positional; query parameters go in one options object. `undefined` options are
left out of the query string, booleans are sent as `true`/`false`. Creator handles may start with `@`
(it is dropped).

### rankings (`rank:read`)

```ts
await client.rankings.official("daily", { region: "VN", period: "current", limit: 50, liveOnly: true, leagueTier: "A1" });
await client.rankings.regions();
await client.rankings.history("daily", { region: "VN", limit: 24 });
await client.rankings.dayBars({ region: "VN", board: "daily", days: 30 });
await client.rankings.masked(entry.maskedRef!); // LIVE totals of a masked row
```

Board keys: `daily`, `hourly`, `popular_live`, `gaming_daily`, `gaming_weekly`, `shopping_daily`,
`shopping_weekly` (needs `shop:read`), `league_a1` … `league_d5`.

### creators (`creator:read`; `gifters` needs `gifter:read`, `videos` needs `content:read`)

```ts
await client.creators.search({ q: "linh", limit: 20 });
await client.creators.get("linh.live");                 // waits while the profile is looked up (202)
await client.creators.analysis("linh.live");
await client.creators.changes("linh.live", { limit: 50 });
await client.creators.followers("linh.live", { limit: 100 });
await client.creators.following("linh.live", { cursor });
await client.creators.gifters("linh.live", { window: "30d", limit: 20 });
await client.creators.insights("linh.live");
await client.creators.videos("linh.live");
```

### agencies (`creator:read`)

```ts
await client.agencies.list({ q: "star", region: "VN", publicOnly: true, sort: "revenue", limit: 50, offset: 0 });
await client.agencies.get("7012345678901234567");
```

### insights (`creator:read`)

```ts
await client.insights.bestTime({ region: "VN" });
await client.insights.presence({ region: "VN", hours: 48 });
```

### videos (`content:read`)

```ts
await client.videos.get("7400000000000000001");
await client.videos.comments("7400000000000000001", { parentCommentId, limit: 50 }); // 202 aware
```

### trends (`trend:read`), gifts (`live:read`), gifters (`gifter:read`)

```ts
await client.trends.list({ region: "US", type: "hashtag" });
await client.gifts.list({ q: "rose", minCoins: 1, maxCoins: 100, region: "VN", sort: "coins", limit: 100 });
await client.gifters.list({ window: "7d", segment: "whale", region: "VN", limit: 50 });
```

### live (`live:read`; `playback` needs `live:stream`)

```ts
await client.live.sessions({ creatorUniqueId: "linh.live", limit: 50 });
await client.live.session(sessionId);
await client.live.timeline(sessionId);
await client.live.compare([idA, idB]);                    // 2 to 4 ids
await client.live.performance("linh.live", { limit: 50 });
await client.live.refresh("linh.live");                   // POST, returns { queued, jobId }
await client.live.playback("linh.live");                  // signed HLS/FLV URL, 202 aware
const chat = await client.live.comments("linh.live");     // free; poll with { after: chat.cursor }
```

### alerts (`webhook:manage`, free)

```ts
const { rule, webhookSecrets } = await client.alerts.createRule({
  name: "Envelopes from Linh",
  creatorUniqueId: "linh.live",
  eventTypes: ["envelope"],
  minDiamondCount: 100,
  channels: [{ channelType: "webhook", target: "https://example.com/hooks/toktokdata" }],
});
await client.alerts.listRules();
await client.alerts.updateRule(rule.id, { enabled: false });
await client.alerts.testRule(rule.id);
await client.alerts.fires({ ruleId: rule.id, limit: 50 });
await client.alerts.deliveries({ status: "failed" });
await client.alerts.stats();
await client.alerts.deleteRule(rule.id);
```

### exports (`export`)

```ts
let job = await client.exports.create({ dataset: "live_sessions", format: "csv", rangeDays: 30 });
job = await client.exports.waitUntilReady(job.id);       // polls every 2 s, up to 300 s
if (job.status === "ready") {
  const csv = await client.exports.download(job.id);                    // string
  const xlsx = await client.exports.download(job.id, { as: "bytes" });  // Uint8Array
}
await client.exports.list();
```

`waitUntilReady` returns the job when it is `ready` or `failed` (check `job.status`) and throws
`PendingTimeoutError` if it is still pending after `timeoutMs`.

### account (`keys:manage`, free)

```ts
await client.account.entitlements();                 // plan, limits, credits.remaining, credits.resetAt
await client.account.usage({ period: "2026-11-01" }); // credits spent per operation
```

Anything not wrapped yet: `await client.request("GET", "/v1/...", { query, body })` (same auth,
retries and errors).

## Errors

Every failure is a `TokTokDataError`:

| Field | |
|---|---|
| `status` | HTTP status, `0` for network errors / timeouts |
| `code` | e.g. `insufficient_scope`, `credit_cap_exceeded`, `rate_limited`, `http_error` (non-JSON body), `network_error`, `timeout` |
| `message`, `requestId` | Quote `requestId` to support |
| `retryable` | Retrying later can succeed |
| `retryAfter` | Seconds from `Retry-After` |
| `details` | The whole error envelope (`resetAt` on 402) |

Helpers: `isUnauthorized` (401), `isPaymentRequired` (402), `isForbidden` (403), `isNotFound` (404),
`isRateLimited` (429).

```ts
import { TokTokDataError } from "@toktokdata.live/sdk";

try {
  await client.gifters.list();
} catch (error) {
  if (error instanceof TokTokDataError) {
    if (error.isPaymentRequired) console.log("Out of credits until", error.details?.resetAt);
    else if (error.isForbidden) console.log("Missing scope or plan:", error.code, error.message);
    else if (error.isRateLimited) console.log("Retry in", error.retryAfter, "s");
    else throw error;
  }
}
```

## Retries

`maxRetries` (default 2) extra attempts:

- `429` is retried for every method (the limiter answers before the request runs), except
  `daily_quota_exceeded`.
- Network errors, timeouts and `5xx` (when the envelope says `retryable: true` or there is no envelope)
  are retried for `GET` only.
- `402` and other `4xx` are never retried.

The client waits `Retry-After` seconds when present (capped at 60 s), else 0.5 s, 1 s, 2 s … with jitter.

## 202: data still being collected

`creators.get`, `creators.followers`, `creators.following`, `videos.comments` and `live.playback` may
answer `202 { status: "resolving" | "saturated", jobId, retryAfterSeconds }` while toktokdata.live
reads TikTok. By default these methods wait `retryAfterSeconds` (1–120 s) and ask again until the data
arrives, up to `pendingTimeoutMs` (180 s), then throw `PendingTimeoutError` (a `TokTokDataError` with
`jobId`, `pendingStatus` and `retryAfterSeconds`). A 202 is never billed beyond a cached read.

```ts
import { isPending } from "@toktokdata.live/sdk";

const profile = await client.creators.get("new.creator", { maxWaitMs: 30_000 }); // shorter budget
const maybe = await client.creators.get("new.creator", { wait: false });        // opt out
if (isPending(maybe)) console.log("ask again in", maybe.retryAfterSeconds, "s");
```

## Pagination

Cursor lists return `data.nextCursor` (`null` on the last page). Iterators yield whole pages:

```ts
for await (const page of client.live.iterSessions({ limit: 100 })) {
  for (const session of page.data.items) console.log(session.id, session.totalGiftDiamonds);
}

// Any cursor list, with a page cap:
for await (const page of client.paginate((p) => client.creators.followers("linh.live", p), { limit: 100 }, { maxPages: 5 })) {
  console.log(page.data.entries.length);
}
```

Built-in iterators: `rankings.iterHistory`, `creators.iterFollowers`, `creators.iterFollowing`,
`videos.iterComments`, `live.iterSessions`, `live.iterPerformance`.

## Rate-limit info

`client.lastRateLimit` holds the last response's `X-RateLimit-*` headers:

```ts
await client.rankings.regions();
console.log(client.lastRateLimit);
// { limit, remaining, minuteLimit, minuteRemaining, dailyLimit, dailyRemaining }
```

## Webhooks

Alert webhooks carry `x-toktokdata-signature: sha256=<hex>` (HMAC-SHA256 of
`"<x-toktokdata-timestamp>.<raw body>"` with the channel's signing secret, returned once by
`createRule` in `webhookSecrets`). Verify with the raw body, before any JSON parsing:

```ts
import express from "express";
import { verifyWebhook, WebhookVerificationError } from "@toktokdata.live/sdk/webhooks";

app.post("/hooks/toktokdata", express.raw({ type: "application/json" }), (req, res) => {
  try {
    const payload = verifyWebhook(req.body, req.headers, process.env.TOKTOKDATA_WEBHOOK_SECRET!);
    console.log(payload.alertId, payload.event);
    res.sendStatus(204);
  } catch (error) {
    if (error instanceof WebhookVerificationError) return res.status(400).send(error.code);
    throw error;
  }
});
```

Timestamps more than 300 s from now are rejected (`{ toleranceSeconds }` to change). Error codes:
`webhook_missing_header`, `webhook_invalid_timestamp`, `webhook_timestamp_expired`,
`webhook_invalid_signature`, `webhook_invalid_json`. `@toktokdata.live/sdk/webhooks` uses `node:crypto`
(Node, Bun, Deno); the main entry has no Node imports.

## License

MIT
