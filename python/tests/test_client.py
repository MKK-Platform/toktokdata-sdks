from __future__ import annotations

import datetime as dt

import pytest

import toktokdata
from conftest import API_KEY, BASE_URL, FakeClock, FakeTransport, make_client, ok
from toktokdata import RateLimit, TokTokData, TokTokDataError


def test_missing_key_fails_at_construction() -> None:
    with pytest.raises(TokTokDataError) as info:
        TokTokData(transport=FakeTransport())
    assert info.value.code == "missing_api_key"
    assert "TOKTOKDATA_API_KEY" in str(info.value)


def test_key_and_base_url_from_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TOKTOKDATA_API_KEY", "ttk_live_env_key")
    monkeypatch.setenv("TOKTOKDATA_BASE_URL", "https://staging.example.test/")
    transport = FakeTransport().add(200, ok())
    client = TokTokData(transport=transport)
    client.rankings.regions()
    assert transport.last.url == "https://staging.example.test/v1/rankings/regions"
    assert transport.last.headers["Authorization"] == "Bearer ttk_live_env_key"


def test_explicit_options_win_over_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TOKTOKDATA_API_KEY", "ttk_live_env_key")
    transport = FakeTransport().add(200, ok())
    client = TokTokData(api_key="ttk_live_arg", base_url=BASE_URL, transport=transport)
    client.rankings.regions()
    assert transport.last.headers["Authorization"] == "Bearer ttk_live_arg"
    assert transport.last.url.startswith(BASE_URL)


def test_default_base_url() -> None:
    client = TokTokData(api_key=API_KEY, transport=FakeTransport())
    assert client.base_url == "https://api.toktokdata.live"
    assert toktokdata.DEFAULT_BASE_URL == "https://api.toktokdata.live"


def test_auth_and_default_headers(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, ok({"boards": []}))
    body = client.rankings.official("hourly", region="VN")
    assert body == ok({"boards": []})
    call = transport.last
    assert call.method == "GET"
    assert call.headers["Authorization"] == f"Bearer {API_KEY}"
    assert call.headers["Accept"] == "application/json"
    assert call.headers["User-Agent"] == f"toktokdata-python/{toktokdata.__version__}"
    assert "Content-Type" not in call.headers
    assert call.body is None
    assert call.timeout == 30.0


