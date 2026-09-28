# Local Model Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing Modelctl daemon and Web UI with persistent local settings, a reviewed community registry, and MCP configuration management while preserving Laya model lifecycle behavior.

**Architecture:** Keep the Node.js daemon as the single local control plane and keep the Python Laya adapter unchanged. Add small persistence and validation helpers in the existing core layer, expose loopback-only JSON endpoints from the daemon, and add hash-routed vanilla Web UI views that consume those endpoints. Community entries are metadata and configuration only; the UI never executes third-party commands.

**Tech Stack:** Node.js 20+ ES modules, native `node:fs/promises`, native `node:test` and `node:assert/strict`, existing HTTP server, vanilla HTML/CSS/JavaScript.

**Spec:** `docs/superpowers/specs/2026-09-28-local-model-workbench-design.md`

## Global Constraints

- Preserve the existing Node.js daemon and Python Laya adapter boundary.
- Keep the daemon bound to loopback by default.
- Preserve the native Laya `system_one` request and response shape.
- Do not execute community commands, install scripts, or arbitrary shell input.
- Use atomic temporary-file plus rename writes for persistent JSON state.
- Redact proxy credentials and MCP environment values from API responses and errors.
- Preserve the existing local executable-bit changes on `bin/modelctl.js` and `bin/modelctl-mcp.js`.
- Keep `#models` as the default Web UI route.

---

### Task 1: Add configuration and community data primitives

**Files:**
- Create: `community/index.json`
- Modify: `src/core.js:18-70` and adjacent persistence helpers
- Test: `test/core-config.test.js`

**Interfaces:**
- Consumes: `dataRoot()`, `paths()`, `ensureDirs()`, existing atomic state helpers.
- Produces: `settingsPath()`, `mcpPath()`, `defaultSettings()`, `readSettings()`, `writeSettings()`, `readMcpConfig()`, `writeMcpConfig()`, `loadCommunityRegistry()`, `sanitizeProxyUrl()`, and `downloadEnvironment()` behavior backed by persisted settings.

- [ ] **Step 1: Write failing tests for defaults, persistence, redaction, and registry loading**

```js
import test from "node:test";
import assert from "node:assert/strict";
import os from "node:os";
import path from "node:path";
import fsp from "node:fs/promises";

test("settings default to loopback proxy and auto profile", async () => {
  const root = await fsp.mkdtemp(path.join(os.tmpdir(), "modelctl-settings-"));
  process.env.MODELCTL_DATA_DIR = root;
  const { readSettings } = await import("../src/core.js");
  assert.equal((await readSettings()).default_profile, "auto");
});

test("proxy credentials are redacted when read for API use", async () => {
  const { sanitizeProxyUrl } = await import("../src/core.js");
  assert.equal(sanitizeProxyUrl("http://user:secret@127.0.0.1:7897"), "http://***:***@127.0.0.1:7897");
});
```

- [ ] **Step 2: Run the focused test and verify it fails because the helpers do not exist**

Run: `node --test test/core-config.test.js`

Expected: FAIL with a missing export or missing helper error.

- [ ] **Step 3: Implement atomic JSON persistence and schema defaults**

Add paths under `dataRoot()`, create parent directories in `ensureDirs()`, and implement `readJsonFile()` and `writeJsonFile()` using a sibling temporary file and `fsp.rename()`. Missing settings return:

```js
{
  schema_version: 1,
  proxy: { http: "", https: "", all: "", no_proxy: "" },
  default_profile: "auto"
}
```

Missing MCP configuration returns `{ schema_version: 1, items: [] }`. Invalid JSON throws an error with `code = "SETTINGS_UNREADABLE"` or `code = "MCP_CONFIG_INVALID"` without replacing the source file. `community/index.json` contains one reviewed `mcp` entry for the current `modelctl-mcp` command and the current repository revision.

- [ ] **Step 4: Add proxy precedence and redaction helpers**

Implement per-field precedence in this order: `MODELCTL_HTTP_PROXY`, `MODELCTL_HTTPS_PROXY`, `MODELCTL_ALL_PROXY`, `MODELCTL_NO_PROXY`; persisted settings; standard lowercase/uppercase proxy environment variables; empty value. `downloadEnvironment()` must return a child-process environment with the selected values and no credentials in any diagnostic string. Validate only `http`, `https`, and `socks5` URL schemes and reject malformed values with `SETTINGS_INVALID`.

- [ ] **Step 5: Add community registry validation**

Validate the top-level schema version and each item’s `id`, `kind`, `name`, `version`, `description`, `source.url`, `source.revision`, `permissions`, and optional `configuration`. Reject unknown item kinds and invalid URLs with `COMMUNITY_INVALID`. Return a deep copy so route handlers cannot mutate the loaded registry.

