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
  root = await fsp.mkdtemp(path.join(os.tmpdir(), "modelctl-api-"));
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

test("settings endpoint returns local defaults and effective proxy without credentials", async () => {
  process.env.HTTP_PROXY = "http://user:secret@proxy.local:7897";
  const { status, body } = await request("/v1/settings");
  assert.equal(status, 200);
  assert.equal(body.settings.default_profile, "auto");
  assert.equal(body.effective_proxy.http, "http://***:***@proxy.local:7897");
  assert.equal(JSON.stringify(body).includes("secret"), false);
  assert.equal(body.runtime.daemon_url, base);
});

test("settings endpoint saves proxy and supported default profile", async () => {
  const saved = await request("/v1/settings", "PUT", { proxy: { http: "http://127.0.0.1:7897", https: "http://127.0.0.1:7897" }, default_profile: "mps" });
  assert.equal(saved.status, 200);
  assert.equal(saved.body.settings.proxy.http, "http://127.0.0.1:7897");
  const reread = await request("/v1/settings");
  assert.equal(reread.body.settings.default_profile, "mps");
  assert.equal(reread.body.effective_proxy.https, "http://127.0.0.1:7897");
});

test("settings endpoint rejects malformed proxy URLs", async () => {
  const result = await request("/v1/settings", "PUT", { proxy: { https: "file:///etc/passwd" } });
  assert.equal(result.status, 422);
  assert.equal(result.body.error.code, "SETTINGS_INVALID");
});

test("community endpoint exposes the reviewed registry", async () => {
  const result = await request("/v1/community");
  assert.equal(result.status, 200);
  assert.equal(result.body.items[0].id, "modelctl/laya-system-one-mcp");
  assert.deepEqual(result.body.items[0].permissions, ["loopback:modelctl-api"]);
});

test("model preflight marks profiles supported on this host", async () => {
  const result = await request("/v1/models/convaiinnovations%2Flaya");
  assert.equal(result.status, 200);
  const auto = result.body.preflight.profiles.find((profile) => profile.id === "auto");
  assert.equal(auto.supported, true);
});

test("MCP config endpoint returns a client snippet and only saves known entries", async () => {
  const initial = await request("/v1/mcp/config");
  assert.equal(initial.status, 200);
  assert.equal(initial.body.recommended.command, "modelctl-mcp");
  assert.equal(initial.body.recommended.env.MODELCTL_URL, base);
  const saved = await request("/v1/mcp/config", "PUT", { items: [{ id: "modelctl/laya-system-one-mcp", enabled: true }] });
  assert.equal(saved.status, 200);
  assert.equal(saved.body.items[0].enabled, true);
  const invalid = await request("/v1/mcp/config", "PUT", { items: [{ id: "unknown/command", enabled: true, command: "sh" }] });
  assert.equal(invalid.status, 422);
  assert.equal(invalid.body.error.code, "MCP_CONFIG_INVALID");
});
