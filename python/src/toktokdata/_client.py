from __future__ import annotations

import datetime as _dt
import json
import os
import random
import time
from collections.abc import Iterator, Mapping
from typing import Any, Callable, cast

from ._errors import PendingTimeoutError, TokTokDataError, error_from_response, parse_retry_after
from ._transport import Transport, TransportError, TransportResponse, UrllibTransport
from ._types import JSONDict, RateLimit
from ._util import encode_query
from ._version import __version__
from .resources.account import Account
from .resources.agencies import Agencies
from .resources.alerts import Alerts
from .resources.creators import Creators
from .resources.exports import Exports
from .resources.gifters import Gifters
from .resources.gifts import Gifts
from .resources.insights import Insights
from .resources.live import Live
from .resources.rankings import Rankings
from .resources.trends import Trends
from .resources.videos import Videos

DEFAULT_BASE_URL = "https://api.toktokdata.live"
USER_AGENT = f"toktokdata-python/{__version__}"

MAX_RETRY_AFTER = 60.0
"""Longest ``Retry-After`` the retry loop honours, in seconds."""
PENDING_MIN_DELAY = 1.0
PENDING_MAX_DELAY = 120.0
DEFAULT_PENDING_TIMEOUT = 180.0

_NO_RETRY_429_CODES = frozenset({"daily_quota_exceeded"})


