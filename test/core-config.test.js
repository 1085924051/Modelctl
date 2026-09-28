import test, { after, beforeEach } from "node:test";
import assert from "node:assert/strict";
import os from "node:os";
import path from "node:path";
import fsp from "node:fs/promises";

const originalDataDir = process.env.MODELCTL_DATA_DIR;
const originalProxy = Object.fromEntries(
  ["MODELCTL_HTTP_PROXY", "MODELCTL_HTTPS_PROXY", "MODELCTL_ALL_PROXY", "MODELCTL_NO_PROXY", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy"]
    .map((key) => [key, process.env[key]]),
);
let root;

beforeEach(async () => {
  if (root) await fsp.rm(root, { recursive: true, force: true });
  root = await fsp.mkdtemp(path.join(os.tmpdir(), "modelctl-config-"));
  process.env.MODELCTL_DATA_DIR = root;
  for (const key of Object.keys(originalProxy)) delete process.env[key];
});

after(async () => {
  if (originalDataDir === undefined) delete process.env.MODELCTL_DATA_DIR;
  else process.env.MODELCTL_DATA_DIR = originalDataDir;
  for (const [key, value] of Object.entries(originalProxy)) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
  if (root) await fsp.rm(root, { recursive: true, force: true });
});

test("settings default to empty proxy values and auto profile", async () => {
  const { readSettings } = await import("../src/core.js");
  assert.deepEqual(await readSettings(), {
    schema_version: 1,
    proxy: { http: "", https: "", all: "", no_proxy: "" },
    default_profile: "auto",
  });
});

test("settings and MCP config persist atomically", async () => {
  const { readSettings, writeSettings, readMcpConfig, writeMcpConfig } = await import("../src/core.js");
  await writeSettings({ proxy: { http: "http://127.0.0.1:7897" }, default_profile: "mps" });
  await writeMcpConfig({ items: [{ id: "modelctl/laya-system-one-mcp", enabled: true }] });
  assert.equal((await readSettings()).proxy.http, "http://127.0.0.1:7897");
  assert.equal((await readSettings()).default_profile, "mps");
  assert.deepEqual((await readMcpConfig()).items, [{ id: "modelctl/laya-system-one-mcp", enabled: true }]);
  assert.deepEqual((await fsp.readdir(root)).filter((name) => name.endsWith(".tmp")), []);
});

test("corrupt settings are reported and left untouched", async () => {
  const file = path.join(root, "settings.json");
  await fsp.writeFile(file, "{broken", "utf8");
  const { readSettings } = await import("../src/core.js");
  await assert.rejects(readSettings(), { code: "SETTINGS_UNREADABLE" });
  assert.equal(await fsp.readFile(file, "utf8"), "{broken");
});

test("proxy URL credentials are hidden", async () => {
  const { sanitizeProxyUrl } = await import("../src/core.js");
  assert.equal(sanitizeProxyUrl("http://user:secret@127.0.0.1:7897"), "http://***:***@127.0.0.1:7897");
  assert.equal(sanitizeProxyUrl("socks5://user@proxy.local:1080"), "socks5://***@proxy.local:1080");
});

test("download proxy uses modelctl env, then saved settings, then standard env", async () => {
  const { writeSettings, downloadEnvironment } = await import("../src/core.js");
  await writeSettings({ proxy: { http: "http://saved.local:7897", https: "http://saved.local:7897", all: "socks5://saved.local:7897", no_proxy: "localhost" } });
  const saved = await downloadEnvironment({ HTTP_PROXY: "http://standard.local:8080", HTTPS_PROXY: "http://standard.local:8080" });
  assert.equal(saved.HTTP_PROXY, "http://saved.local:7897");
  assert.equal(saved.HTTPS_PROXY, "http://saved.local:7897");
  assert.equal(saved.ALL_PROXY, "socks5://saved.local:7897");
  assert.equal(saved.NO_PROXY, "localhost");
  const overridden = await downloadEnvironment({ MODELCTL_HTTPS_PROXY: "http://override.local:9000", HTTPS_PROXY: "http://standard.local:8080" });
  assert.equal(overridden.HTTPS_PROXY, "http://override.local:9000");
});

test("invalid proxy URLs are rejected", async () => {
  const { writeSettings } = await import("../src/core.js");
  await assert.rejects(writeSettings({ proxy: { https: "file:///etc/passwd" } }), { code: "SETTINGS_INVALID" });
});

test("community registry contains validated metadata and a safe MCP entry", async () => {
  const { loadCommunityRegistry } = await import("../src/core.js");
  const registry = await loadCommunityRegistry();
  assert.equal(registry.schema_version, 1);
  assert.equal(registry.items[0].kind, "mcp");
  assert.equal(registry.items[0].configuration.command, "modelctl-mcp");
  assert.deepEqual(registry.items[0].permissions, ["loopback:modelctl-api"]);
});
