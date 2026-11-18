from __future__ import annotations

import datetime as _dt
import urllib.parse
from collections.abc import Mapping
from typing import Any


def encode_query(params: Mapping[str, Any] | None) -> str:
    """Encode query params: ``None`` and empty values are dropped, booleans are
    ``true``/``false``, lists and tuples are comma-joined, datetimes are ISO 8601."""
    if not params:
        return ""
    pairs: list[tuple[str, str]] = []
    for key, value in params.items():
        if value is None:
            continue
        if isinstance(value, bool):
            text = "true" if value else "false"
        elif isinstance(value, (list, tuple)):
            if not value:
                continue
            text = ",".join(str(item) for item in value)
        elif isinstance(value, (_dt.datetime, _dt.date)):
            text = value.isoformat()
        else:
            text = str(value)
        if text == "":
            continue
        pairs.append((key, text))
    return urllib.parse.urlencode(pairs, quote_via=urllib.parse.quote, safe=",")


def path_segment(value: Any, name: str, *, strip_at: bool = False) -> str:
    """URL-encode one path parameter; reject empty values (they would hit another route)."""
    text = str(value).strip() if value is not None else ""
    if strip_at:
        text = text.lstrip("@")
    if not text:
        raise ValueError(f"{name} must be a non-empty string")
    return urllib.parse.quote(text, safe="")


def require(value: object, name: str) -> None:
    if value is None or (isinstance(value, str) and not value.strip()):
        raise ValueError(f"{name} is required")
