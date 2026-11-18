from __future__ import annotations

import pytest

from conftest import FakeClock, FakeTransport, make_client, ok
from toktokdata import TokTokData, TokTokDataError, TransportError


def rate_limited(code: str = "rate_limit_exceeded") -> dict[str, object]:
    return {"code": code, "message": "API key rate limit exceeded.", "retryable": True}


def test_429_is_retried_after_retry_after(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(429, rate_limited(), headers={"Retry-After": "3"})
    transport.add(200, ok({"n": 1}))
    assert client.rankings.regions() == ok({"n": 1})
    assert len(transport.calls) == 2
    assert clock.sleeps == [3.0]


def test_retry_after_is_capped_at_60s(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(429, rate_limited(), headers={"Retry-After": "3600"})
    transport.add(200, ok())
    client.rankings.regions()
    assert clock.sleeps == [60.0]


def test_429_without_retry_after_uses_backoff(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(429, rate_limited())
    transport.add(429, rate_limited())
    transport.add(200, ok())
    client.rankings.regions()
    assert len(clock.sleeps) == 2
    assert 0.5 <= clock.sleeps[0] <= 0.625
    assert 1.0 <= clock.sleeps[1] <= 1.25


def test_retries_are_bounded_by_max_retries(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    for _ in range(3):
        transport.add(429, rate_limited(), headers={"Retry-After": "1"})
    with pytest.raises(TokTokDataError) as info:
        client.rankings.regions()
    assert info.value.is_rate_limited
    assert len(transport.calls) == 3  # first attempt + max_retries=2
    assert clock.sleeps == [1.0, 1.0]


def test_max_retries_zero(transport: FakeTransport, clock: FakeClock) -> None:
    client = make_client(transport, clock, max_retries=0)
    transport.add(429, rate_limited(), headers={"Retry-After": "1"})
    with pytest.raises(TokTokDataError):
        client.rankings.regions()
    assert len(transport.calls) == 1
    assert clock.sleeps == []


def test_402_is_never_retried(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(
        402,
        {"code": "credit_cap_exceeded", "message": "no credits", "retryable": False},
        headers={"Retry-After": "1"},
    )
    with pytest.raises(TokTokDataError) as info:
        client.creators.search(q="linh")
    assert info.value.is_payment_required
    assert len(transport.calls) == 1
    assert clock.sleeps == []


def test_daily_quota_is_not_retried(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(429, rate_limited("daily_quota_exceeded"), headers={"Retry-After": "40000"})
    with pytest.raises(TokTokDataError) as info:
        client.rankings.regions()
    assert info.value.code == "daily_quota_exceeded"
    assert info.value.retry_after == 40000.0
    assert len(transport.calls) == 1


@pytest.mark.parametrize("status", [400, 401, 403, 404, 409, 422])
def test_other_4xx_are_not_retried(
    client: TokTokData, transport: FakeTransport, status: int
) -> None:
    transport.add(status, {"code": "x", "message": "x", "retryable": False})
    with pytest.raises(TokTokDataError):
        client.rankings.regions()
    assert len(transport.calls) == 1


def test_retryable_5xx_is_retried(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(503, {"code": "collection_saturated", "message": "busy", "retryable": True})
    transport.add(500, raw=b"Internal Server Error")
    transport.add(200, ok())
    assert client.rankings.regions() == ok()
    assert len(transport.calls) == 3
    assert len(clock.sleeps) == 2


def test_5xx_with_retryable_false_is_not_retried(
    client: TokTokData, transport: FakeTransport
) -> None:
    transport.add(
        503,
        {"code": "live_playback_not_configured", "message": "off", "retryable": False},
    )
    with pytest.raises(TokTokDataError) as info:
        client.live.playback("linh.live")
    assert info.value.code == "live_playback_not_configured"
    assert len(transport.calls) == 1


def test_post_5xx_is_not_retried(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(503, {"code": "unavailable", "message": "x", "retryable": True})
    with pytest.raises(TokTokDataError):
        client.exports.create(dataset="live_sessions", format="csv")
    assert len(transport.calls) == 1


def test_post_429_is_retried(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(429, rate_limited(), headers={"Retry-After": "2"})
    transport.add(202, {"queued": True, "jobId": "j1"})
    assert client.live.refresh("linh.live") == {"queued": True, "jobId": "j1"}
    assert len(transport.calls) == 2
    assert clock.sleeps == [2.0]


def test_get_network_error_is_retried(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add_error(TransportError("connection reset"))
    transport.add_error(TransportError("timed out", timeout=True))
    transport.add(200, ok())
    assert client.rankings.regions() == ok()
    assert len(transport.calls) == 3


def test_get_network_error_gives_up(client: TokTokData, transport: FakeTransport) -> None:
    for _ in range(3):
        transport.add_error(TransportError("timed out", timeout=True))
    with pytest.raises(TokTokDataError) as info:
        client.rankings.regions()
    assert info.value.code == "timeout"
    assert info.value.status == 0
    assert isinstance(info.value.__cause__, TransportError)


@pytest.mark.parametrize("method", ["POST", "PATCH", "DELETE"])
def test_non_get_network_error_is_not_retried(
    client: TokTokData, transport: FakeTransport, method: str
) -> None:
    transport.add_error(TransportError("connection reset"))
    with pytest.raises(TokTokDataError) as info:
        if method == "POST":
            client.alerts.test_rule("r1")
        elif method == "PATCH":
            client.alerts.update_rule("r1", {"enabled": False})
        else:
            client.alerts.delete_rule("r1")
    assert info.value.code == "network_error"
    assert len(transport.calls) == 1
