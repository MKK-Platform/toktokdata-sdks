from __future__ import annotations

from .._types import JSONDict
from ._base import Resource


class Account(Resource):
    """Plan, entitlements and API usage of the key's workspace."""

    def entitlements(self, *, period: str | None = None) -> JSONDict:
        """``GET /v1/entitlements``."""
        return self._client._get("/v1/entitlements", {"period": period})

    def usage(self, *, period: str | None = None, limit: int | None = None) -> JSONDict:
        """``GET /v1/usage``."""
        return self._client._get("/v1/usage", {"period": period, "limit": limit})
