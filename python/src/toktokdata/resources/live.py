from __future__ import annotations

from collections.abc import Iterator, Sequence

from .._types import JSONDict
from ._base import Resource, seg


def _uid(uid: str) -> str:
    return seg(uid, "uid", strip_at=True)


class Live(Resource):
    """LIVE sessions, performance, playback and chat (``/v1/live``)."""

    def sessions(
        self,
        *,
        creator_unique_id: str | None = None,
        cursor: str | None = None,
        limit: int | None = None,
    ) -> JSONDict:
        """Recorded LIVE sessions of followed rooms: ``GET /v1/live/sessions`` (paged)."""
        return self._client._get(
            "/v1/live/sessions",
            {"creatorUniqueId": creator_unique_id, "cursor": cursor, "limit": limit},
        )

    def iter_sessions(
        self,
        *,
        creator_unique_id: str | None = None,
        limit: int | None = None,
        cursor: str | None = None,
    ) -> Iterator[JSONDict]:
        """Yield every page of :meth:`sessions`, following ``data.nextCursor``."""
        return self._client._paginate(
            lambda c: self.sessions(creator_unique_id=creator_unique_id, cursor=c, limit=limit),
            cursor,
        )

    def session(self, session_id: str) -> JSONDict:
        """``GET /v1/live/sessions/{session_id}``."""
        return self._client._get(f"/v1/live/sessions/{seg(session_id, 'session_id')}")

    def timeline(self, session_id: str) -> JSONDict:
        """Minute-by-minute series of a session: ``GET /v1/live/sessions/{session_id}/timeline``."""
        return self._client._get(f"/v1/live/sessions/{seg(session_id, 'session_id')}/timeline")

    def compare(self, ids: Sequence[str] | str) -> JSONDict:
        """Two to four sessions side by side: ``GET /v1/live/sessions/compare``.

        ``ids`` is a list of session ids (sent comma-joined) or an already comma-joined string.
        """
        if isinstance(ids, str):
            joined = ids.strip()
        else:
            items = [str(i).strip() for i in ids]
            if not 2 <= len(items) <= 4 or not all(items):
                raise ValueError("compare() needs 2 to 4 non-empty session ids")
            joined = ",".join(items)
        if not joined:
            raise ValueError("ids is required")
        return self._client._get("/v1/live/sessions/compare", {"ids": joined})

    def performance(
        self, uid: str, *, cursor: str | None = None, limit: int | None = None
    ) -> JSONDict:
        """Per-session performance of a creator: ``GET /v1/live/creators/{uid}/performance``
        (paged)."""
        return self._client._get(
            f"/v1/live/creators/{_uid(uid)}/performance", {"cursor": cursor, "limit": limit}
        )

    def iter_performance(
        self, uid: str, *, limit: int | None = None, cursor: str | None = None
    ) -> Iterator[JSONDict]:
        """Yield every page of :meth:`performance`, following ``data.nextCursor``."""
        _uid(uid)
        return self._client._paginate(
            lambda c: self.performance(uid, cursor=c, limit=limit), cursor
        )

    def refresh(self, uid: str) -> JSONDict:
        """Queue a fresh check of the creator's room: ``POST /v1/live/creators/{uid}/refresh``.

        Returns the ``202`` body (``{"queued": true, "jobId": ...}``) as is.
        """
        return self._client._post(f"/v1/live/creators/{_uid(uid)}/refresh")

    def playback(self, uid: str, *, wait: bool = True) -> JSONDict:
        """Signed video URL of the creator's current LIVE room:
        ``GET /v1/live/creators/{uid}/playback`` (202 aware; needs the ``live:stream`` scope)."""
        return self._client._get_pending(f"/v1/live/creators/{_uid(uid)}/playback", wait=wait)

    def comments(self, uid: str, *, after: str | None = None) -> JSONDict:
        """Latest chat of the creator's LIVE room: ``GET /v1/live/creators/{uid}/comments``.

        Pass the previous answer's ``cursor`` as ``after`` to get only newer comments.
        """
        return self._client._get(f"/v1/live/creators/{_uid(uid)}/comments", {"after": after})
