from __future__ import annotations

from typing import TYPE_CHECKING

from .._util import path_segment as seg
from .._util import require

if TYPE_CHECKING:
    from .._client import TokTokData

__all__ = ["Resource", "require", "seg"]


class Resource:
    def __init__(self, client: TokTokData) -> None:
        self._client = client
