from __future__ import annotations

from typing import Any

import pytest

from conftest import FakeClock, FakeTransport
from toktokdata import PendingTimeoutError, TokTokData


def job(status: str, **extra: Any) -> dict[str, Any]:
    return {"id": "job-1", "dataset": "live_sessions", "format": "csv", "status": status, **extra}


def test_create_sends_only_set_fields_in_camel_case(
    client: TokTokData, transport: FakeTransport
) -> None:
    transport.add(201, job("ready"))
    client.exports.create(dataset="rankings", format="xlsx", board="daily", league_tier="A1")
    assert transport.last.method == "POST"
    assert transport.last.path == "/v1/exports"
    assert transport.last.json == {
        "dataset": "rankings",
        "format": "xlsx",
        "board": "daily",
        "leagueTier": "A1",
    }
    transport.add(201, job("ready"))
    client.exports.create(dataset="gifters", format="csv", region="VN", range_days=7)
    assert transport.last.json == {
        "dataset": "gifters",
        "format": "csv",
        "region": "VN",
        "rangeDays": 7,
    }


def test_wait_until_ready_polls(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(200, job("pending"))
    transport.add(200, job("running"), headers={"Retry-After": "5"})
    transport.add(200, job("ready", rowCount=2000))
    result = client.exports.wait_until_ready("job-1")
    assert result["status"] == "ready"
    assert result["rowCount"] == 2000
    assert [c.path for c in transport.calls] == ["/v1/exports/job-1"] * 3
    assert clock.sleeps == [2.0, 5.0]


def test_wait_until_ready_returns_failed_job(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(200, job("failed", error="xlsx needs a plan that includes Excel"))
    result = client.exports.wait_until_ready("job-1")
    assert result["status"] == "failed"
    assert clock.sleeps == []


def test_wait_until_ready_times_out(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    for _ in range(10):
        transport.add(200, job("pending"))
    with pytest.raises(PendingTimeoutError) as info:
        client.exports.wait_until_ready("job-1", timeout=5, poll_interval=2)
    assert info.value.job_id == "job-1"
    assert info.value.last_body == job("pending")
    assert clock.sleeps == [2.0, 2.0, 1.0]


def test_list_get(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, {"items": []})
    client.exports.list()
    assert transport.last.path == "/v1/exports"
    transport.add(200, job("ready"))
    assert client.exports.get("job-1")["status"] == "ready"
