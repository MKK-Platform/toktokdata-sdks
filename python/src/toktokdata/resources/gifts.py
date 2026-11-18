from __future__ import annotations

from .._types import GiftSort, JSONDict
from ._base import Resource


class Gifts(Resource):
    """The LIVE gift catalog (``/v1/gifts``)."""

    def list(
        self,
        *,
        q: str | None = None,
        min_coins: int | None = None,
        max_coins: int | None = None,
        region: str | None = None,
        sort: GiftSort | None = None,
        limit: int | None = None,
    ) -> JSONDict:
        """``GET /v1/gifts``."""
        return self._client._get(
            "/v1/gifts",
            {
                "q": q,
                "minCoins": min_coins,
                "maxCoins": max_coins,
                "region": region,
                "sort": sort,
                "limit": limit,
            },
        )
