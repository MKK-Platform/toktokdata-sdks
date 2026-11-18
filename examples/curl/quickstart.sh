#!/usr/bin/env bash
# toktokdata.live API quickstart: an official board, a creator profile (waiting through
# "202 still collecting") and the workspace entitlements.
# Usage: ./quickstart.sh [REGION] [CREATOR]      e.g. ./quickstart.sh VN tiktok
set -euo pipefail

: "${TOKTOKDATA_API_KEY:?Set TOKTOKDATA_API_KEY (dashboard Settings -> API keys at https://toktokdata.live)}"
BASE="${TOKTOKDATA_BASE_URL:-https://api.toktokdata.live}"
REGION="${1:-VN}"
CREATOR="${2:-tiktok}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

has_jq() { command -v jq >/dev/null 2>&1; }

# api_get PATH: body in $TMP/body, HTTP status in $STATUS, Retry-After in $RETRY_AFTER.
# Retries 429 up to 3 times; returns 1 (and prints the error envelope) on any 4xx/5xx.
api_get() {
  local attempt
  for attempt in 1 2 3 4; do
    STATUS=$(curl -sS -o "$TMP/body" -D "$TMP/headers" -w '%{http_code}' \
      -H "Authorization: Bearer $TOKTOKDATA_API_KEY" \
      -H 'Accept: application/json' \
      -H 'User-Agent: toktokdata-curl-example' \
      "$BASE$1")
    RETRY_AFTER=$(tr -d '\r' <"$TMP/headers" | awk 'tolower($1)=="retry-after:" {v=$2} END {print v}')
    if [ "$STATUS" = 429 ] && [ "$attempt" -lt 4 ] && ! grep -q daily_quota_exceeded "$TMP/body"; then
      echo "  rate limited, retrying in ${RETRY_AFTER:-1}s" >&2
      sleep "${RETRY_AFTER:-1}"
      continue
    fi
    break
  done
  if [ "$STATUS" -ge 400 ]; then
    echo "  HTTP $STATUS:" >&2
    if has_jq; then
      jq -r '"  \(.code): \(.message) (requestId \(.requestId // "-"))" + (if .resetAt then ", credits reset at \(.resetAt)" else "" end)' "$TMP/body" >&2 || cat "$TMP/body" >&2
    else
      cat "$TMP/body" >&2; echo >&2
    fi
    return 1
  fi
}

# show [JQ_FILTER]: pretty-print $TMP/body with jq, or raw JSON without it.
show() {
  if has_jq; then jq -r "${1:-.}" "$TMP/body"; else cat "$TMP/body"; echo; fi
}

echo "== Official hourly board, $REGION (scope rank:read)"
if api_get "/v1/rankings/official?board=hourly&region=$REGION&limit=10"; then
  show '.data.board
    | "publishable: \(.publishable), period \(.periodKey)",
      (.entries[] | "\(.rank)\t\(if .masked then "(masked, plan \(.unmaskTier))" else "@\(.uniqueId)" end)\t\(.score)\(if .isLive then "\tLIVE" else "" end)")'
fi

echo
echo "== Creator @$CREATOR (scope creator:read)"
deadline=$(( $(date +%s) + 180 ))
while api_get "/v1/creators/$CREATOR"; do
  [ "$STATUS" = 202 ] || break
  # Still collecting: wait retryAfterSeconds (or Retry-After), clamped to 1..120 s.
  wait="$RETRY_AFTER"
  state=resolving
  if has_jq; then
    wait=$(jq -r '.retryAfterSeconds // empty' "$TMP/body")
    state=$(jq -r '.status // "resolving"' "$TMP/body")
  fi
  wait="${wait:-5}"
  [ "$wait" -lt 1 ] && wait=1
  [ "$wait" -gt 120 ] && wait=120
  if [ $(( $(date +%s) + wait )) -gt "$deadline" ]; then
    echo "  still collecting after 180 s, try again later" >&2
    break
  fi
  echo "  202 $state, asking again in ${wait}s"
  sleep "$wait"
done
if [ "${STATUS:-0}" = 200 ]; then
  show '.data.creator | "\(.displayName) (@\(.uniqueId)), \(.region // "?"): \(.followerCount) followers, \(.awemeCount) videos, last refreshed \(.lastRefreshedAt)"'
fi

echo
echo "== Entitlements (scope keys:manage)"
if api_get "/v1/entitlements"; then
  show '.data | "plan \(.planCode): \(.credits.remaining) of \(.credits.monthlyAllotment) credits left, reset \(.credits.resetAt)"'
fi
