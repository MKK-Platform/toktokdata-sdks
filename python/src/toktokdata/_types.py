"""Light type helpers. Responses are returned as the parsed JSON body (plain dicts)."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from typing import Any, Literal, TypedDict

JSONDict = dict[str, Any]
"""A parsed JSON object, e.g. ``{"data": ..., "provenance": {...}}``."""

RankingPeriod = Literal["current", "history"]
GifterWindow = Literal["7d", "30d"]
GifterSegment = Literal["whale", "rising", "regular", "one_time", "occasional"]
AgencySort = Literal["size", "revenue", "recent", "name"]
GiftSort = Literal["coins", "name", "recent"]
TrendType = Literal["hashtag", "sound"]
DeliveryStatus = Literal["delivered", "failed", "capped"]
ExportDataset = Literal["live_sessions", "rankings", "watchlist", "recruit_pipeline", "gifters"]
ExportFormat = Literal["csv", "xlsx"]
AlertScope = Literal["creator", "any", "watchlist", "gifter_watchlist"]
AlertEventType = Literal[
    "gift",
    "envelope",
    "goodyBag",
    "live_start",
    "live_end",
    "host_quiet",
    "league_change",
    "diamond_target",
    "follower_milestone",
    "*",
]
AlertChannelType = Literal["webhook", "email", "in_app", "telegram", "discord", "zalo"]


class AlertChannelInput(TypedDict, total=False):
    """One delivery channel of an alert rule (``channelType`` is required)."""

    channelType: AlertChannelType
    target: str
    enabled: bool


class AlertRuleInput(TypedDict, total=False):
    """Body of ``alerts.create_rule`` (``name`` is required) and ``alerts.update_rule``."""

    name: str
    creatorUniqueId: str | None
    scope: AlertScope
    region: str | None
    eventTypes: list[AlertEventType]
    minDiamondCount: int | None
    minFollowerMilestone: int | None
    enabled: bool
    channels: list[AlertChannelInput]


def _int_header(headers: Mapping[str, str], name: str) -> int | None:
    value = headers.get(name)
    if value is None:
        return None
    try:
        return int(value.strip())
    except ValueError:
        return None


@dataclass(frozen=True)
class RateLimit:
    """``X-RateLimit-*`` headers of the last response that carried them.

    ``limit``/``remaining`` are the per-second budget; ``minute_*`` and ``daily_*`` the
    per-minute and per-day ones. A field is ``None`` when the header was absent
    (for example no daily cap on the plan).
    """

    limit: int | None = None
    remaining: int | None = None
    minute_limit: int | None = None
    minute_remaining: int | None = None
    daily_limit: int | None = None
    daily_remaining: int | None = None

    @classmethod
    def from_headers(cls, headers: Mapping[str, str]) -> RateLimit | None:
        """Build from lower-cased headers; ``None`` when no ``X-RateLimit-*`` header is set."""
        info = cls(
            limit=_int_header(headers, "x-ratelimit-limit"),
            remaining=_int_header(headers, "x-ratelimit-remaining"),
            minute_limit=_int_header(headers, "x-ratelimit-minute-limit"),
            minute_remaining=_int_header(headers, "x-ratelimit-minute-remaining"),
            daily_limit=_int_header(headers, "x-ratelimit-daily-limit"),
            daily_remaining=_int_header(headers, "x-ratelimit-daily-remaining"),
        )
        if info == cls():
            return None
        return info


__all__ = [
    "AgencySort",
    "AlertChannelInput",
    "AlertChannelType",
    "AlertEventType",
    "AlertRuleInput",
    "AlertScope",
    "DeliveryStatus",
    "ExportDataset",
    "ExportFormat",
    "GiftSort",
    "GifterSegment",
    "GifterWindow",
    "JSONDict",
    "RankingPeriod",
    "RateLimit",
    "TrendType",
]