- [ ] **Step 6: Run the focused test and verify it passes**

Run: `node --test test/core-config.test.js`

Expected: PASS for defaults, atomic read/write, proxy redaction, precedence, malformed settings, and community validation.

- [ ] **Step 7: Commit the primitives**

```bash
git add community/index.json src/core.js test/core-config.test.js
git commit -m "feat: add local settings and community registry primitives"
```

### Task 2: Expose settings, community, and MCP APIs

**Files:**
- Modify: `src/server.js:30-90` route table and new route helpers
- Modify: `src/mcp.js:1-80` only where configuration discovery needs a stable entry description
- Test: `test/server-settings.test.js`

**Interfaces:**
- Consumes: Task 1 helpers.
- Produces: `GET/PUT /v1/settings`, `GET /v1/community`, `GET/PUT /v1/mcp/config`.

- [ ] **Step 1: Write failing endpoint tests**

Start a server on port `0` with `MODELCTL_DATA_DIR` set to a temporary directory and assert:

```js
assert.deepEqual((await get("/v1/settings")).settings.proxy, { http: "", https: "", all: "", no_proxy: "" });
assert.equal((await get("/v1/community")).schema_version, 1);
assert.equal((await get("/v1/mcp/config")).recommended.command, "modelctl-mcp");
assert.equal((await put("/v1/settings", { proxy: { http: "http://127.0.0.1:7897" }, default_profile: "mps" })).settings.proxy.http, "http://127.0.0.1:7897");
```

Also assert malformed proxy URLs return HTTP 422 with `SETTINGS_INVALID`, unknown MCP fields return HTTP 422 with `MCP_CONFIG_INVALID`, and a proxy password never appears in the returned body.

- [ ] **Step 2: Run the endpoint tests and verify they fail**

Run: `node --test test/server-settings.test.js`

Expected: FAIL with HTTP 404 for the new routes.

- [ ] **Step 3: Add route handlers and request validation**

Add routes before the model route catch-all. `GET /v1/settings` returns sanitized settings plus read-only `data_dir`, daemon URL, and supported profiles. `PUT /v1/settings` merges only allowed fields, validates profile IDs against the catalog profiles, writes atomically, and returns the effective sanitized settings. `GET /v1/community` returns the validated registry. `GET /v1/mcp/config` returns the local config plus a recommended config object; `PUT` validates enabled state, item IDs, command `modelctl-mcp`, empty args, and a restricted environment key set before writing.

- [ ] **Step 4: Make persisted settings affect future downloads**

Update the existing download child environment construction so settings are read when a pull begins. Environment variables keep precedence over persisted settings. The running daemon does not need a restart for a saved proxy to affect the next pull.

- [ ] **Step 5: Run endpoint and existing syntax checks**

Run: `node --test test/server-settings.test.js`; `node --check src/core.js`; `node --check src/server.js`; `npm run catalog:validate`.

Expected: all tests pass and the catalog remains valid.

- [ ] **Step 6: Commit the APIs**

```bash
git add src/core.js src/server.js src/mcp.js test/server-settings.test.js
git commit -m "feat: expose settings community and MCP APIs"
```

### Task 3: Add Web navigation and settings/community/MCP views

**Files:**
- Modify: `web/index.html`
- Modify: `web/app.js`
- Modify: `web/styles.css`
- Test: `test/web-smoke.test.js`

**Interfaces:**
- Consumes: Task 2 endpoints and existing model/instance/playground state.
- Produces: hash routes `#models`, `#instances`, `#playground`, `#community`, and `#settings`; settings form, community cards, MCP configuration copy action, and visible loading/error/saved states.

- [ ] **Step 1: Write a route and markup smoke test**

Fetch `/web/`, `/web/app.js`, and `/web/styles.css`; assert the HTML contains navigation links and the script contains `settings`, `community`, and `mcp/config`. This test is intentionally static because the repository has no browser test dependency.

- [ ] **Step 2: Run the smoke test and verify it fails**

Run: `node --test test/web-smoke.test.js`

Expected: FAIL because the new navigation labels and route names are absent.

- [ ] **Step 3: Add accessible navigation and route containers**

Add a compact navigation bar with text and familiar icons already available in the existing visual language. Keep the model view as the default. Add one main content container per route and ensure route changes do not rebuild the model state unnecessarily.

- [ ] **Step 4: Implement settings view and save flow**

Load `/v1/settings` when entering `#settings`. Render editable HTTP, HTTPS, ALL proxy, `NO_PROXY`, and default profile fields; render data directory and daemon URL read-only. Submit `PUT /v1/settings`, show a saved state, and surface API errors without clearing the previous values. Do not render passwords from a proxy URL; show only the redacted value.

- [ ] **Step 5: Implement community and MCP views**

