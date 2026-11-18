from __future__ import annotations

from typing import Any

from conftest import FakeClock, FakeTransport
from toktokdata import TokTokData


def page(items: list[Any], next_cursor: str | None, key: str = "items") -> dict[str, Any]:
    return {"data": {key: items, "nextCursor": next_cursor}, "provenance": {}}


def test_iter_sessions_follows_next_cursor(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, page([1, 2], "c2"))
    transport.add(200, page([3, 4], "c3"))
    transport.add(200, page([5], None))
    pages = list(client.live.iter_sessions(creator_unique_id="linh.live", limit=2))
    assert [p["data"]["items"] for p in pages] == [[1, 2], [3, 4], [5]]
    assert [c.query for c in transport.calls] == [
        {"creatorUniqueId": "linh.live", "limit": "2"},
        {"creatorUniqueId": "linh.live", "limit": "2", "cursor": "c2"},
        {"creatorUniqueId": "linh.live", "limit": "2", "cursor": "c3"},
    ]


def test_iterator_is_lazy(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, page([1], "c2"))
    it = client.live.iter_sessions()
    assert transport.calls == []
    first = next(it)
    assert first["data"]["items"] == [1]
    assert len(transport.calls) == 1


def test_iter_history_keeps_board(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, page([{"period": "p1"}], "c2", key="periods"))
    transport.add(200, page([{"period": "p2"}], None, key="periods"))
    pages = list(client.rankings.iter_history("daily", region="VN"))
    assert len(pages) == 2
    assert transport.calls[1].query == {"board": "daily", "region": "VN", "cursor": "c2"}


def test_iter_followers_waits_on_202(
    client: TokTokData, transport: FakeTransport, clock: FakeClock
) -> None:
    transport.add(202, {"status": "resolving", "jobId": "j", "retryAfterSeconds": 2})
    transport.add(200, page(["a"], "c2", key="entries"))
    transport.add(200, page(["b"], None, key="entries"))
    pages = list(client.creators.iter_followers("linh.live", limit=1))
    assert [p["data"]["entries"] for p in pages] == [["a"], ["b"]]
    assert clock.sleeps == [2.0]


def test_iter_following_and_performance_and_video_comments(
    client: TokTokData, transport: FakeTransport
) -> None:
    transport.add(200, page([], None, key="entries"))
    assert len(list(client.creators.iter_following("linh.live"))) == 1
    assert transport.last.path == "/v1/creators/linh.live/following"

    transport.add(200, page([1], "x"))
    transport.add(200, page([2], None))
    assert len(list(client.live.iter_performance("linh.live", limit=1))) == 2
    assert transport.last.path == "/v1/live/creators/linh.live/performance"
    assert transport.last.query == {"limit": "1", "cursor": "x"}

    transport.add(200, page([1], None, key="comments"))
    list(client.videos.iter_comments("7300", parent_comment_id="c-1"))
    assert transport.last.query == {"parentCommentId": "c-1"}


def test_start_cursor_is_honoured(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, page([9], None))
    list(client.live.iter_sessions(cursor="start"))
    assert transport.last.query == {"cursor": "start"}


def test_repeated_cursor_stops(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, page([1], "same"))
    transport.add(200, page([2], "same"))
    pages = list(client.live.iter_sessions())
    assert len(pages) == 2


def test_missing_data_stops(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, {"unexpected": True})
    assert list(client.live.iter_sessions()) == [{"unexpected": True}]
