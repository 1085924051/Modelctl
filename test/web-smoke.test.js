import test from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import fsp from "node:fs/promises";
import { createServer } from "../src/server.js";

test("web shell exposes model, instance, playground, community, and settings routes", async () => {
  const oldDataDir = process.env.MODELCTL_DATA_DIR;
  const dataDir = await fsp.mkdtemp(path.join(os.tmpdir(), "modelctl-web-"));
  process.env.MODELCTL_DATA_DIR = dataDir;
  const server = await createServer({ port: 0 });
  const address = server.address();
  const base = `http://127.0.0.1:${address.port}`;
  try {
    const [html, script, styles] = await Promise.all(["/web/", "/web/app.js", "/web/styles.css"].map((route) => new Promise((resolve, reject) => {
      http.get(`${base}${route}`, (response) => {
        const chunks = [];
        response.on("data", (chunk) => chunks.push(chunk));
        response.on("end", () => resolve(Buffer.concat(chunks).toString("utf8")));
      }).on("error", reject);
    })));
    for (const route of ["models", "instances", "playground", "community", "settings"]) assert.match(html, new RegExp(`href="#${route}"`));
    assert.match(html, /rel="icon" href="data:,"/);
    assert.match(script, /\/v1\/settings/);
    assert.match(script, /\/v1\/community/);
    assert.match(script, /\/v1\/mcp\/config/);
    assert.match(html, /id="settings-form"/);
    assert.match(html, /id="community-list"/);
    assert.match(html, /id="mcp-config-output"/);
    assert.match(styles, /\.community-grid/);
  } finally {
    await new Promise((resolve) => server.close(resolve));
    await fsp.rm(dataDir, { recursive: true, force: true });
    if (oldDataDir === undefined) delete process.env.MODELCTL_DATA_DIR;
    else process.env.MODELCTL_DATA_DIR = oldDataDir;
  }
});
