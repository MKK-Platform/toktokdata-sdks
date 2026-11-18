"""The default urllib transport against a throwaway local HTTP server (no network)."""

from __future__ import annotations

import json
import threading
import time
from collections.abc import Iterator
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest

from toktokdata import TokTokData, TokTokDataError, TransportError, UrllibTransport

seen: dict[str, object] = {}


class Handler(BaseHTTPRequestHandler):
    def log_message(self, format: str, *args: object) -> None:
        return None

    def _send(self, status: int, body: bytes, headers: dict[str, str] | None = None) -> None:
        self.send_response(status)
        for k, v in (headers or {}).items():
            self.send_header(k, v)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        seen["path"] = self.path
        seen["auth"] = self.headers.get("Authorization")
        seen["ua"] = self.headers.get("User-Agent")
        if self.path.startswith("/v1/rankings/regions"):
            self._send(
                200,
                json.dumps({"data": {"regions": ["VN"]}}).encode(),
                {"Content-Type": "application/json", "X-RateLimit-Remaining": "4"},
            )
        elif self.path.startswith("/v1/agencies"):
            body = {"code": "insufficient_scope", "message": "needs agency:read", "requestId": "r"}
            self._send(403, json.dumps(body).encode(), {"Content-Type": "application/json"})
        elif self.path.startswith("/v1/redirect"):
            self._send(302, b"", {"Location": "http://127.0.0.1:1/elsewhere"})
        elif self.path.startswith("/v1/slow"):
            time.sleep(1.0)
            self._send(200, b"{}")
        else:
            self._send(404, b"not here", {"Content-Type": "text/plain"})

    def do_POST(self) -> None:
        length = int(self.headers.get("Content-Length") or 0)
        seen["body"] = self.rfile.read(length)
        seen["content_type"] = self.headers.get("Content-Type")
        self._send(201, json.dumps({"id": "r1"}).encode(), {"Content-Type": "application/json"})


@pytest.fixture(scope="module")
def server_url() -> Iterator[str]:
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_address[1]}"
    finally:
        server.shutdown()
        server.server_close()


def test_success_and_headers(server_url: str) -> None:
    with TokTokData(api_key="ttk_live_a_b", base_url=server_url, max_retries=0) as client:
        assert client.rankings.regions() == {"data": {"regions": ["VN"]}}
        assert client.last_rate_limit is not None
        assert client.last_rate_limit.remaining == 4
    assert seen["auth"] == "Bearer ttk_live_a_b"
    assert str(seen["ua"]).startswith("toktokdata-python/")


def test_error_body_is_read(server_url: str) -> None:
    client = TokTokData(api_key="k", base_url=server_url, max_retries=0)
    with pytest.raises(TokTokDataError) as info:
        client.agencies.list()
    assert info.value.status == 403
    assert info.value.code == "insufficient_scope"
    assert info.value.is_forbidden


def test_post_body(server_url: str) -> None:
    client = TokTokData(api_key="k", base_url=server_url, max_retries=0)
    assert client.alerts.create_rule({"name": "x"}) == {"id": "r1"}
    assert seen["body"] == b'{"name":"x"}'
    assert seen["content_type"] == "application/json"


def test_redirects_are_not_followed(server_url: str) -> None:
    client = TokTokData(api_key="k", base_url=server_url, max_retries=0)
    with pytest.raises(TokTokDataError) as info:
        client.request("GET", "/v1/redirect")
    assert info.value.status == 302


def test_timeout_raises_transport_error(server_url: str) -> None:
    transport = UrllibTransport()
    with pytest.raises(TransportError) as info:
        transport.request("GET", f"{server_url}/v1/slow", headers={}, body=None, timeout=0.2)
    assert info.value.timeout


def test_connection_refused() -> None:
    transport = UrllibTransport()
    with pytest.raises(TransportError):
        transport.request("GET", "http://127.0.0.1:1/v1/x", headers={}, body=None, timeout=2)