class TokTokData:
    """Client for the toktokdata.live API.

    >>> client = TokTokData()  # reads TOKTOKDATA_API_KEY
    >>> board = client.rankings.official("hourly", region="VN")

    :param api_key: API key (``ttk_live_...``). Defaults to ``TOKTOKDATA_API_KEY``.
    :param base_url: API origin. Defaults to ``TOKTOKDATA_BASE_URL`` or
        ``https://api.toktokdata.live``.
    :param timeout: seconds per HTTP request.
    :param max_retries: retries after the first attempt for network errors (GET only),
        ``429`` and retryable ``5xx`` (GET only).
    :param pending_timeout: how long 202-aware methods wait for data, in seconds.
    :param transport: custom HTTP transport (see :class:`toktokdata.Transport`).
    """

    def __init__(
        self,
        api_key: str | None = None,
        base_url: str | None = None,
        timeout: float = 30.0,
        max_retries: int = 2,
        *,
        pending_timeout: float = DEFAULT_PENDING_TIMEOUT,
        transport: Transport | None = None,
    ) -> None:
        key = api_key if api_key is not None else os.environ.get("TOKTOKDATA_API_KEY")
        key = (key or "").strip()
        if not key:
            raise TokTokDataError(
                "No API key: pass api_key=... or set the TOKTOKDATA_API_KEY environment "
                "variable. Create a key at https://toktokdata.live (Settings -> API keys).",
                code="missing_api_key",
            )
        if max_retries < 0:
            raise ValueError("max_retries must be >= 0")
        if timeout <= 0:
            raise ValueError("timeout must be > 0")
        self._api_key = key
        base = base_url or os.environ.get("TOKTOKDATA_BASE_URL") or DEFAULT_BASE_URL
        self.base_url = base.rstrip("/")
        self.timeout = float(timeout)
        self.max_retries = int(max_retries)
        self.pending_timeout = float(pending_timeout)
        self.last_rate_limit: RateLimit | None = None
        """``X-RateLimit-*`` headers of the last response that carried them."""
        self._transport: Transport = transport if transport is not None else UrllibTransport()
        # Indirections so tests can run the retry and wait loops without real time.
        self._sleep: Callable[[float], None] = time.sleep
        self._clock: Callable[[], float] = time.monotonic

        self.rankings = Rankings(self)
        self.creators = Creators(self)
        self.agencies = Agencies(self)
        self.insights = Insights(self)
        self.videos = Videos(self)
        self.trends = Trends(self)
        self.gifts = Gifts(self)
        self.gifters = Gifters(self)
        self.live = Live(self)
        self.alerts = Alerts(self)
        self.exports = Exports(self)
        self.account = Account(self)

    # -- lifecycle -------------------------------------------------------------------------

    def close(self) -> None:
        self._transport.close()

    def __enter__(self) -> TokTokData:
        return self

    def __exit__(self, *exc_info: object) -> None:
        self.close()

    def __repr__(self) -> str:
        return f"TokTokData(base_url={self.base_url!r}, api_key='ttk_***')"

    # -- public escape hatch ---------------------------------------------------------------

    def request(
        self,
        method: str,
        path: str,
        *,
        query: Mapping[str, Any] | None = None,
        body: Any = None,
    ) -> Any:
        """Call any route (``path`` starts with ``/v1/``) with auth, retries and error mapping.

        Returns the parsed JSON body (``None`` for an empty body). A ``202`` is returned as is.
        """
        resp = self._send(method.upper(), path, query=query, body=body)
        return self._parse(resp)

    # -- internals used by the resources ---------------------------------------------------

    def _get(self, path: str, query: Mapping[str, Any] | None = None) -> JSONDict:
        return cast(JSONDict, self._parse(self._send("GET", path, query=query)))

    def _post(self, path: str, body: Any = None) -> JSONDict:
        return cast(JSONDict, self._parse(self._send("POST", path, body=body)))

    def _patch(self, path: str, body: Any) -> JSONDict:
        return cast(JSONDict, self._parse(self._send("PATCH", path, body=body)))

    def _delete(self, path: str) -> None:
        self._send("DELETE", path)

    def _get_bytes(self, path: str) -> bytes:
        return self._send("GET", path, accept="*/*").body

    def _get_pending(
        self, path: str, query: Mapping[str, Any] | None = None, *, wait: bool = True
    ) -> JSONDict:
        """GET a 202-aware route; with ``wait`` poll until it answers 200."""
        started = self._clock()
        deadline = started + self.pending_timeout
        while True:
            resp = self._send("GET", path, query=query)
            body = self._parse(resp)
            if resp.status != 202 or not wait:
                return cast(JSONDict, body)
            delay = pending_delay(body, resp)
            remaining = deadline - self._clock()
            if remaining <= 0:
                job_id = body.get("jobId") if isinstance(body, dict) else None
                status = body.get("status") if isinstance(body, dict) else None
                waited = self._clock() - started
                raise PendingTimeoutError(
                    f"{path} was still {status or 'pending'} after {waited:.0f}s; "
                    "ask again later or pass wait=False to get the 202 body",
                    job_id=str(job_id) if job_id is not None else None,
                    last_body=body,
                    waited=waited,
                )
            self._sleep(min(delay, remaining))

    def _paginate(
        self, fetch: Callable[[str | None], JSONDict], cursor: str | None = None
    ) -> Iterator[JSONDict]:
        """Yield whole pages, following ``data.nextCursor`` until it is null."""
        seen: set[str] = set()
        while True:
            page = fetch(cursor)
            yield page
            data = page.get("data") if isinstance(page, dict) else None
            next_cursor = data.get("nextCursor") if isinstance(data, dict) else None
            if not next_cursor or not isinstance(next_cursor, str) or next_cursor in seen:
                return
            seen.add(next_cursor)
            cursor = next_cursor

    def _send(
        self,
        method: str,
        path: str,
        *,
        query: Mapping[str, Any] | None = None,
        body: Any = None,
        accept: str = "application/json",
    ) -> TransportResponse:
        qs = encode_query(query)
        url = self.base_url + path + (f"?{qs}" if qs else "")
        headers = {
            "Authorization": f"Bearer {self._api_key}",
            "Accept": accept,
            "User-Agent": USER_AGENT,
        }
        data: bytes | None = None
        if body is not None:
            data = json.dumps(body, separators=(",", ":"), default=_json_default).encode()
            headers["Content-Type"] = "application/json"

        attempt = 0
        while True:
            try:
                resp = self._transport.request(
                    method, url, headers=headers, body=data, timeout=self.timeout
                )
            except TransportError as exc:
                # Without a response we cannot know whether a POST/PATCH/DELETE ran.
                if method == "GET" and attempt < self.max_retries:
                    self._sleep(backoff(attempt))
                    attempt += 1
                    continue
                raise TokTokDataError(
                    f"{method} {path} failed: {exc}",
                    code="timeout" if exc.timeout else "network_error",
                    retryable=method == "GET",
                ) from exc

            info = RateLimit.from_headers(resp.headers)
            if info is not None:
                self.last_rate_limit = info
            if 200 <= resp.status < 300:
                return resp

            err = error_from_response(resp)
            if attempt < self.max_retries and should_retry(method, err):
                delay = (
                    min(err.retry_after, MAX_RETRY_AFTER)
                    if err.retry_after is not None
                    else backoff(attempt)
                )
                self._sleep(delay)
                attempt += 1
                continue
            raise err

    @staticmethod
    def _parse(resp: TransportResponse) -> Any:
        if resp.status == 204 or not resp.body.strip():
            return None
        try:
            return json.loads(resp.body.decode("utf-8"))
        except ValueError as exc:
            raise TokTokDataError(
                f"HTTP {resp.status}: response is not valid JSON",
                status=resp.status,
                code="invalid_response",
                request_id=resp.header("x-request-id"),
            ) from exc


def should_retry(method: str, err: TokTokDataError) -> bool:
    if err.status == 429:
        # The rate limiter rejects before the handler runs, so any method may retry.
        return err.code not in _NO_RETRY_429_CODES
    if err.status >= 500:
        return method == "GET" and err.retryable
    return False


def backoff(attempt: int) -> float:
    """0.5 s, 1 s, 2 s ... (capped at 8 s) plus up to 25% jitter."""
    base = min(0.5 * (1 << attempt), 8.0)
    return base + random.uniform(0, base * 0.25)


def pending_delay(body: Any, resp: TransportResponse) -> float:
    """Seconds to wait before asking a 202 route again, clamped to 1-120 s."""
    delay: float | None = None
    if isinstance(body, dict):
        value = body.get("retryAfterSeconds")
        if isinstance(value, (int, float)) and not isinstance(value, bool):
            delay = float(value)
    if delay is None:
        delay = parse_retry_after(resp.header("retry-after"))
    if delay is None:
        delay = 2.0
    return max(PENDING_MIN_DELAY, min(PENDING_MAX_DELAY, delay))


def _json_default(value: Any) -> Any:
    if isinstance(value, (_dt.datetime, _dt.date)):
        return value.isoformat()
    raise TypeError(f"Object of type {type(value).__name__} is not JSON serializable")
