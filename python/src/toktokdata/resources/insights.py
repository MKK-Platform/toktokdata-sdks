from __future__ import annotations

from .._types import JSONDict
from ._base import Resource


class Insights(Resource):
    """Regional LIVE insights (``/v1/insights``)."""

    def best_time(self, *, region: str | None = None) -> JSONDict:
        """Best hours to go LIVE: ``GET /v1/insights/best-time``."""
        return self._client._get("/v1/insights/best-time", {"region": region})

    def presence(self, *, region: str | None = None, hours: int | None = None) -> JSONDict:
        """Creators LIVE over time: ``GET /v1/insights/presence``."""
        return self._client._get("/v1/insights/presence", {"region": region, "hours": hours})