Load `/v1/community` and `/v1/mcp/config`. Render each item’s name, kind, version, source link, revision, permissions, and configuration summary. Add a copy button using `navigator.clipboard.writeText()` with a fallback message when clipboard access is unavailable. Render MCP enable/disable state and save it through `PUT /v1/mcp/config`. Never attach a click handler that executes the displayed command.

- [ ] **Step 6: Preserve and harden existing model actions**

Keep variant selection and profile selection in model cards. Ensure settings/community fetch failures show local view error states and do not disable or erase model controls. Keep start and download buttons tied to the selected variant and chosen profile.

- [ ] **Step 7: Run static smoke and syntax checks**

Run: `node --test test/web-smoke.test.js`; `node --check web/app.js`; `curl --noproxy '*' -fsS http://127.0.0.1:11435/web/`.

Expected: PASS and the page source contains all new route/view hooks.

- [ ] **Step 8: Commit the Web UI**

```bash
git add web/index.html web/app.js web/styles.css test/web-smoke.test.js
git commit -m "feat: add settings community and MCP web views"
```

### Task 4: Document usage and verify end-to-end behavior

**Files:**
- Modify: `README.md`
- Modify: `API_CONTRACT.md`
- Modify: `USER_FLOWS.md`
- Test: `test/e2e-local.test.js`

**Interfaces:**
- Consumes: Tasks 1-3 public APIs and Web routes.
- Produces: documented setup, proxy, MCP, community, and troubleshooting flows plus an end-to-end local verification script.

- [ ] **Step 1: Write the end-to-end verification script**

Use the running local daemon and assert `/health`, `/v1/settings`, `/v1/community`, `/v1/mcp/config`, `/v1/models`, and `/v1/instances` all return valid JSON. If a ready instance exists, send one existing `system_one` request and assert `answers` is present. The script must not download a model or change user settings.

- [ ] **Step 2: Run the verification script**

Run: `node --test test/e2e-local.test.js`

Expected: PASS against the local daemon, or a clearly reported skip for the optional inference assertion when no ready instance exists.

- [ ] **Step 3: Update the documentation**

Document `#settings`, `#community`, `#instances`, `#playground`, the `7897` proxy example, MCP client configuration, the fact that community entries are metadata-only in this release, and the commands to start/stop the daemon. Add the new endpoint table to `API_CONTRACT.md` and update the user flows.

- [ ] **Step 4: Run the complete verification set**

Run:

```bash
node --test test/*.test.js
node --check src/core.js
node --check src/server.js
node --check src/cli.js
node --check web/app.js
npm run catalog:validate
git diff --check
```

Expected: all tests pass, catalog validation reports `OK convaiinnovations/laya@0.3.18`, and no whitespace errors appear.

- [ ] **Step 5: Review the diff and commit documentation**

```bash
git diff --stat origin/main...HEAD
git status --short --branch
git add README.md API_CONTRACT.md USER_FLOWS.md test/e2e-local.test.js
git commit -m "docs: document local workbench extensions"
```

Do not stage `bin/modelctl.js`, `bin/modelctl-mcp.js`, or unrelated user changes.

### Task 5: Restart locally and publish the completed work

**Files:**
- No source files; operate on the local daemon and Git remote.

**Interfaces:**
- Consumes: all committed changes from Tasks 1-4.
- Produces: a restarted local daemon, verified Web UI, and a pushed `main` branch.

- [ ] **Step 1: Restart the daemon with the corrected local proxy**

Use `HTTP_PROXY=http://127.0.0.1:7897`, `HTTPS_PROXY=http://127.0.0.1:7897`, `ALL_PROXY=socks5://127.0.0.1:7897`, `NO_PROXY=127.0.0.1,localhost`, and `NODE_OPTIONS=--use-env-proxy`. Stop only the current Modelctl daemon through its existing PID and let `modelctl ps` start the new daemon, then start the default English instance with `--profile auto`.

- [ ] **Step 2: Verify the Web UI and API after restart**

Check `curl --noproxy '*' -fsS http://127.0.0.1:11435/health`, the settings/community/MCP endpoints, the Web UI redirect, and `modelctl ps`. Confirm the default instance is `ready` and the browser can open `http://127.0.0.1:11435/web/#models`.

- [ ] **Step 3: Review the final Git state**

Run `git status --short --branch` and confirm only the intended source, docs, tests, and community files are staged or committed; preserve the two existing executable-bit changes.

- [ ] **Step 4: Push to the configured repository**

```bash
git push origin main
```

Expected: the remote `main` branch advances from the local implementation commit without force push.

- [ ] **Step 5: Record the final commit and runtime URLs**

Report the pushed commit, Web UI URL, daemon health URL, MCP command, tests run, and any remaining limitations such as metadata-only community entries.
