from __future__ import annotations

import datetime as _dt
from collections.abc import Mapping
from typing import Any, Union

from .._types import DeliveryStatus, JSONDict
from ._base import Resource, require, seg

Since = Union[str, _dt.datetime]


class Alerts(Resource):
    """Alert rules, their fires and webhook deliveries (``/v1/alerts``).

    Rule bodies use the API's camelCase field names; see :class:`toktokdata.AlertRuleInput`.
    """

    def list_rules(self) -> JSONDict:
        """``GET /v1/alerts/rules``."""
        return self._client._get("/v1/alerts/rules")

    def create_rule(self, body: Mapping[str, Any]) -> JSONDict:
        """``POST /v1/alerts/rules``. ``body["name"]`` is required."""
        require(body.get("name"), "body['name']")
        return self._client._post("/v1/alerts/rules", dict(body))

    def update_rule(self, rule_id: str, body: Mapping[str, Any]) -> JSONDict:
        """``PATCH /v1/alerts/rules/{rule_id}`` with the fields to change."""
        return self._client._patch(f"/v1/alerts/rules/{seg(rule_id, 'rule_id')}", dict(body))

    def delete_rule(self, rule_id: str) -> None:
        """``DELETE /v1/alerts/rules/{rule_id}`` (204)."""
        self._client._delete(f"/v1/alerts/rules/{seg(rule_id, 'rule_id')}")

    def test_rule(self, rule_id: str) -> JSONDict:
        """Send a test notification to every channel: ``POST /v1/alerts/rules/{rule_id}/test``."""
        return self._client._post(f"/v1/alerts/rules/{seg(rule_id, 'rule_id')}/test")

    def fires(
        self,
        *,
        rule_id: str | None = None,
        since: Since | None = None,
        limit: int | None = None,
    ) -> JSONDict:
        """``GET /v1/alerts/fires``. ``since`` is an ISO 8601 string or a datetime."""
        return self._client._get(
            "/v1/alerts/fires", {"ruleId": rule_id, "since": since, "limit": limit}
        )

    def deliveries(
        self,
        *,
        rule_id: str | None = None,
        status: DeliveryStatus | None = None,
        since: Since | None = None,
        limit: int | None = None,
    ) -> JSONDict:
        """``GET /v1/alerts/deliveries``."""
        return self._client._get(
            "/v1/alerts/deliveries",
            {"ruleId": rule_id, "status": status, "since": since, "limit": limit},
        )

    def stats(self) -> JSONDict:
        """``GET /v1/alerts/stats``."""
        return self._client._get("/v1/alerts/stats")
