import test from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import { handle } from "../src/mcp.js";

test("MCP exposes published capabilities with their request schema", async () => {
  const seen = [];
  const server = http.createServer((req, res) => {
    seen.push({ url: req.url, authorization: req.headers.authorization });
    res.setHeader("content-type", "application/json");
    if (req.url === "/v1/capabilities") return res.end(JSON.stringify({ items: [{ id: "refund-check", version: "1.0.0", name: "Refund check", description: "Check refund intent" }] }));
    if (req.url === "/v1/capabilities/refund-check/schema?version=1.0.0") return res.end(JSON.stringify({ request_schema: { type: "object", required: ["input"], properties: { input: { type: "object", required: ["text"] }, metadata: { type: "object" } } } }));
    res.statusCode = 404;
    return res.end(JSON.stringify({ error: { message: "not found" } }));
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const oldURL = process.env.MODELCTL_URL;
  const oldToken = process.env.MODELCTL_API_TOKEN;
  process.env.MODELCTL_URL = `http://127.0.0.1:${server.address().port}`;
  process.env.MODELCTL_API_TOKEN = "mcp-secret";
  try {
    const response = await handle({ jsonrpc: "2.0", id: 1, method: "tools/list" });
    const tool = response.result.tools.find((item) => item.name === "modelctl_capability_refund-check");
    assert.ok(tool);
    assert.deepEqual(tool.inputSchema.required, ["input"]);
    assert.equal(tool.inputSchema.properties.input.required[0], "text");
    assert.ok(seen.every((request) => request.authorization === "Bearer mcp-secret"));
  } finally {
    if (oldURL === undefined) delete process.env.MODELCTL_URL; else process.env.MODELCTL_URL = oldURL;
    if (oldToken === undefined) delete process.env.MODELCTL_API_TOKEN; else process.env.MODELCTL_API_TOKEN = oldToken;
    await new Promise((resolve) => server.close(resolve));
  }
});

test("MCP capability tool invokes the stable capability endpoint", async () => {
  let invocation;
  const server = http.createServer(async (req, res) => {
    res.setHeader("content-type", "application/json");
    if (req.url === "/v1/capabilities") return res.end(JSON.stringify({ items: [{ id: "refund-check", version: "1.0.0" }] }));
    if (req.url === "/v1/capabilities/refund-check/invoke?version=1.0.0") {
      const chunks = [];
      for await (const chunk of req) chunks.push(chunk);
      invocation = { authorization: req.headers.authorization, body: JSON.parse(Buffer.concat(chunks).toString("utf8")) };
      return res.end(JSON.stringify({ run_id: "run_mcp", output: { refund: { type: "noul", value: 0.9, noul: 0.9 } } }));
    }
    res.statusCode = 404;
    return res.end(JSON.stringify({ error: { message: "not found" } }));
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const oldURL = process.env.MODELCTL_URL;
  const oldToken = process.env.MODELCTL_API_TOKEN;
  process.env.MODELCTL_URL = `http://127.0.0.1:${server.address().port}`;
  process.env.MODELCTL_API_TOKEN = "mcp-secret";
  try {
    const response = await handle({ jsonrpc: "2.0", id: 2, method: "tools/call", params: { name: "modelctl_capability_refund-check", arguments: { input: { text: "refund" }, metadata: { ticket_id: "T-1" }, version: "1.0.0" } } });
    assert.equal(response.result.structuredContent.run_id, "run_mcp");
    assert.equal(invocation.authorization, "Bearer mcp-secret");
    assert.deepEqual(invocation.body, { input: { text: "refund" }, metadata: { ticket_id: "T-1" } });
  } finally {
    if (oldURL === undefined) delete process.env.MODELCTL_URL; else process.env.MODELCTL_URL = oldURL;
    if (oldToken === undefined) delete process.env.MODELCTL_API_TOKEN; else process.env.MODELCTL_API_TOKEN = oldToken;
    await new Promise((resolve) => server.close(resolve));
  }
});
