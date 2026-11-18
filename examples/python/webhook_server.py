"""Receive toktokdata.live alert webhooks and verify their signature (stdlib http.server).

Usage: TOKTOKDATA_WEBHOOK_SECRET=... python webhook_server.py [PORT]
No API key needed here; creating the alert rule that points at this server needs webhook:manage.
"""

import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from toktokdata import WebhookVerificationError, verify_webhook


class AlertHandler(BaseHTTPRequestHandler):
    secret = ""  # set in main()

    def do_POST(self) -> None:
        # Verify the exact bytes that were signed: read the raw body, do not re-serialize JSON.
        raw_body = self.rfile.read(int(self.headers.get("Content-Length") or 0))
        try:
            # Checks x-toktokdata-signature (HMAC-SHA256 of "<timestamp>.<body>")
            # and a 300 s timestamp tolerance.
            payload = verify_webhook(raw_body, dict(self.headers.items()), self.secret)
        except WebhookVerificationError as err:
            print(f"rejected webhook: {err}", file=sys.stderr)
            self.send_response(401)
            self.end_headers()
            return

        when = payload.get("occurredAt") or payload.get("deliveredAt")
        print(f"alert {payload['alertId']} from rule \"{payload['rule']['name']}\" at {when}")
        print(json.dumps(payload.get("event"), indent=2, ensure_ascii=False))
        # Answer 2xx quickly and do slow work after responding, so the delivery does not time out.
        self.send_response(204)
        self.end_headers()

    def do_GET(self) -> None:
        self.send_response(405)
        self.end_headers()

    def log_message(self, format: str, *args: object) -> None:  # noqa: A002
        pass  # keep the output to the alerts themselves


def main() -> None:
    secret = os.environ.get("TOKTOKDATA_WEBHOOK_SECRET")
    if not secret:
        sys.exit("Set TOKTOKDATA_WEBHOOK_SECRET to the signing secret shown when you created the webhook channel.")
    AlertHandler.secret = secret
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8080
    print(f"listening on http://localhost:{port}/ (POST only)")
    ThreadingHTTPServer(("", port), AlertHandler).serve_forever()


if __name__ == "__main__":
    main()
