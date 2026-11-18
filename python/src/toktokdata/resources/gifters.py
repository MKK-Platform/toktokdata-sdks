from __future__ import annotations

from .._types import GifterSegment, GifterWindow, JSONDict
from ._base import Resource


class Gifters(Resource):
    """Top gifters across recorded rooms (``/v1/gifters``)."""

    def list(
        self,
        *,
        window: GifterWindow | None = None,
        segment: GifterSegment | None = None,
        region: str | None = None,
        limit: int | None = None,
    ) -> JSONDict:
        """``GET /v1/gifters``."""
        return self._client._get(
            "/v1/gifters",
            {"window": window, "segment": segment, "region": region, "limit": limit},
        )
