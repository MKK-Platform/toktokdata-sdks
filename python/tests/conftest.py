from __future__ import annotations

import json
import urllib.parse
from collections import deque
from collections.abc import Iterator, Mapping
from dataclasses import dataclass, field
from typing import Any

import pytest

from toktokdata import TokTokData, TransportResponse

API_KEY = "ttk_live_testid_testsecret"
BASE_URL = "https://api.example.test"


@dataclass
class Call:
    method: str
    url: str
    headers: dict[str, str]
    body: bytes | None
    timeout: float

    @property
    def path(self) -> str:
        return urllib.parse.urlsplit(self.url).path

    @property
    def raw_query(self) -> str:
        return urllib.parse.urlsplit(self.url).query

    @property
    def query(self) -> dict[str, str]:
        return dict(urllib.parse.parse_qsl(self.raw_query, keep_blank_values=True))

    @property
    def json(self) -> Any:
        return json.loads(self.body) if self.body else None


@dataclass
class FakeTransport:
    """Scripted responses, recorded requests. An Exception in the queue is raised."""

    queue: deque[Any] = field(default_factory=deque)
    calls: list[Call] = field(default_factory=list)
    closed: bool = False

    def add(
        self,
        status: int = 200,
        body: Any = None,
        *,
        headers: Mapping[str, str] | None = None,
        raw: bytes | None = None,
    ) -> FakeTransport:
        data = raw if raw is not None else (b"" if body is None else json.dumps(body).encode())
        lowered = {k.lower(): v for k, v in (headers or {}).items()}
        self.queue.append(TransportResponse(status=status, headers=lowered, body=data))
        return self

    def add_error(self, exc: Exception) -> FakeTransport:
        self.queue.append(exc)
        return self

    def request(
        self,
        method: str,
        url: str,
        *,
        headers: Mapping[str, str],
        body: bytes | None,
        timeout: float,
    ) -> TransportResponse:
        self.calls.append(Call(method, url, dict(headers), body, timeout))
        if not self.queue:
            raise AssertionError(f"unexpected request {method} {url}")
        item = self.queue.popleft()
        if isinstance(item, Exception):
            raise item
        assert isinstance(item, TransportResponse)
        return item

    def close(self) -> None:
        self.closed = True

    @property
    def last(self) -> Call:
        return self.calls[-1]


class FakeClock:
    """Replaces time.sleep / time.monotonic: sleeping advances the clock instantly."""

    def __init__(self) -> None:
        self.now = 1000.0
        self.sleeps: list[float] = []

    def sleep(self, seconds: float) -> None:
        self.sleeps.append(seconds)
        self.now += seconds

    def monotonic(self) -> float:
        return self.now


@pytest.fixture
def transport() -> FakeTransport:
    return FakeTransport()


@pytest.fixture
def clock() -> FakeClock:
    return FakeClock()


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("TOKTOKDATA_API_KEY", raising=False)
    monkeypatch.delenv("TOKTOKDATA_BASE_URL", raising=False)


def make_client(transport: FakeTransport, clock: FakeClock, **kwargs: Any) -> TokTokData:
    client = TokTokData(api_key=API_KEY, base_url=BASE_URL, transport=transport, **kwargs)
    client._sleep = clock.sleep
    client._clock = clock.monotonic
    return client


@pytest.fixture
def client(transport: FakeTransport, clock: FakeClock) -> Iterator[TokTokData]:
    with make_client(transport, clock) as c:
        yield c


def ok(data: Any = None) -> dict[str, Any]:
    return {"data": data if data is not None else {}, "provenance": {"source": "test"}}
