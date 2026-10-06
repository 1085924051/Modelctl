"""Small dependency-free client for Modelctl business capabilities."""

from __future__ import annotations

import json
import os
import time
from urllib import request
from urllib.parse import quote


class ModelctlError(RuntimeError):
    pass


class Modelctl:
    def __init__(self, base_url: str = "http://127.0.0.1:11435", token: str | None = None):
        self.base_url = base_url.rstrip("/")
        self.token = os.environ.get("MODELCTL_API_TOKEN") if token is None else token

    def invoke(self, capability_id: str, input: dict, metadata: dict | None = None, version: str | None = None) -> dict:
        body = {"input": input}
        if metadata:
            body["metadata"] = metadata
        suffix = f"?version={quote(version, safe='')}" if version else ""
        return self._request("POST", f"/v1/capabilities/{quote(capability_id, safe='')}/invoke{suffix}", body)

    def capabilities(self) -> list[dict]:
        return self._request("GET", "/v1/capabilities").get("items", [])

    def schema(self, capability_id: str, version: str | None = None) -> dict:
        suffix = f"?version={quote(version, safe='')}" if version else ""
        return self._request("GET", f"/v1/capabilities/{quote(capability_id, safe='')}/schema{suffix}")

    def integration(self, capability_id: str, version: str | None = None) -> dict:
        """Return the copyable business-service handoff contract."""
        suffix = f"?version={quote(version, safe='')}" if version else ""
        return self._request("GET", f"/v1/capabilities/{quote(capability_id, safe='')}/integration{suffix}")

    def openapi(self, capability_id: str, version: str | None = None) -> dict:
        """Return an OpenAPI 3.1 document for a pinned capability."""
        suffix = f"?version={quote(version, safe='')}" if version else ""
        return self._request("GET", f"/v1/capabilities/{quote(capability_id, safe='')}/openapi{suffix}")

    def status(self, capability_id: str, version: str | None = None) -> dict:
        """Check model installation and runtime readiness without invoking."""
        suffix = f"?version={quote(version, safe='')}" if version else ""
        return self._request("GET", f"/v1/capabilities/{quote(capability_id, safe='')}/status{suffix}")

    def runs(self) -> list[dict]:
        return self._request("GET", "/v1/runs").get("items", [])

    def batch(self, capability_id: str, items: list[dict], version: str | None = None) -> dict:
        suffix = f"?version={quote(version, safe='')}" if version else ""
        return self._request("POST", f"/v1/capabilities/{quote(capability_id, safe='')}/batch{suffix}", {"items": items})

    def task(self, task_id: str) -> dict:
        return self._request("GET", f"/v1/tasks/{quote(task_id, safe='')}")

    def wait_task(self, task_id: str, poll_seconds: float = 0.5, timeout_seconds: float = 3600) -> dict:
        deadline = time.monotonic() + timeout_seconds
        while True:
            result = self.task(task_id)
            if result.get("status") in {"succeeded", "failed", "cancelled"}:
                return result
            if time.monotonic() >= deadline:
                raise ModelctlError(f"task {task_id} did not finish before timeout")
            time.sleep(poll_seconds)

    def _request(self, method: str, path: str, body: dict | None = None) -> dict:
        payload = None if body is None else json.dumps(body).encode("utf-8")
        headers = {"accept": "application/json"}
        if payload is not None:
            headers["content-type"] = "application/json"
        if self.token:
            headers["authorization"] = f"Bearer {self.token}"
        req = request.Request(self.base_url + path, data=payload, headers=headers, method=method)
        try:
            with request.urlopen(req, timeout=120) as response:
                return json.loads(response.read().decode("utf-8"))
        except Exception as exc:
            if hasattr(exc, "read"):
                try:
                    detail = json.loads(exc.read().decode("utf-8"))
                    message = detail.get("error", {}).get("message", str(exc))
                    raise ModelctlError(message) from exc
                except (ValueError, AttributeError):
                    pass
            raise ModelctlError(str(exc)) from exc
