# TypeScript examples (`@toktokdata.live/sdk`)

| Script | What it does | Scopes |
|---|---|---|
| `leaderboard.ts` | Top 10 of an official board (`hourly` by default) for a region, plus the rate-limit headers of the call | `rank:read` |
| `creator-report.ts` | Profile (waits through `202` while the creator is being looked up), content analysis, and up to 30 recent LIVE sessions read with the paging iterator | `creator:read`, `live:read` |
| `export-csv.ts` | Creates a `rankings` CSV export, waits until it is ready, downloads it to a file | `export` |
| `webhook-server.ts` | `node:http` server that verifies alert webhook signatures with `verifyWebhook` (from `@toktokdata.live/sdk/webhooks`, Node only) and prints each alert | none (the alert rule needs `webhook:manage`) |

## Setup

Node.js 20 or newer.

```bash
cd examples/typescript
npm install
export TOKTOKDATA_API_KEY=ttk_live_...   # Settings -> API keys at https://toktokdata.live
```

`package.json` points `@toktokdata.live/sdk` at the SDK source in this repo (`file:../../js`; build it
first with `npm install && npm run build` in `js/`). Once the SDK is on npm, switch the dependency to
`"@toktokdata.live/sdk": "^0.1.0"`.

## Run

```bash
npx tsx leaderboard.ts VN hourly
npx tsx creator-report.ts tiktok
npx tsx export-csv.ts VN daily            # writes toktokdata-rankings-....csv
TOKTOKDATA_WEBHOOK_SECRET=... npx tsx webhook-server.ts 8080
```

(or `npm run leaderboard -- VN hourly`, and so on.) `npm run typecheck` type-checks all scripts.

## What they print

```
hourly board, VN, period 2026-11-18T09
  1  Creator One @creator.one                        99,000  LIVE
  2  Creator Two @creator.two                        98,000
  ...
 11  (masked, plan pro)                              41,000

rate limit: 149 of 150 left, 1199998 left today
```

```
TikTok (@tiktok) [verified]
  region US, 90,000,000 followers, 1,200 videos, 3,000,000,000 likes
  last refreshed 2026-11-18T09:12:00.000Z

Content (30 videos over 41 days)
  plays avg 2,100,000, median 1,400,000, engagement 6.2%
  5 posts per week

Recent LIVE sessions (up to 30)
  2026-11-17T12:00:00.000Z   120 min  peak   2,000 viewers      5,000 diamonds
```

Numbers above are illustrative. Errors print the envelope fields (`status`, `code`, `message`,
`requestId`) and, for `402 credit_cap_exceeded`, when credits reset. The SDK already retries
`429` (after `Retry-After`) and retryable `5xx`.

## Webhooks

Create an alert rule with a `webhook` channel (dashboard, or `client.alerts.createRule(...)`
with the `webhook:manage` scope). The signing secret is shown once when the channel is created;
put it in `TOKTOKDATA_WEBHOOK_SECRET`. The server rejects a request with `401` when the
`x-toktokdata-signature` header does not match or `x-toktokdata-timestamp` is older than 300 s.
Expose it with a tunnel (for example `cloudflared` or `ngrok`) while testing: webhook targets
must be publicly reachable (private and loopback addresses are refused).
