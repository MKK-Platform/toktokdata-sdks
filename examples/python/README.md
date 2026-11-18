# Python examples (`toktokdata`)

| Script | What it does | Scopes |
|---|---|---|
| `leaderboard.py` | Top 10 of an official board (`hourly` by default) for a region | `rank:read` |
| `creator_report.py` | Profile (waits through `202` while the creator is being looked up), content analysis, and up to 30 recent LIVE sessions read with the `iter_sessions` generator | `creator:read`, `live:read` |
| `export_csv.py` | Creates a `rankings` CSV export, waits until it is ready, downloads it to a file | `export` |
| `webhook_server.py` | Standard-library `http.server` that verifies alert webhook signatures with `verify_webhook` and prints each alert | none (the alert rule needs `webhook:manage`) |

## Setup

Python 3.10 or newer. Run from this folder (the requirements file uses a path relative to it):

```bash
cd examples/python
python -m venv .venv && . .venv/bin/activate     # Windows: .venv\Scripts\activate
pip install -r requirements.txt                  # installs the SDK from ../../python (editable)
export TOKTOKDATA_API_KEY=ttk_live_...           # Settings -> API keys at https://toktokdata.live
```

With [uv](https://docs.astral.sh/uv/): `uv venv && uv pip install -e ../../python`.
Once the SDK is on PyPI, `pip install toktokdata` is enough.

## Run

```bash
python leaderboard.py VN hourly
python creator_report.py tiktok
python export_csv.py VN daily                    # writes the file named by the export job
TOKTOKDATA_WEBHOOK_SECRET=... python webhook_server.py 8080
```

## What they print

```
hourly board, VN, period 2026-11-18T09
  1  Creator One @creator.one                          99,000  LIVE
  2  Creator Two @creator.two                          98,000
 ...
 11  (masked, plan pro)                                41,000
```

`creator_report.py` prints the profile line, follower / video / like counts, the analysis
(average and median plays, engagement rate, posts per week, any `limits` notes) and one line per
LIVE session (start, minutes, peak viewers, diamonds). Numbers depend on live data.

Errors print `HTTP <status> <code>: <message> (requestId ...)`, plus when credits reset on
`402 credit_cap_exceeded`. The SDK already retries `429` (after `Retry-After`) and retryable `5xx`.

## Webhooks

Create an alert rule with a `webhook` channel (dashboard, or `client.alerts.create_rule(...)` with
the `webhook:manage` scope). The signing secret is shown once when the channel is created; put it
in `TOKTOKDATA_WEBHOOK_SECRET`. The server answers `401` when `x-toktokdata-signature` does not
match or `x-toktokdata-timestamp` is older than 300 s. Expose it with a tunnel while testing:
webhook targets must be publicly reachable (private and loopback addresses are refused).
