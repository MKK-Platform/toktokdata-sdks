from __future__ import annotations

import hashlib
import hmac
import json

import pytest

from toktokdata import TokTokDataError, WebhookVerificationError, sign_webhook, verify_webhook

SECRET = "whsec_test_secret"
NOW = 1_762_000_000
PAYLOAD = {"type": "gift", "creatorUniqueId": "linh.live", "diamondCount": 5000}
RAW = json.dumps(PAYLOAD, separators=(",", ":")).encode()


def signed_headers(raw: bytes = RAW, ts: int = NOW, secret: str = SECRET) -> dict[str, str]:
    # Same construction as the API: sha256=HMAC(secret, "<timestamp>.<raw body>").
    digest = hmac.new(secret.encode(), f"{ts}.".encode() + raw, hashlib.sha256).hexdigest()
    return {"X-TokTokData-Signature": f"sha256={digest}", "X-TokTokData-Timestamp": str(ts)}


def test_valid_signature_returns_payload() -> None:
    assert verify_webhook(RAW, signed_headers(), SECRET, now=NOW) == PAYLOAD


def test_str_body_and_lowercase_headers() -> None:
    headers = {k.lower(): v for k, v in signed_headers().items()}
    assert verify_webhook(RAW.decode(), headers, SECRET, now=NOW + 10) == PAYLOAD


def test_sign_webhook_matches_api_format() -> None:
    assert sign_webhook(SECRET, NOW, RAW) == signed_headers()["X-TokTokData-Signature"]


def test_non_ascii_body() -> None:
    raw = json.dumps({"name": "Linh Lê ✨"}, ensure_ascii=False).encode("utf-8")
    assert verify_webhook(raw, signed_headers(raw), SECRET, now=NOW) == {"name": "Linh Lê ✨"}


def test_wrong_secret_is_rejected() -> None:
    with pytest.raises(WebhookVerificationError) as info:
        verify_webhook(RAW, signed_headers(secret="other"), SECRET, now=NOW)
    assert info.value.code == "webhook_invalid_signature"
    assert isinstance(info.value, TokTokDataError)


def test_tampered_body_is_rejected() -> None:
    headers = signed_headers()
    tampered = RAW.replace(b"5000", b"9000")
    with pytest.raises(WebhookVerificationError) as info:
        verify_webhook(tampered, headers, SECRET, now=NOW)
    assert info.value.code == "webhook_invalid_signature"


def test_reserialised_body_is_rejected() -> None:
    pretty = json.dumps(PAYLOAD, indent=2).encode()
    with pytest.raises(WebhookVerificationError):
        verify_webhook(pretty, signed_headers(), SECRET, now=NOW)


@pytest.mark.parametrize("skew", [301, -301, 3600])
def test_expired_or_future_timestamp_is_rejected(skew: int) -> None:
    with pytest.raises(WebhookVerificationError) as info:
        verify_webhook(RAW, signed_headers(), SECRET, now=NOW + skew)
    assert info.value.code == "webhook_timestamp_expired"


def test_custom_tolerance() -> None:
    assert verify_webhook(RAW, signed_headers(), SECRET, now=NOW + 600, tolerance_seconds=900)
    with pytest.raises(WebhookVerificationError):
        verify_webhook(RAW, signed_headers(), SECRET, now=NOW + 60, tolerance_seconds=30)


def test_missing_headers() -> None:
    with pytest.raises(WebhookVerificationError) as info:
        verify_webhook(RAW, {}, SECRET, now=NOW)
    assert info.value.code == "webhook_missing_header"
    headers = signed_headers()
    del headers["X-TokTokData-Timestamp"]
    with pytest.raises(WebhookVerificationError):
        verify_webhook(RAW, headers, SECRET, now=NOW)


def test_bad_timestamp() -> None:
    headers = signed_headers()
    headers["X-TokTokData-Timestamp"] = "yesterday"
    with pytest.raises(WebhookVerificationError) as info:
        verify_webhook(RAW, headers, SECRET, now=NOW)
    assert info.value.code == "webhook_invalid_timestamp"


def test_signed_non_json_body() -> None:
    raw = b"not json"
    with pytest.raises(WebhookVerificationError) as info:
        verify_webhook(raw, signed_headers(raw), SECRET, now=NOW)
    assert info.value.code == "webhook_invalid_json"


def test_empty_secret_is_a_programming_error() -> None:
    with pytest.raises(ValueError):
        verify_webhook(RAW, signed_headers(), "", now=NOW)
