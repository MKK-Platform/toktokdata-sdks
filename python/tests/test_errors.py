from __future__ import annotations

import pytest

from conftest import FakeClock, FakeTransport, make_client
from toktokdata import TokTokDataError


def _raise(transport: FakeTransport, clock: FakeClock) -> TokTokDataError:
    client = make_client(transport, clock, max_retries=0)
    with pytest.raises(TokTokDataError) as info:
        client.rankings.official("daily")
    return info.value


def test_envelope_is_mapped(transport: FakeTransport, clock: FakeClock) -> None:
    envelope = {
        "code": "credit_cap_exceeded",
        "message": "The credit wallet cannot cover this call.",
        "requestId": "req-123",
        "retryable": False,
        "resetAt": "2026-12-01T00:00:00Z",
    }
    transport.add(402, envelope)
    err = _raise(transport, clock)
    assert err.status == 402
    assert err.code == "credit_cap_exceeded"
    assert err.message == "The credit wallet cannot cover this call."
    assert err.request_id == "req-123"
    assert err.retryable is False
    assert err.retry_after is None
    assert err.details == envelope
    assert err.details["resetAt"] == "2026-12-01T00:00:00Z"
    assert err.is_payment_required
    assert not err.is_rate_limited
    assert "402 credit_cap_exceeded" in str(err)
    assert "req-123" in str(err)


@pytest.mark.parametrize(
    ("status", "code", "helper"),
    [
        (401, "invalid_api_key", "is_unauthorized"),
        (403, "insufficient_scope", "is_forbidden"),
        (403, "module_not_entitled", "is_forbidden"),
        (404, "not_found", "is_not_found"),
        (429, "daily_quota_exceeded", "is_rate_limited"),
        (402, "credit_cap_exceeded", "is_payment_required"),
    ],
)
def test_helpers(
    transport: FakeTransport, clock: FakeClock, status: int, code: str, helper: str
) -> None:
    transport.add(status, {"code": code, "message": "nope", "retryable": False})
    err = _raise(transport, clock)
    helpers = [
        "is_rate_limited",
        "is_payment_required",
        "is_forbidden",
        "is_unauthorized",
        "is_not_found",
    ]
    for name in helpers:
        assert getattr(err, name) is (name == helper), name


def test_non_json_error_body(transport: FakeTransport, clock: FakeClock) -> None:
    transport.add(502, raw=b"<html><body>Bad Gateway</body></html>")
    err = _raise(transport, clock)
    assert err.status == 502
    assert err.code == "http_error"
    assert "Bad Gateway" in err.message
    assert err.retryable is True
    assert err.details == {"body": "<html><body>Bad Gateway</body></html>"}


def test_empty_error_body(transport: FakeTransport, clock: FakeClock) -> None:
    transport.add(404, raw=b"", headers={"X-Request-Id": "hdr-1"})
    err = _raise(transport, clock)
    assert err.code == "http_error"
    assert err.message == "HTTP 404"
    assert err.request_id == "hdr-1"
    assert err.is_not_found
    assert err.retryable is False


def test_json_without_envelope(transport: FakeTransport, clock: FakeClock) -> None:
    transport.add(400, {"detail": "bad"})
    err = _raise(transport, clock)
    assert err.code == "http_error"
    assert err.details == {"detail": "bad"}


def test_retry_after_header_is_exposed(transport: FakeTransport, clock: FakeClock) -> None:
    transport.add(
        429,
        {"code": "rate_limit_exceeded", "message": "slow down", "retryable": True},
        headers={"Retry-After": "7"},
    )
    err = _raise(transport, clock)
    assert err.retry_after == 7.0
    assert err.is_rate_limited


def test_redirect_is_an_error(transport: FakeTransport, clock: FakeClock) -> None:
    transport.add(302, raw=b"", headers={"Location": "https://elsewhere.test/"})
    err = _raise(transport, clock)
    assert err.status == 302
    assert err.code == "http_error"
