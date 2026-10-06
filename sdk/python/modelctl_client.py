"""Small dependency-free client for Modelctl business capabilities."""

from __future__ import annotations

import json
from urllib import request


class ModelctlError(RuntimeError):
    pass


class Modelctl:
    def __init__(self, base_url: str = "http://127.0.0.1:11435", token: str | None = None):
        self.base_url = base_url.rstrip("/")
        self.token = token

    def invoke(self, capability_id: str, input: dict, metadata: dict | None = None) -> dict:
        body = {"input": input}
        if metadata:
            body["metadata"] = metadata
        return self._request("POST", f"/v1/capabilities/{capability_id}/invoke", body)

    def capabilities(self) -> list[dict]:
        return self._request("GET", "/v1/capabilities").get("items", [])

    def runs(self) -> list[dict]:
        return self._request("GET", "/v1/runs").get("items", [])

    def batch(self, capability_id: str, items: list[dict]) -> dict:
        return self._request("POST", f"/v1/capabilities/{capability_id}/batch", {"items": items})

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
