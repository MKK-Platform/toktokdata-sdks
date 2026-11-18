from __future__ import annotations

from .._types import JSONDict, TrendType
from ._base import Resource


class Trends(Resource):
    """Trending hashtags and sounds (``/v1/trends``)."""

    def list(self, *, region: str | None = None, type: TrendType | None = None) -> JSONDict:
        """``GET /v1/trends``."""
        return self._client._get("/v1/trends", {"region": region, "type": type})
