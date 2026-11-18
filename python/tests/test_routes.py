"""Every public operation: method, path, query names and body, checked against the OpenAPI
document of the repo when it is available."""

from __future__ import annotations

import json
import re
from collections.abc import Callable
from pathlib import Path
from typing import Any

import pytest

from conftest import FakeTransport
from toktokdata import TokTokData

OPENAPI = Path(__file__).resolve().parents[2] / "openapi" / "openapi.json"

Route = tuple[str, Callable[[TokTokData], Any], str, str, dict[str, str], Any]

# (template, call, method, expected path, expected query, expected JSON body)
ROUTES: list[Route] = [
    # rankings
    (
        "/v1/rankings/official",
        lambda c: c.rankings.official(
            "hourly", region="VN", period="history", limit=10, live_only=False, league_tier="B2"
        ),
        "GET",
        "/v1/rankings/official",
        {
            "board": "hourly",
            "region": "VN",
            "period": "history",
            "limit": "10",
            "liveOnly": "false",
            "leagueTier": "B2",
        },
        None,
    ),
    (
        "/v1/rankings/regions",
        lambda c: c.rankings.regions(),
        "GET",
        "/v1/rankings/regions",
        {},
        None,
    ),
    (
        "/v1/rankings/history",
        lambda c: c.rankings.history("daily", region="TH", cursor="c", limit=24, league_tier="A1"),
        "GET",
        "/v1/rankings/history",
        {"board": "daily", "region": "TH", "cursor": "c", "limit": "24", "leagueTier": "A1"},
        None,
    ),
    (
        "/v1/rankings/day-bars",
        lambda c: c.rankings.day_bars(region="VN", board="daily", days=30),
        "GET",
        "/v1/rankings/day-bars",
        {"region": "VN", "board": "daily", "days": "30"},
        None,
    ),
    (
        "/v1/rankings/masked/{ref}",
        lambda c: c.rankings.masked("m_abc"),
        "GET",
        "/v1/rankings/masked/m_abc",
        {},
        None,
    ),
    # creators
    (
        "/v1/creators",
        lambda c: c.creators.search(q="linh", limit=5),
        "GET",
        "/v1/creators",
        {"q": "linh", "limit": "5"},
        None,
    ),
    (
        "/v1/creators/{uid}",
        lambda c: c.creators.get("linh.live"),
        "GET",
        "/v1/creators/linh.live",
        {},
        None,
    ),
    (
        "/v1/creators/{uid}/analysis",
        lambda c: c.creators.analysis("linh.live"),
        "GET",
        "/v1/creators/linh.live/analysis",
        {},
        None,
    ),
    (
        "/v1/creators/{uid}/changes",
        lambda c: c.creators.changes("linh.live", limit=3),
        "GET",
        "/v1/creators/linh.live/changes",
        {"limit": "3"},
        None,
    ),
    (
        "/v1/creators/{uid}/followers",
        lambda c: c.creators.followers("linh.live", cursor="c", limit=50),
        "GET",
        "/v1/creators/linh.live/followers",
        {"cursor": "c", "limit": "50"},
        None,
    ),
    (
        "/v1/creators/{uid}/following",
        lambda c: c.creators.following("linh.live", cursor="c", limit=50),
        "GET",
        "/v1/creators/linh.live/following",
        {"cursor": "c", "limit": "50"},
        None,
    ),
    (
        "/v1/creators/{uid}/gifters",
        lambda c: c.creators.gifters("linh.live", window="30d", limit=20),
        "GET",
        "/v1/creators/linh.live/gifters",
        {"window": "30d", "limit": "20"},
        None,
    ),
    (
        "/v1/creators/{uid}/insights",
        lambda c: c.creators.insights("linh.live"),
        "GET",
        "/v1/creators/linh.live/insights",
        {},
        None,
    ),
    (
        "/v1/creators/{uid}/videos",
        lambda c: c.creators.videos("linh.live"),
        "GET",
        "/v1/creators/linh.live/videos",
        {},
        None,
    ),
    # agencies
    (
        "/v1/agencies",
        lambda c: c.agencies.list(
            q="star", region="VN", public_only=True, sort="revenue", limit=10, offset=20
        ),
        "GET",
        "/v1/agencies",
        {
            "q": "star",
            "region": "VN",
            "publicOnly": "true",
            "sort": "revenue",
            "limit": "10",
            "offset": "20",
        },
        None,
    ),
    (
        "/v1/agencies/{agency_id}",
        lambda c: c.agencies.get("ag-1"),
        "GET",
        "/v1/agencies/ag-1",
        {},
        None,
    ),
    # insights
    (
        "/v1/insights/best-time",
        lambda c: c.insights.best_time(region="VN"),
        "GET",
        "/v1/insights/best-time",
        {"region": "VN"},
        None,
    ),
    (
        "/v1/insights/presence",
        lambda c: c.insights.presence(region="VN", hours=24),
        "GET",
        "/v1/insights/presence",
        {"region": "VN", "hours": "24"},
        None,
    ),
    # videos
    ("/v1/videos/{video_id}", lambda c: c.videos.get("7300"), "GET", "/v1/videos/7300", {}, None),
    (
        "/v1/videos/{video_id}/comments",
        lambda c: c.videos.comments("7300", parent_comment_id="p", cursor="c", limit=20),
        "GET",
        "/v1/videos/7300/comments",
        {"parentCommentId": "p", "cursor": "c", "limit": "20"},
        None,
    ),
    # trends, gifts, gifters
    (
        "/v1/trends",
        lambda c: c.trends.list(region="US", type="sound"),
        "GET",
        "/v1/trends",
        {"region": "US", "type": "sound"},
        None,
    ),
    (
        "/v1/gifts",
        lambda c: c.gifts.list(
            q="rose", min_coins=1, max_coins=100, region="VN", sort="name", limit=50
        ),
        "GET",
        "/v1/gifts",
        {
            "q": "rose",
            "minCoins": "1",
            "maxCoins": "100",
            "region": "VN",
            "sort": "name",
            "limit": "50",
        },
        None,
    ),
    (
        "/v1/gifters",
        lambda c: c.gifters.list(window="7d", segment="whale", region="VN", limit=100),
        "GET",
        "/v1/gifters",
        {"window": "7d", "segment": "whale", "region": "VN", "limit": "100"},
        None,
    ),
    # live
    (
        "/v1/live/sessions",
        lambda c: c.live.sessions(creator_unique_id="linh.live", cursor="c", limit=10),
        "GET",
        "/v1/live/sessions",
        {"creatorUniqueId": "linh.live", "cursor": "c", "limit": "10"},
        None,
    ),
    (
        "/v1/live/sessions/{session_id}",
        lambda c: c.live.session("s-1"),
        "GET",
        "/v1/live/sessions/s-1",
        {},
        None,
    ),
    (
        "/v1/live/sessions/{session_id}/timeline",
        lambda c: c.live.timeline("s-1"),
        "GET",
        "/v1/live/sessions/s-1/timeline",
        {},
        None,
    ),
    (
        "/v1/live/sessions/compare",
        lambda c: c.live.compare(["s-1", "s-2"]),
        "GET",
        "/v1/live/sessions/compare",
        {"ids": "s-1,s-2"},
        None,
    ),
    (
        "/v1/live/creators/{uid}/performance",
        lambda c: c.live.performance("linh.live", cursor="c", limit=5),
        "GET",
        "/v1/live/creators/linh.live/performance",
        {"cursor": "c", "limit": "5"},
        None,
    ),
    (
        "/v1/live/creators/{uid}/refresh",
        lambda c: c.live.refresh("linh.live"),
        "POST",
        "/v1/live/creators/linh.live/refresh",
        {},
        None,
    ),
    (
        "/v1/live/creators/{uid}/playback",
        lambda c: c.live.playback("linh.live"),
        "GET",
        "/v1/live/creators/linh.live/playback",
        {},
        None,
    ),
    (
        "/v1/live/creators/{uid}/comments",
        lambda c: c.live.comments("linh.live", after="cur-1"),
        "GET",
        "/v1/live/creators/linh.live/comments",
        {"after": "cur-1"},
        None,
    ),
    # alerts
    ("/v1/alerts/rules", lambda c: c.alerts.list_rules(), "GET", "/v1/alerts/rules", {}, None),
    (
        "/v1/alerts/rules",
        lambda c: c.alerts.create_rule(
            {
                "name": "Whale gifts",
                "scope": "watchlist",
                "eventTypes": ["gift", "envelope"],
                "minDiamondCount": 1000,
                "channels": [{"channelType": "webhook", "target": "https://example.test/hook"}],
            }
        ),
        "POST",
        "/v1/alerts/rules",
        {},
        {
            "name": "Whale gifts",
            "scope": "watchlist",
            "eventTypes": ["gift", "envelope"],
            "minDiamondCount": 1000,
            "channels": [{"channelType": "webhook", "target": "https://example.test/hook"}],
        },
    ),
    (
        "/v1/alerts/rules/{rule_id}",
        lambda c: c.alerts.update_rule("r-1", {"enabled": False}),
        "PATCH",
        "/v1/alerts/rules/r-1",
        {},
        {"enabled": False},
    ),
    (
        "/v1/alerts/rules/{rule_id}",
        lambda c: c.alerts.delete_rule("r-1"),
        "DELETE",
        "/v1/alerts/rules/r-1",
        {},
        None,
    ),
    (
        "/v1/alerts/rules/{rule_id}/test",
        lambda c: c.alerts.test_rule("r-1"),
        "POST",
        "/v1/alerts/rules/r-1/test",
        {},
        None,
    ),
    (
        "/v1/alerts/fires",
        lambda c: c.alerts.fires(rule_id="r-1", since="2026-11-01T00:00:00Z", limit=5),
        "GET",
        "/v1/alerts/fires",
        {"ruleId": "r-1", "since": "2026-11-01T00:00:00Z", "limit": "5"},
        None,
    ),
    (
        "/v1/alerts/deliveries",
        lambda c: c.alerts.deliveries(rule_id="r-1", status="failed", since="2026-11-01", limit=5),
        "GET",
        "/v1/alerts/deliveries",
        {"ruleId": "r-1", "status": "failed", "since": "2026-11-01", "limit": "5"},
        None,
    ),
    ("/v1/alerts/stats", lambda c: c.alerts.stats(), "GET", "/v1/alerts/stats", {}, None),
    # exports
    ("/v1/exports", lambda c: c.exports.list(), "GET", "/v1/exports", {}, None),
    (
        "/v1/exports",
        lambda c: c.exports.create(dataset="live_sessions", format="csv", range_days=30),
        "POST",
        "/v1/exports",
        {},
        {"dataset": "live_sessions", "format": "csv", "rangeDays": 30},
    ),
    ("/v1/exports/{job_id}", lambda c: c.exports.get("j-1"), "GET", "/v1/exports/j-1", {}, None),
    (
        "/v1/exports/{job_id}/download",
        lambda c: c.exports.download("j-1"),
        "GET",
        "/v1/exports/j-1/download",
        {},
        None,
    ),
    # account
    (
        "/v1/entitlements",
        lambda c: c.account.entitlements(period="2026-11"),
        "GET",
        "/v1/entitlements",
        {"period": "2026-11"},
        None,
    ),
    (
        "/v1/usage",
        lambda c: c.account.usage(period="2026-11", limit=10),
        "GET",
        "/v1/usage",
        {"period": "2026-11", "limit": "10"},
        None,
    ),
]


