from __future__ import annotations

from collections.abc import Iterator

from .._types import GifterWindow, JSONDict
from ._base import Resource, seg


def _uid(uid: str) -> str:
    return seg(uid, "uid", strip_at=True)


class Creators(Resource):
    """Creator profiles, followers and analytics (``/v1/creators``).

    ``uid`` is a handle (``linh.live``; a leading ``@`` is stripped) or a numeric TikTok
    user id.
    """

    def search(self, *, q: str | None = None, limit: int | None = None) -> JSONDict:
        """Search stored creators: ``GET /v1/creators``."""
        return self._client._get("/v1/creators", {"q": q, "limit": limit})

    def get(self, uid: str, *, wait: bool = True) -> JSONDict:
        """Profile and recent counters: ``GET /v1/creators/{uid}``.

        202 aware: by default waits while the profile is looked up on TikTok; with
        ``wait=False`` the ``{"status", "jobId", "retryAfterSeconds"}`` body is returned.
        """
        return self._client._get_pending(f"/v1/creators/{_uid(uid)}", wait=wait)

    def analysis(self, uid: str) -> JSONDict:
        """``GET /v1/creators/{uid}/analysis``."""
        return self._client._get(f"/v1/creators/{_uid(uid)}/analysis")

    def changes(self, uid: str, *, limit: int | None = None) -> JSONDict:
        """Profile change log: ``GET /v1/creators/{uid}/changes``."""
        return self._client._get(f"/v1/creators/{_uid(uid)}/changes", {"limit": limit})

    def followers(
        self,
        uid: str,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        wait: bool = True,
    ) -> JSONDict:
        """Stored followers: ``GET /v1/creators/{uid}/followers`` (202 aware, paged)."""
        return self._client._get_pending(
            f"/v1/creators/{_uid(uid)}/followers", {"cursor": cursor, "limit": limit}, wait=wait
        )

    def iter_followers(
        self, uid: str, *, limit: int | None = None, cursor: str | None = None
    ) -> Iterator[JSONDict]:
        """Yield every page of :meth:`followers` (waiting on 202s)."""
        _uid(uid)
        return self._client._paginate(lambda c: self.followers(uid, cursor=c, limit=limit), cursor)

    def following(
        self,
        uid: str,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        wait: bool = True,
    ) -> JSONDict:
        """Accounts the creator follows: ``GET /v1/creators/{uid}/following`` (202 aware,
        paged)."""
        return self._client._get_pending(
            f"/v1/creators/{_uid(uid)}/following", {"cursor": cursor, "limit": limit}, wait=wait
        )

    def iter_following(
        self, uid: str, *, limit: int | None = None, cursor: str | None = None
    ) -> Iterator[JSONDict]:
        """Yield every page of :meth:`following` (waiting on 202s)."""
        _uid(uid)
        return self._client._paginate(lambda c: self.following(uid, cursor=c, limit=limit), cursor)

    def gifters(
        self, uid: str, *, window: GifterWindow | None = None, limit: int | None = None
    ) -> JSONDict:
        """Top gifters of the creator: ``GET /v1/creators/{uid}/gifters``."""
        return self._client._get(
            f"/v1/creators/{_uid(uid)}/gifters", {"window": window, "limit": limit}
        )

    def insights(self, uid: str) -> JSONDict:
        """``GET /v1/creators/{uid}/insights``."""
        return self._client._get(f"/v1/creators/{_uid(uid)}/insights")

    def videos(self, uid: str) -> JSONDict:
        """``GET /v1/creators/{uid}/videos``."""
        return self._client._get(f"/v1/creators/{_uid(uid)}/videos")
