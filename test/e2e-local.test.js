import test from "node:test";
import assert from "node:assert/strict";
import http from "node:http";

const base = process.env.MODELCTL_TEST_URL;

function request(route, method = "GET", body) {
  return new Promise((resolve, reject) => {
    const url = new URL(route, base);
    const payload = body === undefined ? null : Buffer.from(JSON.stringify(body));
    const req = http.request(url, { method, headers: payload ? { "content-type": "application/json", "content-length": payload.length } : {} }, (res) => {
      const chunks = [];
      res.on("data", (chunk) => chunks.push(chunk));
      res.on("end", () => resolve({ status: res.statusCode, body: JSON.parse(Buffer.concat(chunks).toString("utf8")) }));
    });
    req.on("error", reject);
    if (payload) req.write(payload);
    req.end();
  });
}

test("local workbench endpoints and ready instance are usable", { skip: !base && "set MODELCTL_TEST_URL to a running local daemon" }, async (context) => {
  for (const route of ["/health", "/v1/settings", "/v1/community", "/v1/mcp/config", "/v1/models", "/v1/instances"]) {
    const response = await request(route);
    assert.equal(response.status, 200, `${route} should return HTTP 200`);
    assert.ok(response.body && typeof response.body === "object", `${route} should return JSON`);
  }

  const { body: instances } = await request("/v1/instances");
  const ready = instances.items.find((instance) => instance.status === "ready" && instance.capabilities?.includes("system_one"));
  if (!ready) return context.skip("no ready system_one instance is running");

  const result = await request(`/v1/instances/${encodeURIComponent(ready.id)}/operations/system_one`, "POST", {
    state: { body: "We were charged twice. Please refund us." },
    questions: { refund: { type: "noul", instructions: "Is the customer asking for a refund?" } },
  });
  assert.equal(result.status, 200);
  assert.ok(result.body.answers?.refund);
});
