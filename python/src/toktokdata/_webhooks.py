"""Alert webhook signature check.

toktokdata.live signs every webhook delivery with the rule channel's secret::

    x-toktokdata-timestamp: <unix seconds>
    x-toktokdata-signature: sha256=<hex HMAC-SHA256(secret, "<timestamp>.<raw body>")>
"""

from __future__ import annotations

import hashlib
import hmac
import json
import time
from collections.abc import Iterable
from typing import Any, Protocol

from ._errors import WebhookVerificationError

SIGNATURE_HEADER = "x-toktokdata-signature"
TIMESTAMP_HEADER = "x-toktokdata-timestamp"
DEFAULT_TOLERANCE_SECONDS = 300


class HeadersLike(Protocol):
    """Anything with `get(name)` and `items()`: a dict or a framework headers object."""

    def get(self, key: str, /) -> Any: ...

    def items(self) -> Iterable[tuple[Any, Any]]: ...


def _header(headers: HeadersLike, name: str) -> str | None:
    # Works with plain dicts (any case) and with case-insensitive framework headers.
    value = headers.get(name)
    if value is None:
        for key, candidate in headers.items():
            if isinstance(key, str) and key.lower() == name:
                value = candidate
                break
    if isinstance(value, (list, tuple)):
        value = value[0] if value else None
    if isinstance(value, bytes):
        value = value.decode("latin-1")
    return value.strip() if isinstance(value, str) else None


def sign_webhook(secret: str, timestamp: int | str, raw_body: bytes | str) -> str:
    """The ``x-toktokdata-signature`` value for a body (useful in tests of your handler)."""
    body = raw_body.encode("utf-8") if isinstance(raw_body, str) else raw_body
    message = str(timestamp).encode("ascii") + b"." + body
    digest = hmac.new(secret.encode("utf-8"), message, hashlib.sha256).hexdigest()
    return f"sha256={digest}"


def verify_webhook(
    raw_body: bytes | str,
    headers: HeadersLike,
    secret: str,
    *,
    tolerance_seconds: int = DEFAULT_TOLERANCE_SECONDS,
    now: float | None = None,
) -> Any:
    """Check an alert webhook and return its parsed JSON payload.

    :param raw_body: the request body exactly as received (bytes preferred). Do not
        re-serialise parsed JSON: any whitespace change breaks the signature.
    :param headers: the request headers: a dict or a framework headers object (Flask,
        Starlette, Django `request.headers`); lookup is case-insensitive.
    :param secret: the webhook channel's signing secret.
    :param tolerance_seconds: reject timestamps further than this from now (replay guard).
    :param now: current unix time, for tests.
    :raises WebhookVerificationError: ``code`` is ``webhook_missing_header``,
        ``webhook_invalid_timestamp``, ``webhook_timestamp_expired``,
        ``webhook_invalid_signature`` or ``webhook_invalid_json``.
    """
    if not secret:
        raise ValueError("secret is required")
    signature = _header(headers, SIGNATURE_HEADER)
    timestamp = _header(headers, TIMESTAMP_HEADER)
    if not signature or not timestamp:
        raise WebhookVerificationError(
            f"missing {SIGNATURE_HEADER} or {TIMESTAMP_HEADER} header",
            code="webhook_missing_header",
        )
    try:
        sent_at = int(timestamp)
    except ValueError:
        raise WebhookVerificationError(
            f"{TIMESTAMP_HEADER} is not a unix timestamp", code="webhook_invalid_timestamp"
        ) from None
    current = time.time() if now is None else now
    if abs(current - sent_at) > tolerance_seconds:
        raise WebhookVerificationError(
            f"webhook timestamp is outside the {tolerance_seconds}s tolerance",
            code="webhook_timestamp_expired",
        )
    expected = sign_webhook(secret, timestamp, raw_body)
    if not hmac.compare_digest(expected.encode("ascii"), signature.lower().encode("utf-8")):
        raise WebhookVerificationError(
            "webhook signature does not match", code="webhook_invalid_signature"
        )
    body = raw_body.decode("utf-8") if isinstance(raw_body, bytes) else raw_body
    try:
        return json.loads(body)
    except ValueError:
        raise WebhookVerificationError(
            "webhook body is not valid JSON", code="webhook_invalid_json"
        ) from None
