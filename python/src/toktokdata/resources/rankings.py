from __future__ import annotations

from collections.abc import Iterator

from .._types import JSONDict, RankingPeriod
from ._base import Resource, require, seg


class Rankings(Resource):
    """Official TikTok LIVE boards (``/v1/rankings``)."""

    def official(
        self,
        board: str,
        *,
        region: str | None = None,
        period: RankingPeriod | None = None,
        limit: int | None = None,
        live_only: bool | None = None,
        league_tier: str | None = None,
    ) -> JSONDict:
        """One board for one region: ``GET /v1/rankings/official``.

        ``board``: ``daily``, ``hourly``, ``popular_live``, ``gaming_daily``, ``gaming_weekly``,
        ``shopping_daily``, ``shopping_weekly`` or a league board ``league_a1`` ... ``league_d5``.
        """
        require(board, "board")
        return self._client._get(
            "/v1/rankings/official",
            {
                "board": board,
                "region": region,
                "period": period,
                "limit": limit,
                "liveOnly": live_only,
                "leagueTier": league_tier,
            },
        )

    def regions(self) -> JSONDict:
        """Regions with boards: ``GET /v1/rankings/regions``."""
        return self._client._get("/v1/rankings/regions")

    def history(
        self,
        board: str,
        *,
        region: str | None = None,
        cursor: str | None = None,
        limit: int | None = None,
        league_tier: str | None = None,
    ) -> JSONDict:
        """Completed periods of a board, newest first: ``GET /v1/rankings/history`` (paged)."""
        require(board, "board")
        return self._client._get(
            "/v1/rankings/history",
            {
                "board": board,
                "region": region,
                "cursor": cursor,
                "limit": limit,
                "leagueTier": league_tier,
            },
        )

    def iter_history(
        self,
        board: str,
        *,
        region: str | None = None,
        limit: int | None = None,
        league_tier: str | None = None,
        cursor: str | None = None,
    ) -> Iterator[JSONDict]:
        """Yield every page of :meth:`history`, following ``data.nextCursor``."""
        require(board, "board")
        return self._client._paginate(
            lambda c: self.history(
                board, region=region, cursor=c, limit=limit, league_tier=league_tier
            ),
            cursor,
        )

    def day_bars(
        self,
        *,
        region: str | None = None,
        board: str | None = None,
        days: int | None = None,
    ) -> JSONDict:
        """Daily totals of a board: ``GET /v1/rankings/day-bars``."""
        return self._client._get(
            "/v1/rankings/day-bars", {"region": region, "board": board, "days": days}
        )

    def masked(self, ref: str) -> JSONDict:
        """LIVE totals of a masked entry (``maskedRef``): ``GET /v1/rankings/masked/{ref}``."""
        return self._client._get(f"/v1/rankings/masked/{seg(ref, 'ref')}")
