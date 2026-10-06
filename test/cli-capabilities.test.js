import test from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import fsp from "node:fs/promises";
import { main } from "../src/cli.js";

test("capabilities CLI forwards the enterprise bearer token", async () => {
  let seenAuthorization = "";
  const server = http.createServer((req, res) => {
    seenAuthorization = req.headers.authorization || "";
    res.setHeader("content-type", "application/json");
    if (req.url === "/health") return res.end(JSON.stringify({ status: "ok" }));
    if (req.url === "/v1/capabilities") return res.end(JSON.stringify({ items: [{ id: "refund-check" }] }));
    res.statusCode = 404;
    return res.end(JSON.stringify({ error: { message: "not found" } }));
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  const oldURL = process.env.MODELCTL_URL;
  const oldToken = process.env.MODELCTL_API_TOKEN;
  process.env.MODELCTL_URL = `http://127.0.0.1:${address.port}`;
  process.env.MODELCTL_API_TOKEN = "cli-secret";
  try {
    await main(["capabilities", "list"]);
  } finally {
    if (oldURL === undefined) delete process.env.MODELCTL_URL; else process.env.MODELCTL_URL = oldURL;
    if (oldToken === undefined) delete process.env.MODELCTL_API_TOKEN; else process.env.MODELCTL_API_TOKEN = oldToken;
    await new Promise((resolve) => server.close(resolve));
  }
  assert.equal(seenAuthorization, "Bearer cli-secret");
});

test("capabilities integration CLI fetches the copyable handoff contract", async () => {
  let seenPath = "";
  const server = http.createServer((req, res) => {
    seenPath = req.url;
    res.setHeader("content-type", "application/json");
    if (req.url === "/health") return res.end(JSON.stringify({ status: "ok" }));
    if (req.url === "/v1/capabilities/refund-check/integration?version=1.0.0") return res.end(JSON.stringify({ endpoint: "ok", examples: { curl: "curl" } }));
    res.statusCode = 404;
    return res.end(JSON.stringify({ error: { message: "not found" } }));
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  const oldURL = process.env.MODELCTL_URL;
  try {
    process.env.MODELCTL_URL = `http://127.0.0.1:${address.port}`;
    await main(["capabilities", "integration", "refund-check", "--version", "1.0.0"]);
  } finally {
    if (oldURL === undefined) delete process.env.MODELCTL_URL; else process.env.MODELCTL_URL = oldURL;
    await new Promise((resolve) => server.close(resolve));
  }
  assert.equal(seenPath, "/v1/capabilities/refund-check/integration?version=1.0.0");
});

test("capabilities batch --wait polls until the task is terminal", async () => {
  let polls = 0;
  let seenBatch = null;
  const server = http.createServer(async (req, res) => {
    res.setHeader("content-type", "application/json");
    if (req.url === "/health") return res.end(JSON.stringify({ status: "ok" }));
    if (req.method === "POST" && req.url === "/v1/capabilities/ticket-routing/batch") {
      const chunks = [];
      for await (const chunk of req) chunks.push(chunk);
      seenBatch = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      return res.end(JSON.stringify({ task_id: "task_cli", status: "queued", poll: "/v1/tasks/task_cli" }));
    }
    if (req.method === "GET" && req.url === "/v1/tasks/task_cli") {
      polls += 1;
      const status = polls < 2 ? "running" : "succeeded";
      return res.end(JSON.stringify({ id: "task_cli", status, progress: { items_done: status === "succeeded" ? 1 : 0, items_total: 1 }, results: [{ index: 0, run_id: "run_cli" }], errors: [] }));
    }
    res.statusCode = 404;
    return res.end(JSON.stringify({ error: { message: "not found" } }));
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  const temp = await fsp.mkdtemp(path.join(os.tmpdir(), "modelctl-cli-"));
  const inputPath = path.join(temp, "batch.json");
  await fsp.writeFile(inputPath, JSON.stringify({ items: [{ input: { text: "hello" }, metadata: { ticket_id: "T-1" } }] }));
  const oldURL = process.env.MODELCTL_URL;
  try {
    process.env.MODELCTL_URL = `http://127.0.0.1:${address.port}`;
    await main(["capabilities", "batch", "ticket-routing", "--json", inputPath, "--wait"]);
  } finally {
    if (oldURL === undefined) delete process.env.MODELCTL_URL; else process.env.MODELCTL_URL = oldURL;
    await new Promise((resolve) => server.close(resolve));
    await fsp.rm(temp, { recursive: true, force: true });
  }
  assert.deepEqual(seenBatch, { items: [{ input: { text: "hello" }, metadata: { ticket_id: "T-1" } }] });
  assert.ok(polls >= 2);
});
