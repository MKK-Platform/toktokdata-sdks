from __future__ import annotations

from typing import Any

from .._errors import PendingTimeoutError, parse_retry_after
from .._types import ExportDataset, ExportFormat, JSONDict
from ._base import Resource, require, seg

_DONE = frozenset({"ready", "failed"})


class Exports(Resource):
    """CSV / Excel exports (``/v1/exports``)."""

    def list(self) -> JSONDict:
        """``GET /v1/exports``."""
        return self._client._get("/v1/exports")

    def create(
        self,
        *,
        dataset: ExportDataset,
        format: ExportFormat,
        board: str | None = None,
        region: str | None = None,
        league_tier: str | None = None,
        range_days: int | None = None,
    ) -> JSONDict:
        """``POST /v1/exports``.

        Returns the job as is: ``status: "ready"`` (201, up to 1,000 rows) or ``"pending"``
        (202); use :meth:`wait_until_ready` for the latter.
        """
        require(dataset, "dataset")
        require(format, "format")
        body: dict[str, Any] = {"dataset": dataset, "format": format}
        optional = {
            "board": board,
            "region": region,
            "leagueTier": league_tier,
            "rangeDays": range_days,
        }
        body.update({k: v for k, v in optional.items() if v is not None})
        return self._client._post("/v1/exports", body)

    def get(self, job_id: str) -> JSONDict:
        """Status and download link: ``GET /v1/exports/{job_id}``."""
        return self._client._get(f"/v1/exports/{seg(job_id, 'job_id')}")

    def download(self, job_id: str) -> bytes:
        """The file of a ready export: ``GET /v1/exports/{job_id}/download``."""
        return self._client._get_bytes(f"/v1/exports/{seg(job_id, 'job_id')}/download")

    def wait_until_ready(
        self, job_id: str, *, timeout: float = 300.0, poll_interval: float = 2.0
    ) -> JSONDict:
        """Poll :meth:`get` until ``status`` is ``ready`` or ``failed`` and return the job.

        Waits ``poll_interval`` seconds between polls (longer when the API sends
        ``Retry-After``). Raises :class:`toktokdata.PendingTimeoutError` after ``timeout``
        seconds. A ``failed`` job is returned, not raised: check ``job["status"]``
        and ``job["error"]``.
        """
        path = f"/v1/exports/{seg(job_id, 'job_id')}"
        client = self._client
        started = client._clock()
        deadline = started + timeout
        while True:
            resp = client._send("GET", path)
            job: Any = client._parse(resp)
            status = job.get("status") if isinstance(job, dict) else None
            if status in _DONE:
                return job  # type: ignore[no-any-return]
            remaining = deadline - client._clock()
            if remaining <= 0:
                waited = client._clock() - started
                raise PendingTimeoutError(
                    f"export {job_id} was still {status or 'pending'} after {waited:.0f}s",
                    job_id=job_id,
                    last_body=job,
                    waited=waited,
                )
            delay = poll_interval
            retry_after = parse_retry_after(resp.header("retry-after"))
            if retry_after is not None:
                delay = max(delay, min(retry_after, 60.0))
            client._sleep(min(delay, remaining))
