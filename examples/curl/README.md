# curl / PowerShell quickstart

Three calls with nothing but an HTTP client:

1. `GET /v1/rankings/official?board=hourly&region=VN&limit=10`: the top 10 of the official hourly board.
2. `GET /v1/creators/{uid}`: a creator profile. A creator that is not stored yet answers
   `202` with `retryAfterSeconds`; the script waits that long and asks again (up to 180 s).
3. `GET /v1/entitlements`: your plan and remaining credits.

`429` responses are retried after `Retry-After`; other errors print the error envelope
(`code`, `message`, `requestId`, and `resetAt` on `402 credit_cap_exceeded`).

## Scopes

`rank:read`, `creator:read`, `keys:manage` (a step whose scope is missing prints
`403 insufficient_scope` and the script goes on).

## Run

```bash
export TOKTOKDATA_API_KEY=ttk_live_...   # Settings -> API keys at https://toktokdata.live
./quickstart.sh                           # region VN, creator @tiktok
./quickstart.sh US tiktok
```

[jq](https://jqlang.github.io/jq/) is optional: with it the output is formatted, without it you
get raw JSON.

```powershell
$env:TOKTOKDATA_API_KEY = 'ttk_live_...'
.\quickstart.ps1 -Region VN -Creator tiktok
```

The PowerShell script works on Windows PowerShell 5.1 and PowerShell 7 and needs no jq.

## Output (shape)

```
== Official hourly board, VN (scope rank:read)
publishable: true, period 2026-11-18T09
1	@creator.one	99000	LIVE
2	@creator.two	98000
...

== Creator @tiktok (scope creator:read)
  202 resolving, asking again in 5s
TikTok (@tiktok), US: 90000000 followers, 1200 videos, last refreshed 2026-11-18T09:12:00.000Z

== Entitlements (scope keys:manage)
plan pro: 812345 of 1000000 credits left, reset 2026-12-01T00:00:00.000Z
```

Numbers above are illustrative.