@pytest.mark.parametrize("route", ROUTES, ids=[f"{r[2]} {r[0]}" for r in ROUTES])
def test_route(client: TokTokData, transport: FakeTransport, route: Route) -> None:
    _, call, method, path, query, body = route
    transport.add(200, {"data": {}, "provenance": {}})
    call(client)
    assert transport.last.method == method
    assert transport.last.path == path
    assert transport.last.query == query
    assert transport.last.json == body


def _public_operations() -> dict[tuple[str, str], dict[str, Any]]:
    spec = json.loads(OPENAPI.read_text(encoding="utf-8"))
    ops: dict[tuple[str, str], dict[str, Any]] = {}
    for path, item in spec["paths"].items():
        for method, op in item.items():
            if method in {"get", "post", "patch", "put", "delete"} and op.get("x-public"):
                ops[(method.upper(), path)] = op
    return ops


@pytest.mark.skipif(not OPENAPI.exists(), reason="openapi/openapi.json is not next to the SDK")
def test_every_public_operation_is_covered() -> None:
    ops = _public_operations()
    covered = {(r[2], r[0]) for r in ROUTES}
    assert covered == set(ops), {"missing": set(ops) - covered, "extra": covered - set(ops)}
    assert len(ops) == 45


@pytest.mark.skipif(not OPENAPI.exists(), reason="openapi/openapi.json is not next to the SDK")
def test_query_names_exist_in_openapi() -> None:
    ops = _public_operations()
    for template, _, method, _, query, _ in ROUTES:
        op = ops[(method, template)]
        names = {p["name"] for p in op.get("parameters", []) if p.get("in") == "query"}
        required = {
            p["name"]
            for p in op.get("parameters", [])
            if p.get("in") == "query" and p.get("required")
        }
        assert set(query) <= names, (template, set(query) - names)
        assert required <= set(query), (template, required - set(query))
        # The table exercises every documented query parameter.
        assert set(query) == names, (template, names - set(query))
        path_params = re.findall(r"{(\w+)}", template)
        assert path_params == [p["name"] for p in op.get("parameters", []) if p.get("in") == "path"]
