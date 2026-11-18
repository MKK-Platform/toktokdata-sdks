from __future__ import annotations

from .._types import AgencySort, JSONDict
from ._base import Resource, seg


class Agencies(Resource):
    """LIVE agencies (``/v1/agencies``)."""

    def list(
        self,
        *,
        q: str | None = None,
        region: str | None = None,
        public_only: bool | None = None,
        sort: AgencySort | None = None,
        limit: int | None = None,
        offset: int | None = None,
    ) -> JSONDict:
        """``GET /v1/agencies``."""
        return self._client._get(
            "/v1/agencies",
            {
                "q": q,
                "region": region,
                "publicOnly": public_only,
                "sort": sort,
                "limit": limit,
                "offset": offset,
            },
        )

    def get(self, agency_id: str) -> JSONDict:
        """``GET /v1/agencies/{agency_id}``."""
        return self._client._get(f"/v1/agencies/{seg(agency_id, 'agency_id')}")
