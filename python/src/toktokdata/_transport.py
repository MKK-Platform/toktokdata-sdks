"""The HTTP layer behind a small interface, so tests (or callers) can swap it out.

The default transport uses only ``urllib`` from the standard library.
"""

from __future__ import annotations

import http.client
import socket
import urllib.error
import urllib.request
from collections.abc import Mapping
from dataclasses import dataclass, field
from typing import Any, Protocol


@dataclass(frozen=True)
class TransportResponse:
    """One HTTP response as the client sees it.

    ``headers`` keys are lower-cased.
    """

    status: int
    headers: Mapping[str, str] = field(default_factory=dict)
    body: bytes = b""

    def header(self, name: str) -> str | None:
        return self.headers.get(name.lower())


class TransportError(Exception):
    """No HTTP response was received (DNS, connection refused, reset, timeout ...)."""

    def __init__(self, message: str, *, timeout: bool = False) -> None:
        super().__init__(message)
        self.timeout = timeout


class Transport(Protocol):
    """What the client needs from an HTTP library.

    ``request`` must return every HTTP response, including 4xx/5xx, as a
    :class:`TransportResponse`, raise :class:`TransportError` when no response arrived,
    and must not follow redirects (the ``Authorization`` header would travel with them).
    """

    def request(
        self,
        method: str,
        url: str,
        *,
        headers: Mapping[str, str],
        body: bytes | None,
        timeout: float,
    ) -> TransportResponse: ...

    def close(self) -> None: ...


def lower_headers(headers: Any) -> dict[str, str]:
    if headers is None:
        return {}
    return {str(k).lower(): str(v) for k, v in headers.items()}


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(
        self, req: Any, fp: Any, code: int, msg: str, headers: Any, newurl: str
    ) -> None:
        return None


class UrllibTransport:
    """Default transport built on :mod:`urllib.request`."""

    def __init__(self) -> None:
        self._opener = urllib.request.build_opener(_NoRedirect())

    def request(
        self,
        method: str,
        url: str,
        *,
        headers: Mapping[str, str],
        body: bytes | None,
        timeout: float,
    ) -> TransportResponse:
        req = urllib.request.Request(url, data=body, headers=dict(headers), method=method)
        try:
            with self._opener.open(req, timeout=timeout) as resp:
                return TransportResponse(
                    status=int(resp.status), headers=lower_headers(resp.headers), body=resp.read()
                )
        except urllib.error.HTTPError as exc:
            try:
                data = exc.read()
            except Exception:
                data = b""
            return TransportResponse(status=exc.code, headers=lower_headers(exc.headers), body=data)
        except urllib.error.URLError as exc:
            reason = exc.reason
            is_timeout = isinstance(reason, (socket.timeout, TimeoutError))
            raise TransportError(f"request failed: {reason}", timeout=is_timeout) from exc
        except (socket.timeout, TimeoutError) as exc:
            raise TransportError("request timed out", timeout=True) from exc
        except (http.client.HTTPException, OSError) as exc:
            raise TransportError(f"request failed: {exc}") from exc

    def close(self) -> None:
        return None