def test_json_body_and_content_type(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(201, {"id": "r1", "name": "Big gifts"})
    rule = client.alerts.create_rule({"name": "Big gifts", "eventTypes": ["gift"]})
    assert rule["id"] == "r1"
    call = transport.last
    assert call.method == "POST"
    assert call.headers["Content-Type"] == "application/json"
    assert call.json == {"name": "Big gifts", "eventTypes": ["gift"]}


def test_version_and_timeout_option(transport: FakeTransport, clock: FakeClock) -> None:
    assert toktokdata.__version__ == "0.1.0"
    client = make_client(transport, clock, timeout=5)
    transport.add(200, ok())
    client.trends.list()
    assert transport.last.timeout == 5.0


def test_repr_never_shows_the_key(client: TokTokData) -> None:
    assert API_KEY not in repr(client)
    assert "testsecret" not in repr(client)


def test_context_manager_closes_transport(transport: FakeTransport, clock: FakeClock) -> None:
    with make_client(transport, clock) as c:
        assert isinstance(c, TokTokData)
    assert transport.closed


def test_query_encoding(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, ok())
    client.rankings.official(
        "league_a1", region="VN", period=None, limit=50, live_only=True, league_tier="A1"
    )
    assert transport.last.query == {
        "board": "league_a1",
        "region": "VN",
        "limit": "50",
        "liveOnly": "true",
        "leagueTier": "A1",
    }
    transport.add(200, ok())
    client.agencies.list(public_only=False, offset=0)
    assert transport.last.query == {"publicOnly": "false", "offset": "0"}


def test_none_params_are_omitted(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, ok())
    client.gifts.list()
    assert transport.last.raw_query == ""
    assert transport.last.url == f"{BASE_URL}/v1/gifts"


def test_compare_ids_are_comma_joined(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, ok())
    client.live.compare(["s1", "s2", "s3"])
    assert transport.last.raw_query == "ids=s1,s2,s3"
    transport.add(200, ok())
    client.live.compare("a,b")
    assert transport.last.query == {"ids": "a,b"}


@pytest.mark.parametrize("ids", [["only-one"], ["a", "b", "c", "d", "e"], ["a", ""]])
def test_compare_rejects_wrong_count(client: TokTokData, ids: list[str]) -> None:
    with pytest.raises(ValueError):
        client.live.compare(ids)


def test_special_characters_are_encoded(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, ok())
    client.creators.search(q="linh live & co")
    assert transport.last.raw_query == "q=linh%20live%20%26%20co"
    assert transport.last.query == {"q": "linh live & co"}


def test_datetime_params_are_iso(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, ok())
    since = dt.datetime(2026, 11, 1, 9, 30, tzinfo=dt.timezone.utc)
    client.alerts.fires(rule_id="r1", since=since, limit=10)
    assert transport.last.query == {
        "ruleId": "r1",
        "since": "2026-11-01T09:30:00+00:00",
        "limit": "10",
    }


def test_path_params_are_encoded_and_at_is_stripped(
    client: TokTokData, transport: FakeTransport
) -> None:
    transport.add(200, ok())
    client.creators.get("@linh.live")
    assert transport.last.path == "/v1/creators/linh.live"
    transport.add(200, ok())
    client.videos.get("a/b?c")
    assert transport.last.url == f"{BASE_URL}/v1/videos/a%2Fb%3Fc"


@pytest.mark.parametrize("bad", ["", "  ", "@"])
def test_empty_path_param_is_rejected(client: TokTokData, bad: str) -> None:
    with pytest.raises(ValueError):
        client.creators.get(bad)


def test_required_board(client: TokTokData) -> None:
    with pytest.raises(ValueError):
        client.rankings.official("")
    with pytest.raises(ValueError):
        client.rankings.history("")


def test_delete_returns_none_on_204(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(204)
    assert client.alerts.delete_rule("r1") is None
    assert transport.last.method == "DELETE"
    assert transport.last.path == "/v1/alerts/rules/r1"


def test_download_returns_bytes(client: TokTokData, transport: FakeTransport) -> None:
    csv = b"rank,creator\n1,linh.live\n"
    transport.add(200, raw=csv, headers={"Content-Type": "text/csv"})
    assert client.exports.download("job-1") == csv
    assert transport.last.path == "/v1/exports/job-1/download"
    assert transport.last.headers["Accept"] == "*/*"


def test_last_rate_limit(client: TokTokData, transport: FakeTransport) -> None:
    assert client.last_rate_limit is None
    transport.add(
        200,
        ok(),
        headers={
            "X-RateLimit-Limit": "10",
            "X-RateLimit-Remaining": "9",
            "X-RateLimit-Minute-Limit": "300",
            "X-RateLimit-Minute-Remaining": "250",
            "X-RateLimit-Daily-Limit": "5000",
            "X-RateLimit-Daily-Remaining": "4321",
        },
    )
    client.trends.list(region="US", type="hashtag")
    assert client.last_rate_limit == RateLimit(
        limit=10,
        remaining=9,
        minute_limit=300,
        minute_remaining=250,
        daily_limit=5000,
        daily_remaining=4321,
    )
    # A response without the headers keeps the last known values.
    transport.add(200, ok())
    client.trends.list()
    assert client.last_rate_limit is not None
    assert client.last_rate_limit.remaining == 9


def test_rate_limit_partial_headers() -> None:
    info = RateLimit.from_headers({"x-ratelimit-limit": "5", "x-ratelimit-remaining": "x"})
    assert info == RateLimit(limit=5)
    assert RateLimit.from_headers({}) is None


def test_request_escape_hatch(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, ok({"x": 1}))
    body = client.request("get", "/v1/rankings/regions", query={"a": None, "b": True})
    assert body == ok({"x": 1})
    assert transport.last.method == "GET"
    assert transport.last.raw_query == "b=true"


def test_invalid_json_on_success(client: TokTokData, transport: FakeTransport) -> None:
    transport.add(200, raw=b"<html>oops</html>")
    with pytest.raises(TokTokDataError) as info:
        client.rankings.regions()
    assert info.value.code == "invalid_response"
    assert info.value.status == 200
