# toktokdata (Python)

Official Python client for the [toktokdata.live](https://toktokdata.live) API: TikTok LIVE
rankings, creators, gifters, agencies, LIVE sessions, trends, alerts and exports.

- Python 3.9+, **no runtime dependencies** (standard library `urllib` only), fully typed (`py.typed`).
- Auth, retries with `Retry-After`, waiting for data that is still being collected (`202`),
  cursor pagination, typed errors, rate-limit info and a webhook signature helper.

API docs: <https://toktokdata.live/docs> · Reference: <https://toktokdata.live/docs/reference>

## Install

```bash
pip install toktokdata
```

## Quickstart

Create a key at <https://toktokdata.live> (**Settings → API keys**) and keep it in the environment:

```bash
export TOKTOKDATA_API_KEY=ttk_live_...
```

```python
from toktokdata import TokTokData

with TokTokData() as client:  # reads TOKTOKDATA_API_KEY
    board = client.rankings.official("hourly", region="VN", limit=10)
    for entry in board["data"]["board"]["entries"]:
        print(entry["rank"], entry["uniqueId"], entry["score"])
```

Every method returns the parsed JSON body unchanged, as a plain `dict`. Reads look like
`{"data": ..., "provenance": {...}}`; exports and alert rules are flat objects. Field names are
the API's (camelCase); method and argument names are snake_case.

## Configuration

```python
client = TokTokData(
    api_key="ttk_live_...",  # default: TOKTOKDATA_API_KEY (required, checked here)
    base_url="https://api.toktokdata.live",  # default: TOKTOKDATA_BASE_URL or this URL
    timeout=30.0,  # seconds per HTTP request
    max_retries=2,  # retries after the first attempt
    pending_timeout=180.0,  # how long 202-aware methods wait (keyword-only)
)
```

The client never logs or prints the key (`repr(client)` masks it). Call `client.close()` or
use it as a context manager.

## One example per namespace

```python
# rankings: official boards (daily, hourly, popular_live, gaming_*, shopping_*, league_a1..d5)
client.rankings.official("daily", region="VN", period="current", live_only=True)
client.rankings.regions()
client.rankings.history("daily", region="VN", limit=24)
client.rankings.day_bars(region="VN", board="daily", days=30)
client.rankings.masked("<maskedRef from a board entry>")

# creators: uid is a handle (a leading "@" is stripped) or a numeric TikTok user id
client.creators.search(q="linh", limit=20)
client.creators.get("linh.live")  # waits on 202 while TikTok is read
client.creators.analysis("linh.live")
client.creators.changes("linh.live", limit=50)
client.creators.followers("linh.live", limit=100)
client.creators.following("linh.live")
client.creators.gifters("linh.live", window="30d")
client.creators.insights("linh.live")
client.creators.videos("linh.live")

# agencies
client.agencies.list(region="VN", public_only=True, sort="revenue", limit=50, offset=0)
client.agencies.get("<agency id>")

# insights
client.insights.best_time(region="VN")
client.insights.presence(region="VN", hours=48)

# videos
client.videos.get("7300000000000000000")
client.videos.comments("7300000000000000000", limit=50)

# trends, gifts, gifters
client.trends.list(region="US", type="hashtag")
client.gifts.list(q="rose", min_coins=1, max_coins=100, sort="coins")
client.gifters.list(window="7d", segment="whale", region="VN")

# live
client.live.sessions(creator_unique_id="linh.live", limit=50)
client.live.session("<session id>")
client.live.timeline("<session id>")
client.live.compare(["<session id>", "<session id>"])  # 2 to 4 ids
client.live.performance("linh.live")
client.live.refresh("linh.live")  # 202 {"queued": true, "jobId": ...}
client.live.playback("linh.live")  # needs the live:stream scope
chat = client.live.comments("linh.live")
newer = client.live.comments("linh.live", after=chat["cursor"])

# alerts
created = client.alerts.create_rule(
    {
        "name": "Whale gifts",
        "scope": "watchlist",
        "eventTypes": ["gift", "envelope"],
        "minDiamondCount": 1000,
        "channels": [{"channelType": "webhook", "target": "https://example.com/hooks/toktokdata"}],
    }
)
rule_id = created["rule"]["id"]  # created["webhookSecrets"]: signing secrets
client.alerts.update_rule(rule_id, {"enabled": False})
client.alerts.test_rule(rule_id)
client.alerts.list_rules()
client.alerts.fires(rule_id=rule_id, limit=20)
client.alerts.deliveries(status="failed", since="2026-11-01T00:00:00Z")
client.alerts.stats()
client.alerts.delete_rule(rule_id)  # returns None (204)

# exports
job = client.exports.create(dataset="rankings", format="csv", board="daily", region="VN")
job = client.exports.wait_until_ready(job["id"])  # no-op when it is already ready
if job["status"] == "ready":
    open(job["filename"], "wb").write(client.exports.download(job["id"]))
client.exports.list()

# account
client.account.entitlements()
client.account.usage(limit=50)
```

Type hints list the accepted enum values (for example `period: "current" | "history"`), and
`toktokdata.AlertRuleInput` describes the alert rule body. Any route can also be called
directly with `client.request("GET", "/v1/...", query={...})`.

## Errors

Every failed call raises `toktokdata.TokTokDataError`:

```python
from toktokdata import TokTokDataError

try:
    client.creators.get("linh.live")
except TokTokDataError as err:
    print(err.status, err.code, err.message, err.request_id)
    if err.is_payment_required:  # 402 credit_cap_exceeded
        print("credits reset at", err.details.get("resetAt"))
    elif err.is_rate_limited:  # 429 (incl. daily_quota_exceeded)
        print("retry in", err.retry_after, "s")
    elif err.is_forbidden:  # 403 insufficient_scope / module_not_entitled
        ...
    elif err.is_unauthorized:  # 401
        ...
    elif err.is_not_found:  # 404
        ...
```

| Field | Meaning |
|---|---|
| `status` | HTTP status (`0` when no response arrived: `code` is `network_error` or `timeout`) |
| `code` | Machine-readable code from the envelope; `http_error` when the body was not JSON |
| `message` | Human-readable message |
| `request_id` | Quote it to support |
| `retryable` | The API's hint whether a retry may succeed |
| `retry_after` | Seconds from the `Retry-After` header, or `None` |
| `details` | The whole error envelope (for example `resetAt`) |

`PendingTimeoutError` (a subclass) is raised when a 202 read or an export is still not ready
after the wait; `WebhookVerificationError` (a subclass) by `verify_webhook`.

## Retries

The client retries up to `max_retries` times (default 2):

- `429` rate limits, waiting `Retry-After` seconds (capped at 60 s). A `429` with code
  `daily_quota_exceeded` is not retried: the quota resets at midnight UTC.
- `5xx` answers whose envelope says `retryable: true` (or that have no envelope), GET only.
- Network errors and timeouts, GET only.

Without `Retry-After` it backs off 0.5 s, 1 s, 2 s ... with jitter. `402` and other `4xx`
are never retried, and a POST/PATCH/DELETE is not retried once it may have reached the server.

## Waiting for fresh data (202)

`creators.get`, `creators.followers`, `creators.following`, `videos.comments` and
`live.playback` may answer `202` (`{"status": "resolving" | "saturated", "jobId",
"retryAfterSeconds"}`) while toktokdata.live reads TikTok. These methods wait
`retryAfterSeconds` (clamped to 1-120 s) and ask again until the data is there, for up to
`pending_timeout` seconds (180 by default), then raise `PendingTimeoutError`. A 202 is never
billed beyond a cached read.

```python
from toktokdata import PendingTimeoutError

try:
    profile = client.creators.get("new.creator")
except PendingTimeoutError as err:
    print("still resolving, job", err.job_id)

# Or handle it yourself:
body = client.creators.get("new.creator", wait=False)
if body.get("status") in ("resolving", "saturated"):
    ...  # ask again after body["retryAfterSeconds"]
```

`live.refresh` (202 queued) and `exports.create` (201 ready or 202 pending) return their body
as is; use `exports.wait_until_ready(job_id, timeout=300, poll_interval=2)` for exports.

## Pagination

Cursor lists return `data.nextCursor` (null at the end) and take `cursor` + `limit`. Each has an
`iter_*` generator that yields whole pages and follows the cursor for you:

```python
for page in client.live.iter_sessions(creator_unique_id="linh.live", limit=100):
    for session in page["data"]["items"]:
        print(session["startedAt"], session["totalGiftDiamonds"])
```

| Generator | Items under |
|---|---|
| `rankings.iter_history(board, ...)` | `data.periods` |
| `creators.iter_followers(uid, ...)` / `creators.iter_following(uid, ...)` | `data.entries` |
| `videos.iter_comments(video_id, ...)` | `data.comments` |
| `live.iter_sessions(...)` | `data.items` |
| `live.iter_performance(uid, ...)` | `data.items` |

Pages are fetched lazily, one request per page; pass `cursor=` to resume.

## Webhooks

Alert webhooks carry `x-toktokdata-timestamp` and
`x-toktokdata-signature: sha256=<hex HMAC-SHA256(secret, "<timestamp>.<raw body>")>`.
`verify_webhook` checks the signature in constant time, rejects timestamps more than
`tolerance_seconds` (300) away from now, and returns the parsed payload. Always pass the
**raw** request body, never re-serialised JSON.

Flask:

```python
import os
from flask import Flask, abort, request
from toktokdata import WebhookVerificationError, verify_webhook

app = Flask(__name__)
SECRET = os.environ["TOKTOKDATA_WEBHOOK_SECRET"]


@app.post("/hooks/toktokdata")
def toktokdata_hook():
    try:
        payload = verify_webhook(request.get_data(), request.headers, SECRET)
    except WebhookVerificationError:
        abort(400)
    print(payload["rule"]["name"], payload["event"])
    return "", 204
```

FastAPI:

```python
import os
from fastapi import FastAPI, HTTPException, Request, Response
from toktokdata import WebhookVerificationError, verify_webhook

app = FastAPI()
SECRET = os.environ["TOKTOKDATA_WEBHOOK_SECRET"]


@app.post("/hooks/toktokdata")
async def toktokdata_hook(request: Request) -> Response:
    raw = await request.body()
    try:
        payload = verify_webhook(raw, request.headers, SECRET)
    except WebhookVerificationError as err:
        raise HTTPException(status_code=400, detail=err.code)
    print(payload["rule"]["name"], payload["event"])
    return Response(status_code=204)
```

`sign_webhook(secret, timestamp, raw_body)` builds the header value, handy for testing your
handler.

## Rate-limit info

`client.last_rate_limit` holds the `X-RateLimit-*` headers of the last response that carried
them (or `None` before the first call):

```python
client.rankings.regions()
rl = client.last_rate_limit
if rl is not None:
    print(rl.limit, rl.remaining)  # per second
    print(rl.minute_limit, rl.minute_remaining)  # per minute
    print(rl.daily_limit, rl.daily_remaining)  # per day (None when the plan has no cap)
```

## Custom transport and testing

The HTTP layer sits behind a small `toktokdata.Transport` protocol (`request(method, url, *,
headers, body, timeout) -> TransportResponse` and `close()`). Pass your own with
`TokTokData(transport=...)` to add a proxy, logging or a fake for your tests; the default is
`toktokdata.UrllibTransport`.

## Roadmap

- An async client (`AsyncTokTokData`) is planned; v0.1 is synchronous only. In async code, call
  the client through `asyncio.to_thread(...)` for now.

## Development

```bash
pip install -e ".[dev]"
ruff check . && mypy --strict src && pytest
```

The tests run offline against a fake transport and never call the real API.

## License

MIT
