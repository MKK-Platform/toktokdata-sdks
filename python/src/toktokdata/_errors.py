from __future__ import annotations

import email.utils
import json
import time
from collections.abc import Mapping
from typing import Any

from ._transport import TransportResponse


class TokTokDataError(Exception):
    """Every error the SDK raises for a failed API call.

    Attributes mirror the API's error envelope
    ``{"code", "message", "requestId", "retryable", ...}``:

    - ``status``: HTTP status (``0`` when no response was received).
    - ``code``: machine-readable code, e.g. ``rate_limit_exceeded``, ``credit_cap_exceeded``,
      ``insufficient_scope``; ``http_error`` when the body was not a JSON envelope.
    - ``message``: human-readable message.
    - ``request_id``: id to quote to support.
    - ``retryable``: whether the API says a retry may succeed.
    - ``retry_after``: seconds from the ``Retry-After`` header, if any.
    - ``details``: the whole envelope (may carry ``resetAt`` and other extra fields).
    """

    def __init__(
        self,
        message: str,
        *,
        status: int = 0,
        code: str = "http_error",
        request_id: str | None = None,
        retryable: bool = False,
        retry_after: float | None = None,
        details: Mapping[str, Any] | None = None,
    ) -> None:
        super().__init__(message)
        self.status = status
        self.code = code
        self.message = message
        self.request_id = request_id
        self.retryable = retryable
        self.retry_after = retry_after
        self.details: dict[str, Any] = dict(details or {})

    def __str__(self) -> str:
        head = f"{self.status} {self.code}: {self.message}" if self.status else self.message
        if self.request_id:
            head += f" (request id {self.request_id})"
        return head

    def __repr__(self) -> str:
        return (
            f"{type(self).__name__}(status={self.status!r}, code={self.code!r}, "
            f"message={self.message!r}, request_id={self.request_id!r})"
        )

    @property
    def is_rate_limited(self) -> bool:
        """HTTP 429: per-second, per-minute or daily (``daily_quota_exceeded``) limit."""
        return self.status == 429

    @property
    def is_payment_required(self) -> bool:
        """HTTP 402: the credit wallet cannot cover the call (``credit_cap_exceeded``)."""
        return self.status == 402

    @property
    def is_forbidden(self) -> bool:
        """HTTP 403: ``insufficient_scope`` or ``module_not_entitled``."""
        return self.status == 403

    @property
    def is_unauthorized(self) -> bool:
        """HTTP 401: missing, unknown, revoked or suspended key."""
        return self.status == 401

    @property
    def is_not_found(self) -> bool:
        """HTTP 404."""
        return self.status == 404


class PendingTimeoutError(TokTokDataError):
    """A ``202`` read (or an export job) was still not ready when the wait ran out.

    ``job_id`` and ``last_body`` hold the last pending answer.
    """

    def __init__(
        self,
        message: str,
        *,
        job_id: str | None = None,
        last_body: Any = None,
        waited: float = 0.0,
    ) -> None:
        details = last_body if isinstance(last_body, Mapping) else {}
        super().__init__(
            message, status=202, code="pending_timeout", retryable=True, details=details
        )
        self.job_id = job_id
        self.last_body = last_body
        self.waited = waited


class WebhookVerificationError(TokTokDataError):
    """A webhook request failed signature or timestamp verification."""

    def __init__(self, message: str, *, code: str) -> None:
        super().__init__(message, status=0, code=code)


def parse_retry_after(value: str | None, *, now: float | None = None) -> float | None:
    """Seconds from a ``Retry-After`` header (delta-seconds or an HTTP date)."""
    if value is None:
        return None
    value = value.strip()
    if not value:
        return None
    try:
        seconds = float(value)
    except ValueError:
        try:
            parsed = email.utils.parsedate_to_datetime(value)
        except (TypeError, ValueError, IndexError):
            return None
        if parsed is None:
            return None
        seconds = parsed.timestamp() - (time.time() if now is None else now)
    return max(0.0, seconds)


def error_from_response(resp: TransportResponse) -> TokTokDataError:
    retry_after = parse_retry_after(resp.header("retry-after"))
    text = resp.body.decode("utf-8", errors="replace") if resp.body else ""
    envelope: Any = None
    if text:
        try:
            envelope = json.loads(text)
        except ValueError:
            envelope = None
    default_retryable = resp.status == 429 or resp.status >= 500
    header_request_id = resp.header("x-request-id")
    if isinstance(envelope, dict) and isinstance(envelope.get("code"), str):
        retryable = envelope.get("retryable")
        request_id = envelope.get("requestId")
        return TokTokDataError(
            str(envelope.get("message") or f"HTTP {resp.status}"),
            status=resp.status,
            code=envelope["code"],
            request_id=str(request_id) if request_id else header_request_id,
            retryable=retryable if isinstance(retryable, bool) else default_retryable,
            retry_after=retry_after,
            details=envelope,
        )
    snippet = " ".join(text.split())[:200]
    message = f"HTTP {resp.status}" + (f": {snippet}" if snippet else "")
    details: dict[str, Any] = envelope if isinstance(envelope, dict) else {}
    if snippet and not details:
        details = {"body": text[:2000]}
    return TokTokDataError(
        message,
        status=resp.status,
        code="http_error",
        request_id=header_request_id,
        retryable=default_retryable,
        retry_after=retry_after,
        details=details,
    )
