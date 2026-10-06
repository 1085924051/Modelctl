import test, { after, beforeEach } from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import fsp from "node:fs/promises";
import { createServer } from "../src/server.js";

const oldDataDir = process.env.MODELCTL_DATA_DIR;
let root;
let server;
let base;

beforeEach(async () => {
  if (server) await new Promise((resolve) => server.close(resolve));
  if (root) await fsp.rm(root, { recursive: true, force: true });
  root = await fsp.mkdtemp(path.join(os.tmpdir(), "modelctl-capability-"));
  process.env.MODELCTL_DATA_DIR = root;
  server = await createServer({ port: 0 });
  const address = server.address();
  base = `http://127.0.0.1:${address.port}`;
});

after(async () => {
  if (server) await new Promise((resolve) => server.close(resolve));
  if (root) await fsp.rm(root, { recursive: true, force: true });
  if (oldDataDir === undefined) delete process.env.MODELCTL_DATA_DIR;
  else process.env.MODELCTL_DATA_DIR = oldDataDir;
});

function request(route, method = "GET", body) {
  return new Promise((resolve, reject) => {
    const payload = body === undefined ? null : Buffer.from(JSON.stringify(body));
    const req = http.request(new URL(route, base), { method, headers: payload ? { "content-type": "application/json", "content-length": payload.length } : {} }, (res) => {
      const chunks = [];
      res.on("data", (chunk) => chunks.push(chunk));
      res.on("end", () => resolve({ status: res.statusCode, body: JSON.parse(Buffer.concat(chunks).toString("utf8")) }));
    });
    req.on("error", reject);
    if (payload) req.write(payload);
    req.end();
  });
}

const capability = {
  id: "refund-check",
  name: "Refund check",
  description: "Decide whether the customer asks for a refund.",
  model: { model_id: "convaiinnovations/laya", variant: "english", profile: "auto" },
  input: { type: "text", field: "text" },
  questions: { refund: { type: "noul", instructions: "Does the customer ask for a refund?" } },
};

test("capabilities can be published and listed", async () => {
  const created = await request("/v1/capabilities", "POST", capability);
  assert.equal(created.status, 201);
  assert.equal(created.body.id, "refund-check");
  assert.equal(created.body.version, "1.0.0");
  const listed = await request("/v1/capabilities");
  assert.equal(listed.status, 200);
  assert.deepEqual(listed.body.items.map((item) => item.id), ["refund-check"]);
  const schema = await request("/v1/capabilities/refund-check/schema");
  assert.equal(schema.status, 200);
  assert.deepEqual(schema.body.request_schema.properties.input.required, ["text"]);
});

test("run history is available as a replayable collection", async () => {
  const result = await request("/v1/runs");
  assert.equal(result.status, 200);
  assert.deepEqual(result.body.items, []);
});

test("capability versions can be published and activated explicitly", async () => {
  const first = await request("/v1/capabilities", "POST", capability);
  assert.equal(first.body.active, true);
  const second = await request("/v1/capabilities", "POST", { ...capability, version: "1.1.0", name: "Refund check v2", activate: false });
  assert.equal(second.status, 201);
  assert.equal(second.body.active, false);
  const current = await request("/v1/capabilities/refund-check");
  assert.equal(current.body.version, "1.0.0");
  const activated = await request("/v1/capabilities/refund-check/activate", "POST", { version: "1.1.0" });
  assert.equal(activated.status, 200);
  assert.equal(activated.body.version, "1.1.0");
  const selected = await request("/v1/capabilities/refund-check?version=1.0.0");
  assert.equal(selected.body.version, "1.0.0");
});

test("capability validation hides the raw protocol from invalid clients", async () => {
  const invalid = await request("/v1/capabilities", "POST", { id: "Bad ID", name: "x" });
  assert.equal(invalid.status, 422);
  assert.equal(invalid.body.error.code, "CAPABILITY_INVALID");
});

test("capability versions use semantic versioning", async () => {
  const invalid = await request("/v1/capabilities", "POST", { ...capability, version: "draft" });
  assert.equal(invalid.status, 422);
  assert.equal(invalid.body.error.code, "CAPABILITY_VERSION_INVALID");
});

test("capability invocation explains that its model must be downloaded", async () => {
  await request("/v1/capabilities", "POST", capability);
  const result = await request("/v1/capabilities/refund-check/invoke", "POST", { input: { text: "Please refund this order" } });
  assert.equal(result.status, 409);
  assert.equal(result.body.error.code, "MODEL_NOT_INSTALLED");
});

test("capability batches return a task that records per-item errors", async () => {
  await request("/v1/capabilities", "POST", capability);
  const created = await request("/v1/capabilities/refund-check/batch", "POST", { items: [{ input: { text: "one" } }, { input: { text: "two" } }] });
  assert.equal(created.status, 202);
  let task;
  for (let attempt = 0; attempt < 30; attempt += 1) {
    task = await request(created.body.poll);
    if (["succeeded", "failed", "cancelled"].includes(task.body.status)) break;
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  assert.equal(task.body.status, "succeeded");
  assert.equal(task.body.progress.items_done, 2);
  assert.equal(task.body.errors.length, 2);
});

test("remote binding requires an explicit API token", async () => {
  const previous = process.env.MODELCTL_API_TOKEN;
  delete process.env.MODELCTL_API_TOKEN;
  await assert.rejects(() => createServer({ host: "0.0.0.0", port: 0 }), /MODELCTL_API_TOKEN/);
  if (previous === undefined) delete process.env.MODELCTL_API_TOKEN;
  else process.env.MODELCTL_API_TOKEN = previous;
});
