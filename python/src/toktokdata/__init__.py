"""Official Python SDK for the toktokdata.live API.

>>> from toktokdata import TokTokData
>>> client = TokTokData()  # reads TOKTOKDATA_API_KEY
>>> board = client.rankings.official("hourly", region="VN")
"""

from ._client import DEFAULT_BASE_URL, TokTokData
from ._errors import PendingTimeoutError, TokTokDataError, WebhookVerificationError
from ._transport import Transport, TransportError, TransportResponse, UrllibTransport
from ._types import (
    AlertChannelInput,
    AlertRuleInput,
    JSONDict,
    RateLimit,
)
from ._version import __version__
from ._webhooks import sign_webhook, verify_webhook

__all__ = [
    "DEFAULT_BASE_URL",
    "AlertChannelInput",
    "AlertRuleInput",
    "JSONDict",
    "PendingTimeoutError",
    "RateLimit",
    "TokTokData",
    "TokTokDataError",
    "Transport",
    "TransportError",
    "TransportResponse",
    "UrllibTransport",
    "WebhookVerificationError",
    "__version__",
    "sign_webhook",
    "verify_webhook",
]
