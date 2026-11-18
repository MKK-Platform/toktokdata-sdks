from __future__ import annotations

from collections.abc import Iterator

from .._types import JSONDict
from ._base import Resource, seg


class Videos(Resource):
    """TikTok videos (``/v1/videos``)."""

    def get(self, video_id: str) -> JSONDict:
        """``GET /v1/videos/{video_id}``."""
        return self._client._get(f"/v1/videos/{seg(video_id, 'video_id')}")

    def comments(
        self,
        video_id: str,
        *,
        parent_comment_id: str | None = None,
        cursor: str | None = None,
        limit: int | None = None,
        wait: bool = True,
    ) -> JSONDict:
        """Stored comments (or the replies of ``parent_comment_id``):
        ``GET /v1/videos/{video_id}/comments`` (202 aware, paged)."""
        return self._client._get_pending(
            f"/v1/videos/{seg(video_id, 'video_id')}/comments",
            {"parentCommentId": parent_comment_id, "cursor": cursor, "limit": limit},
            wait=wait,
        )

    def iter_comments(
        self,
        video_id: str,
        *,
        parent_comment_id: str | None = None,
        limit: int | None = None,
        cursor: str | None = None,
    ) -> Iterator[JSONDict]:
        """Yield every page of :meth:`comments` (waiting on 202s)."""
        seg(video_id, "video_id")
        return self._client._paginate(
            lambda c: self.comments(
                video_id, parent_comment_id=parent_comment_id, cursor=c, limit=limit
            ),
            cursor,
        )
