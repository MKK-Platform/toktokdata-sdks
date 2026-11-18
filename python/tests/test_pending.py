from __future__ import annotations

from collections.abc import Callable
from typing import Any

import pytest

from conftest import FakeClock, FakeTransport, make_client, ok
from toktokdata import PendingTimeoutError, TokTokData, TokTokDataError


def resolving(job_id: str = "job-1", retry_after: Any = 3) -> dict[str, Any]:
    body: dict[str, Any] = {"status": "resolving", "jobId": job_id}
    if retry_after is not None:
        body["retryAfterSeconds"] = retry_after
    return body


PENDING_METHODS: list[tuple[str, Callable[[TokTokData], Any], str]] = [
    ("creators.get", lambda c: c.creators.get("linh.live"), "/v1/creators/linh.live"),
    (
        "creators.followers",
        lambda c: c.creators.followers("linh.live", limit=10),
        "/v1/creators/linh.live/followers",
    ),
    (
        "creators.following",
        lambda c: c.creators.following("linh.live"),
        "/v1/creators/linh.live/following",
    ),
    ("videos.comments", lambda c: c.videos.comments("7300"), "/v1/videos/7300/comments"),
    (
        "live.playback",
        lambda c: c.live.playback("linh.live"),
        "/v1/live/creators/linh.live/playback",
    ),
]


@pytest.mark.parametrize(
    ("name", "call", "path"), PENDING_METHODS, ids=[m[0] for m in PENDING_METHODS]
)
def test_202_is_awaited(
    client: TokTokData,
    transport: FakeTransport,
    clock: FakeClock,
    name: str,
    call: Callable[[TokTokData], Any],
    path: str,
) -> None:
    transport.add(202, resolving(retry_after=4), headers={"Retry-After": "4"})
    transport.add(202, resolving(retry_after=2))
    transport.add(200, ok({"done": True}))
    assert call(client) == ok({"done": True})
    assert [c.path for c in transport.calls] == [path] * 3
    assert clock.sleeps == [4.0, 2.0]


def test_retry_after_header_is_used_without_body_hint(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(202, resolving(retry_after=None), headers={"Retry-After": "9"})
    transport.add(200, ok())
    client.creators.get("linh.live")
    assert clock.sleeps == [9.0]


@pytest.mark.parametrize(("hint", "expected"), [(0, 1.0), (0.2, 1.0), (500, 120.0), (30, 30.0)])
def test_wait_is_clamped(
    client: TokTokData, transport: FakeTransport, clock: FakeClock, hint: float, expected: float
) -> None:
    transport.add(202, resolving(retry_after=hint))
    transport.add(200, ok())
    client.creators.get("linh.live")
    assert clock.sleeps == [expected]


def test_wait_false_returns_the_202_body(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    body = {"status": "saturated", "jobId": "job-9", "retryAfterSeconds": 30}
    transport.add(202, body, headers={"Retry-After": "30"})
    assert client.creators.get("linh.live", wait=False) == body
    assert len(transport.calls) == 1
    assert clock.sleeps == []


def test_pending_timeout(client: TokTokData, transport: FakeTransport, clock: FakeClock) -> None:
    for _ in range(10):
        transport.add(202, resolving("job-7", retry_after=60))
    with pytest.raises(PendingTimeoutError) as info:
        client.creators.get("linh.live")
    err = info.value
    assert isinstance(err, TokTokDataError)
    assert err.job_id == "job-7"
    assert err.code == "pending_timeout"
    assert err.status == 202
    assert err.last_body == resolving("job-7", retry_after=60)
    assert sum(clock.sleeps) == pytest.approx(180.0)
    assert len(transport.calls) == 4  # t=0, 60, 120, 180


def test_pending_timeout_is_configurable(transport: FakeTransport, clock: FakeClock) -> None:
    client = make_client(transport, clock, pending_timeout=5)
    for _ in range(5):
        transport.add(202, resolving(retry_after=3))
    with pytest.raises(PendingTimeoutError):
        client.live.playback("linh.live")
    assert clock.sleeps == [3.0, 2.0]


def test_saturated_is_also_awaited(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(202, {"status": "saturated", "jobId": None, "retryAfterSeconds": 10})
    transport.add(200, ok({"creator": {"uniqueId": "linh.live"}}))
    assert client.creators.get("linh.live")["data"]["creator"]["uniqueId"] == "linh.live"
    assert clock.sleeps == [10.0]


def test_refresh_returns_202_as_is(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(202, {"queued": True, "jobId": "1761994604935-0"})
    assert client.live.refresh("@linh.live") == {"queued": True, "jobId": "1761994604935-0"}
    assert transport.last.method == "POST"
    assert transport.last.path == "/v1/live/creators/linh.live/refresh"
    assert clock.sleeps == []


def test_export_create_returns_202_as_is(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(202, {"id": "job-1", "status": "pending"})
    assert client.exports.create(dataset="rankings", format="csv", board="daily") == {
        "id": "job-1",
        "status": "pending",
    }
    assert len(transport.calls) == 1
