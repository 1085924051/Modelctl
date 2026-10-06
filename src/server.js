import http from "node:http";
import path from "node:path";
import os from "node:os";
import fsp from "node:fs/promises";
import { ensureDirs, listCatalog, findManifest, findVariant, readState, updateState, jsonResponse, errorResponse, requestId, readJson, idFor, newId, apiError, pullModel, cancelPull, paths, safeId, startLaya, fetchHealth, stopProcess, isManagedProcess, rootDir, readSettings, writeSettings, readMcpConfig, writeMcpConfig, loadCommunityRegistry, sanitizeProxyUrl, downloadEnvironment } from "./core.js";

const webFiles = new Map([
  ["/web/", ["index.html", "text/html; charset=utf-8"]],
  ["/web/app.js", ["app.js", "text/javascript; charset=utf-8"]],
  ["/web/styles.css", ["styles.css", "text/css; charset=utf-8"]],
]);

export async function createServer({ port = Number(process.env.MODELCTL_PORT || 11435), host = process.env.MODELCTL_HOST || "127.0.0.1" } = {}) {
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw new Error("MODELCTL_PORT must be an integer between 0 and 65535");
  const loopback = new Set(["127.0.0.1", "::1", "localhost"]).has(host);
  if (!loopback && !process.env.MODELCTL_API_TOKEN) throw new Error("remote binding requires MODELCTL_API_TOKEN");
  await ensureDirs();
  await updateState((state) => {
    for (const instance of Object.values(state.instances)) {
      if (["preparing", "starting", "ready", "stopping"].includes(instance.status)) instance.status = "orphaned";
    }
    for (const task of Object.values(state.tasks)) {
      if (["queued", "running"].includes(task.status)) { task.status = "failed"; task.error = { code: "DAEMON_RESTARTED", message: "daemon restarted before task completion; a later pull will resume any partial artifact" }; }
    }
  });
  const server = http.createServer(async (req, res) => {
    const rid = requestId(req); res.setHeader("x-request-id", rid);
    try {
      const address = server.address();
      const serverHost = host.includes(":") ? `[${host}]` : host;
      const baseUrl = process.env.MODELCTL_URL || `http://${serverHost}:${typeof address === "object" ? address.port : process.env.MODELCTL_PORT || 11435}`;
      authenticate(req, new URL(req.url, "http://localhost").pathname);
      await route(req, res, rid, baseUrl);
    } catch (error) { errorResponse(res, error, rid); }
  });
  await new Promise((resolve) => server.listen(port, host, resolve));
  return server;
}

async function route(req, res, rid, baseUrl) {
  const url = new URL(req.url, "http://localhost"); const pathname = url.pathname;
  if (req.method === "GET" && (pathname === "/" || pathname === "/web")) { res.writeHead(302, { location: "/web/", "cache-control": "no-store" }); res.end(); return; }
  if (req.method === "GET" && webFiles.has(pathname)) return serveWeb(pathname, res);
  if (req.method === "GET" && pathname === "/health") return jsonResponse(res, 200, { status: "ok" });
  if (req.method === "GET" && pathname === "/v1/settings") return getSettings(res, baseUrl);
  if (req.method === "PUT" && pathname === "/v1/settings") return putSettings(req, res, baseUrl);
  if (req.method === "GET" && pathname === "/v1/community") return jsonResponse(res, 200, await loadCommunityRegistry());
  if (req.method === "GET" && pathname === "/v1/capabilities") return listCapabilities(res);
  if (req.method === "POST" && pathname === "/v1/capabilities") return createCapability(req, res);
  const capabilityMatch = pathname.match(/^\/v1\/capabilities\/([^/]+)$/);
  if (req.method === "GET" && capabilityMatch) return getCapability(capabilityMatch[1], res);
  const invokeCapabilityMatch = pathname.match(/^\/v1\/capabilities\/([^/]+)\/invoke$/);
  if (req.method === "POST" && invokeCapabilityMatch) return invokeCapability(invokeCapabilityMatch[1], req, res);
  const runMatch = pathname.match(/^\/v1\/runs\/([^/]+)$/);
  if (req.method === "GET" && runMatch) return getRun(runMatch[1], res);
  if (req.method === "GET" && pathname === "/v1/runs") return listRuns(res);
  if (req.method === "GET" && pathname === "/v1/mcp/config") return getMcpConfig(res, baseUrl);
  if (req.method === "PUT" && pathname === "/v1/mcp/config") return putMcpConfig(req, res, baseUrl);
  if (req.method === "GET" && pathname === "/v1/models") {
    const state = await readState(); const manifests = await listCatalog();
    return jsonResponse(res, 200, { items: manifests.map((m) => m._invalid ? m : ({ id: m.id, version: m.version, revision: m.source?.revision, variants: m.variants.map((v) => v.id), installed_variants: Object.values(state.models).filter((x) => x.id === m.id && x.version === m.version).map((x) => x.variant), capabilities: (m.capabilities || []).map((x) => x.name), license: m.license?.spdx })) });
  }
  const modelMatch = pathname.match(/^\/v1\/models\/(.+)$/);
  if (req.method === "GET" && modelMatch) {
    const encodedId = modelMatch[1];
    let modelId;
    try { modelId = decodeURIComponent(encodedId); } catch { throw apiError(400, "INVALID_REQUEST", "model id must be URL encoded"); }
    const manifests = await listCatalog();
    const m = findManifest(manifests, modelId, url.searchParams.get("version") || undefined);
    const state = await readState();
    return jsonResponse(res, 200, {
      id: m.id,
      display_name: m.display_name,
      version: m.version,
      source: m.source,
      license: m.license,
      runtime: m.runtime,
      variants: m.variants.map((v) => ({
        id: v.id,
        description: v.description,
        default: !!v.default,
        artifacts: v.artifacts,
        installed: !!state.models[idFor(m.id, m.version, v.id)],
        installed_path: state.models[idFor(m.id, m.version, v.id)]?.path || null,
      })),
      capabilities: m.capabilities || [],
      profiles: m.profiles || [],
      preflight: await modelPreflight(m, state),
    });
  }
  if (req.method === "DELETE" && modelMatch) return removeModel(modelMatch[1], url, res);
  if (req.method === "GET" && pathname === "/v1/catalog/validate") {
    const manifests = await listCatalog(); const invalid = manifests.filter((m) => m._invalid); return jsonResponse(res, invalid.length ? 422 : 200, { valid: invalid.length === 0, items: invalid.length ? invalid : manifests.map((m) => ({ id: m.id, version: m.version, valid: true })) });
  }
  if (req.method === "GET" && pathname.startsWith("/v1/tasks/")) { const s = await readState(); const t = s.tasks[pathname.split("/")[3]]; if (!t) throw apiError(404, "TASK_NOT_FOUND", "task not found"); return jsonResponse(res, 200, t); }
  if (req.method === "POST" && pathname.match(/^\/v1\/tasks\/[^/]+\/cancel$/)) return cancelTask(pathname.split("/")[3], res);
  if (req.method === "GET" && pathname === "/v1/tasks") { const s = await readState(); return jsonResponse(res, 200, { items: Object.values(s.tasks) }); }
  if (req.method === "GET" && pathname === "/v1/instances") { const s = await readState(); return jsonResponse(res, 200, { items: Object.values(s.instances) }); }
  if (req.method === "GET" && pathname.startsWith("/v1/instances/")) { const s = await readState(); const i = s.instances[pathname.split("/")[3]]; if (!i) throw apiError(404, "INSTANCE_NOT_FOUND", "instance not found"); return jsonResponse(res, 200, i); }
  if (req.method === "POST" && pathname === "/v1/pulls") return createPull(req, res);
  if (req.method === "POST" && pathname === "/v1/pulls/stream") return createPullStream(req, res);
  if (req.method === "POST" && pathname === "/v1/instances") return createInstance(req, res);
  if (req.method === "DELETE" && pathname.startsWith("/v1/instances/")) return stopInstance(pathname.split("/")[3], res);
  const opMatch = pathname.match(/^\/v1\/instances\/([^/]+)\/operations\/([^/]+)$/);
  if (req.method === "POST" && opMatch) return invokeInstance(opMatch[1], opMatch[2], req, res);
  if (req.method === "POST" && pathname === "/v1/systemone") return invokeDefault(req, res);
  throw apiError(404, "NOT_FOUND", "route not found");
}

async function getSettings(res, baseUrl) {
  const settings = await readSettings();
  const downloadEnv = await downloadEnvironment();
  const effectiveProxy = {
    http: sanitizeProxyUrl(downloadEnv.HTTP_PROXY || downloadEnv.http_proxy || ""),
    https: sanitizeProxyUrl(downloadEnv.HTTPS_PROXY || downloadEnv.https_proxy || ""),
    all: sanitizeProxyUrl(downloadEnv.ALL_PROXY || downloadEnv.all_proxy || ""),
    no_proxy: downloadEnv.NO_PROXY || downloadEnv.no_proxy || "",
  };
  const manifests = (await listCatalog()).filter((manifest) => !manifest._invalid);
  const profileEntries = manifests.flatMap((manifest) => (manifest.profiles || []).filter((profile) => profileSupportsHost(manifest, profile.id)).map((profile) => [profile.id, profile]));
  const profiles = [...new Map(profileEntries).values()];
  return jsonResponse(res, 200, {
    settings: { ...settings, proxy: Object.fromEntries(Object.entries(settings.proxy).map(([key, value]) => [key, key === "no_proxy" ? value : sanitizeProxyUrl(value)])) },
    effective_proxy: effectiveProxy,
    runtime: { data_dir: paths().root, daemon_url: baseUrl, supported_profiles: profiles.map((profile) => profile.id) },
  });
}

async function putSettings(req, res, baseUrl) {
  const body = objectBody(await readJson(req));
  if (Object.keys(body).some((key) => !["proxy", "default_profile"].includes(key))) throw apiError(422, "SETTINGS_INVALID", "settings contains an unknown field");
  if (body.default_profile !== undefined) {
    const manifests = (await listCatalog()).filter((manifest) => !manifest._invalid);
    const supported = manifests.some((manifest) => profileSupportsHost(manifest, body.default_profile));
    if (!supported) throw apiError(422, "SETTINGS_INVALID", `default profile is not supported on ${hostPlatform()}`);
  }
  await writeSettings(body);
  return getSettings(res, baseUrl);
}

async function getMcpConfig(res, baseUrl) {
  const [registry, config] = await Promise.all([loadCommunityRegistry(), readMcpConfig()]);
  const enabled = new Map(config.items.map((item) => [item.id, item.enabled]));
  const items = registry.items.filter((item) => item.kind === "mcp").map((item) => ({ ...item, enabled: enabled.get(item.id) || false }));
  return jsonResponse(res, 200, { schema_version: 1, items, recommended: { command: "modelctl-mcp", args: [], env: { MODELCTL_URL: baseUrl } } });
}

async function putMcpConfig(req, res, baseUrl) {
  const body = objectBody(await readJson(req));
  if (Object.keys(body).some((key) => !["schema_version", "items"].includes(key))) throw apiError(422, "MCP_CONFIG_INVALID", "MCP configuration contains an unknown field");
  const config = await writeMcpConfig(body);
  const registry = await loadCommunityRegistry();
  const enabled = new Map(config.items.map((item) => [item.id, item.enabled]));
  const items = registry.items.filter((item) => item.kind === "mcp").map((item) => ({ ...item, enabled: enabled.get(item.id) || false }));
  return jsonResponse(res, 200, { schema_version: 1, items, recommended: { command: "modelctl-mcp", args: [], env: { MODELCTL_URL: baseUrl } } });
}

async function serveWeb(pathname, res) {
  const [name, contentType] = webFiles.get(pathname);
  const body = await fsp.readFile(path.join(rootDir, "web", name));
  res.writeHead(200, {
    "content-type": contentType,
    "content-length": body.length,
    "cache-control": "no-store",
    "x-content-type-options": "nosniff",
    "x-frame-options": "DENY",
    "content-security-policy": "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'self'; frame-ancestors 'none'",
  });
  res.end(body);
}

async function createPull(req, res) {
  const result = await startPull(objectBody(await readJson(req)));
  return jsonResponse(res, 202, result);
}

async function startPull(body) {
  if (typeof body.model_id !== "string" || !body.model_id) throw apiError(400, "INVALID_REQUEST", "model_id is required"); const manifests = await listCatalog(); const m = findManifest(manifests, body.model_id, body.version); if (body.revision && body.revision !== m.source?.revision) throw apiError(409, "REVISION_MISMATCH", "requested revision does not match catalog revision"); const v = findVariant(m, body.variant); const taskId = newId("task");
  const state = await readState();
  const duplicate = Object.values(state.tasks).find((task) => task.kind === "pull" && task.model_id === m.id && task.version === m.version && task.variant === v.id && ["queued", "running"].includes(task.status));
  if (duplicate) return { task_id: duplicate.id, status: duplicate.status, poll: `/v1/tasks/${duplicate.id}`, shared: true };
  await updateState((s) => { s.tasks[taskId] = { id: taskId, kind: "pull", status: "queued", model_id: m.id, version: m.version, variant: v.id }; });
  void pullModel(m, v, taskId).catch(async (error) => { await updateState((s) => { if (s.tasks[taskId]) { s.tasks[taskId].status = "failed"; s.tasks[taskId].error = { code: error.code || "DOWNLOAD_FAILED", message: error.message }; } }); });
  return { task_id: taskId, status: "queued", poll: `/v1/tasks/${taskId}` };
}

async function createPullStream(req, res) {
  const task = await startPull(objectBody(await readJson(req)));
  res.writeHead(200, { "content-type": "application/x-ndjson; charset=utf-8", "cache-control": "no-cache", connection: "keep-alive" });
  let closed = false;
  const write = (event) => { if (!closed && !res.destroyed) res.write(`${JSON.stringify(event)}\n`); };
  const emit = async () => {
    if (closed) return;
    const current = (await readState()).tasks[task.task_id];
    if (!current) { write({ status: "error", task_id: task.task_id, error: { code: "TASK_NOT_FOUND", message: "pull task disappeared" } }); res.end(); return; }
    const progress = current.progress || {};
    if (current.status === "queued") write({ status: "queued", task_id: task.task_id, variant: current.variant });
    else if (current.status === "running") write({ status: "downloading", task_id: task.task_id, variant: current.variant, completed: progress.bytes_done || 0, total: progress.bytes_total || 0 });
    else if (current.status === "succeeded") { write({ status: "success", task_id: task.task_id, completed: progress.bytes_done || 0, total: progress.bytes_total || 0 }); res.end(); return; }
    else if (current.status === "cancelled") { write({ status: "cancelled", task_id: task.task_id, error: current.error || null }); res.end(); return; }
    else if (current.status === "failed") { write({ status: "error", task_id: task.task_id, error: current.error || { code: "DOWNLOAD_FAILED", message: "pull failed" } }); res.end(); return; }
    setTimeout(() => { void emit().catch((error) => { if (!closed) { write({ status: "error", task_id: task.task_id, error: { code: error.code || "INTERNAL_ERROR", message: error.message } }); res.end(); } }); }, 500);
  };
  req.on("close", () => { closed = true; });
  write({ status: task.status, task_id: task.task_id, ...(task.shared ? { shared: true } : {}) });
  await emit();
}

async function createInstance(req, res) {
  const body = objectBody(await readJson(req)); if (typeof body.model_id !== "string" || !body.model_id) throw apiError(400, "INVALID_REQUEST", "model_id is required"); const manifests = await listCatalog(); const m = findManifest(manifests, body.model_id, body.version); if (body.revision && body.revision !== m.source?.revision) throw apiError(409, "REVISION_MISMATCH", "requested revision does not match catalog revision"); const v = findVariant(m, body.variant); const profile = body.profile || "auto";
  if (!(m.profiles || []).some((item) => item.id === profile)) throw apiError(400, "INVALID_REQUEST", `unsupported profile: ${profile}`);
  if (!profileSupportsHost(m, profile)) throw apiError(400, "PROFILE_UNSUPPORTED", `profile ${profile} is not supported on ${hostPlatform()}`);
  const s = await readState(); const installed = s.models[idFor(m.id, m.version, v.id)];
  if (!installed || installed.variant !== v.id) throw apiError(409, "MODEL_NOT_INSTALLED", "pull the selected variant before starting it");
  const p = paths(); const id = newId("inst"); const port = body.port === undefined ? await freePort() : Number(body.port); if (!Number.isInteger(port) || port < 1 || port > 65535) throw apiError(400, "INVALID_REQUEST", "port must be an integer between 1 and 65535"); const instance = { id, status: "starting", model: { id: m.id, version: m.version, revision: m.source.revision, variant: v.id }, runtime: `${m.runtime.id}@${m.runtime.version}`, profile, device: profile === "auto" ? "auto" : profile, host: "127.0.0.1", port, log_path: path.join(p.instances, `${id}.log`), started_at: new Date().toISOString(), capabilities: (m.capabilities || []).map((x) => x.name), default: !!body.default };
  await updateState((state) => { if (instance.default) for (const i of Object.values(state.instances)) i.default = false; state.instances[id] = instance; });
  let child;
  try { child = await startLaya(instance, installed.path, v.id, profile); await waitReady(instance, id); }
  catch (error) {
    await stopProcess(child?.pid).catch(() => undefined);
    const code = error.code || "ADAPTER_ERROR";
    await updateState((state) => { state.instances[id].status = "failed"; state.instances[id].error = { code, message: error.message }; });
    throw apiError(error.status || 503, code, error.message, { instance_id: id }, error.retryable ?? true);
  }
  return jsonResponse(res, 200, (await readState()).instances[id]);
}

async function cancelTask(id, res) {
  const state = await readState();
  const task = state.tasks[id];
  if (!task) throw apiError(404, "TASK_NOT_FOUND", "task not found");
  if (["succeeded", "failed", "cancelled"].includes(task.status)) return jsonResponse(res, 200, task);
  cancelPull(id);
  await updateState((next) => { next.tasks[id].status = "cancelled"; next.tasks[id].updated_at = new Date().toISOString(); });
  return jsonResponse(res, 200, (await readState()).tasks[id]);
}

async function removeModel(encodedId, url, res) {
  let modelId;
  try { modelId = decodeURIComponent(encodedId); } catch { throw apiError(400, "INVALID_REQUEST", "model id must be URL encoded"); }
  const manifests = await listCatalog();
  const manifest = findManifest(manifests, modelId, url.searchParams.get("version") || undefined);
  const variant = findVariant(manifest, url.searchParams.get("variant"));
  const key = idFor(manifest.id, manifest.version, variant.id);
  const state = await readState();
  const active = Object.values(state.instances).filter((instance) => instance.model?.id === manifest.id && instance.model?.version === manifest.version && instance.model?.variant === variant.id && !["stopped", "failed", "orphaned"].includes(instance.status));
  if (active.length) throw apiError(409, "MODEL_IN_USE", "stop all instances using this model variant first", { instance_ids: active.map((instance) => instance.id) });
  const installed = state.models[key];
  const purge = url.searchParams.get("purge") === "true";
  let purgedArtifacts = 0;
  if (installed) {
    await updateState((next) => { delete next.models[key]; });
    const expected = path.resolve(paths().models, `${safeId(manifest.id)}@${safeId(manifest.version)}`, safeId(variant.id));
    const root = path.resolve(paths().models);
    if (expected.startsWith(`${root}${path.sep}`)) await fsp.rm(expected, { recursive: true, force: true });
    if (purge) {
      const remainingHashes = new Set();
      for (const record of Object.values(state.models)) {
        if (record === installed) continue;
        const remainingManifest = manifests.find((item) => item.id === record.id && item.version === record.version);
        const remainingVariant = remainingManifest?.variants?.find((item) => item.id === record.variant);
        for (const artifact of remainingVariant?.artifacts || []) remainingHashes.add(artifact.sha256);
      }
      for (const artifact of variant.artifacts) {
        if (!remainingHashes.has(artifact.sha256)) {
          await fsp.rm(path.join(paths().artifacts, artifact.sha256), { force: true });
          purgedArtifacts += 1;
        }
      }
    }
  }
  return jsonResponse(res, 200, { removed: !!installed, model_id: manifest.id, version: manifest.version, variant: variant.id, cache_retained: !purge, purged_artifacts: purgedArtifacts });
}

async function waitReady(instance, id) {
  const until = Date.now() + 30000; while (Date.now() < until) { const current = (await readState()).instances[id]; if (current?.status === "failed") throw apiError(503, current.error?.code || "ADAPTER_ERROR", current.error?.message || "adapter failed to start"); const health = await fetchHealth(instance); if (health) { if (Array.isArray(health.loaded) && !health.loaded.includes(instance.model.variant)) throw apiError(502, "ADAPTER_MODEL_MISMATCH", `adapter loaded ${health.loaded.join(", ") || "no variant"}; expected ${instance.model.variant}`); await updateState((s) => { const active = s.instances[id]; active.status = "ready"; if (typeof health.device === "string") active.device = health.device; if (Array.isArray(health.loaded)) active.loaded_variants = health.loaded; active.ready_at = new Date().toISOString(); }); return; } await new Promise((r) => setTimeout(r, 250)); }
  throw apiError(503, "SERVICE_UNAVAILABLE", "adapter did not become ready", undefined, true);
}

async function stopInstance(id, res) {
  const state = await readState();
  const instance = state.instances[id];
  if (!instance) throw apiError(404, "INSTANCE_NOT_FOUND", "instance not found");
  if (instance.status === "stopped") return jsonResponse(res, 200, instance);
  if (instance.status === "orphaned" || (instance.pid && !isManagedProcess(instance.pid) && !["failed"].includes(instance.status))) {
    throw apiError(409, "INSTANCE_ORPHANED", "instance belongs to a previous daemon process");
  }
  await updateState((next) => { next.instances[id].status = "stopping"; });
  await stopProcess(instance.pid);
  await updateState((next) => { if (next.instances[id]) { next.instances[id].status = "stopped"; delete next.instances[id].pid; } });
  return jsonResponse(res, 200, (await readState()).instances[id]);
}

async function invokeDefault(req, res) { const s = await readState(); const ready = Object.values(s.instances).filter((x) => x.status === "ready" && x.capabilities?.includes("system_one")); const selected = ready.find((x) => x.default); if (!selected && ready.length > 1) throw apiError(409, "INSTANCE_AMBIGUOUS", "multiple ready instances; select a default or use the instance endpoint"); const i = selected || ready[0]; if (!i) throw apiError(503, "SERVICE_UNAVAILABLE", "no ready default instance"); return invokeInstance(i.id, "system_one", req, res); }

async function invokeInstance(id, operation, req, res) {
  const body = await readJson(req);
  const result = await invokeInstanceBody(id, operation, body);
  return jsonResponse(res, 200, result);
}

function authenticate(req, pathname) {
  const token = process.env.MODELCTL_API_TOKEN;
  if (!token || pathname === "/health") return;
  const provided = req.headers.authorization?.replace(/^Bearer\s+/i, "");
  if (provided !== token) throw apiError(401, "UNAUTHORIZED", "a valid Modelctl API token is required");
}

async function invokeInstanceBody(id, operation, body) {
  const s = await readState(); const i = s.instances[id];
  if (!i) throw apiError(404, "INSTANCE_NOT_FOUND", "instance not found");
  if (i.status !== "ready") throw apiError(503, "SERVICE_UNAVAILABLE", "instance is not ready", undefined, true);
  const targetPath = operation === "system_one" ? "/v1/systemone" : `/v1/operation/${encodeURIComponent(operation)}`;
  const target = `http://${i.host}:${i.port}${targetPath}`; let response;
  try { response = await fetch(target, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body), signal: AbortSignal.timeout(120000) }); }
  catch (error) { throw apiError(502, "ADAPTER_UNREACHABLE", `adapter request failed: ${error.name === "TimeoutError" ? "timeout" : "connection error"}`, undefined, true); }
  const text = await response.text(); let parsed;
  try { parsed = JSON.parse(text); } catch { parsed = { raw: text }; }
  if (!response.ok) throw apiError(response.status, parsed?.error?.code || "ADAPTER_ERROR", parsed?.error?.message || `adapter returned HTTP ${response.status}`);
  return parsed;
}

function capabilityId(value) { return typeof value === "string" && /^[a-z0-9][a-z0-9._-]{1,63}$/.test(value); }

function validateCapability(body) {
  if (!body || typeof body !== "object" || Array.isArray(body)) throw apiError(400, "INVALID_REQUEST", "capability must be an object");
  if (!capabilityId(body.id)) throw apiError(422, "CAPABILITY_INVALID", "id must use lowercase letters, digits, dot, underscore, or dash");
  if (typeof body.name !== "string" || !body.name.trim()) throw apiError(422, "CAPABILITY_INVALID", "name is required");
  if (typeof body.description !== "string") throw apiError(422, "CAPABILITY_INVALID", "description must be a string");
  if (!body.model || typeof body.model !== "object" || typeof body.model.model_id !== "string" || typeof body.model.variant !== "string") throw apiError(422, "CAPABILITY_INVALID", "model_id and variant are required");
  if (!body.questions || typeof body.questions !== "object" || Array.isArray(body.questions) || !Object.keys(body.questions).length) throw apiError(422, "CAPABILITY_INVALID", "questions must be a non-empty object");
  for (const [id, question] of Object.entries(body.questions)) {
    if (!/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/.test(id) || !question || typeof question !== "object") throw apiError(422, "CAPABILITY_INVALID", `invalid question ${id}`);
    if (!["noul", "choice", "score"].includes(question.type) || typeof question.instructions !== "string" || !question.instructions.trim()) throw apiError(422, "CAPABILITY_INVALID", `question ${id} needs type and instructions`);
    if (["choice", "score"].includes(question.type) && (!question.criteria || (Array.isArray(question.criteria) && !question.criteria.length))) throw apiError(422, "CAPABILITY_INVALID", `question ${id} needs criteria`);
  }
  const version = typeof body.version === "string" && body.version.trim() ? body.version.trim() : "1.0.0";
  return { schema_version: 1, id: body.id, version, name: body.name.trim(), description: body.description.trim(), model: { model_id: body.model.model_id, version: body.model.version || undefined, variant: body.model.variant, profile: body.model.profile || "auto" }, input: body.input && typeof body.input === "object" ? body.input : { type: "text", field: "text" }, questions: body.questions, created_at: body.created_at || new Date().toISOString(), updated_at: new Date().toISOString() };
}

async function listCapabilities(res) { const state = await readState(); return jsonResponse(res, 200, { items: Object.values(state.capabilities || {}).sort((a, b) => a.id.localeCompare(b.id)) }); }
async function getCapability(id, res) { const state = await readState(); const item = state.capabilities[id]; if (!item) throw apiError(404, "CAPABILITY_NOT_FOUND", "capability not found"); return jsonResponse(res, 200, item); }
async function createCapability(req, res) { const capability = validateCapability(await readJson(req)); await updateState((state) => { state.capabilities[capability.id] = capability; }); return jsonResponse(res, 201, capability); }

async function invokeCapability(id, req, res) {
  const state = await readState(); const capability = state.capabilities[id]; if (!capability) throw apiError(404, "CAPABILITY_NOT_FOUND", "capability not found");
  const body = objectBody(await readJson(req)); const input = objectBody(body.input || body); const field = capability.input?.field || "text"; const text = input[field];
  if (typeof text !== "string" || !text.trim()) throw apiError(422, "INPUT_INVALID", `${field} must be a non-empty string`);
  const ready = Object.values(state.instances).filter((instance) => instance.status === "ready" && instance.capabilities?.includes("system_one") && instance.model?.id === capability.model.model_id && instance.model?.variant === capability.model.variant);
  const selected = ready.find((instance) => instance.default) || ready[0]; if (!selected) throw apiError(503, "CAPABILITY_NOT_READY", "start the capability model instance before invoking it", { model_id: capability.model.model_id, variant: capability.model.variant }, true);
  const response = await invokeInstanceBody(selected.id, "system_one", { state: { body: text.trim() }, questions: capability.questions });
  const run = { id: newId("run"), capability_id: capability.id, capability_version: capability.version, instance_id: selected.id, input: { [field]: text.trim() }, response, created_at: new Date().toISOString() };
  await updateState((next) => { next.runs[run.id] = run; });
  return jsonResponse(res, 200, { request_id: run.id, capability: { id: capability.id, version: capability.version }, output: response.answers || response, raw: response, model: selected.model, run_id: run.id });
}

async function getRun(id, res) { const state = await readState(); const run = state.runs[id]; if (!run) throw apiError(404, "RUN_NOT_FOUND", "run not found"); return jsonResponse(res, 200, run); }
async function listRuns(res) { const state = await readState(); const items = Object.values(state.runs || {}).sort((a, b) => String(b.created_at || "").localeCompare(String(a.created_at || ""))).slice(0, 100); return jsonResponse(res, 200, { items }); }

async function freePort() { const net = await import("node:net"); return new Promise((resolve, reject) => { const s = net.createServer(); s.listen(0, "127.0.0.1", () => { const p = s.address().port; s.close(() => resolve(p)); }); s.on("error", reject); }); }

function objectBody(body) { if (!body || typeof body !== "object" || Array.isArray(body)) throw apiError(400, "INVALID_REQUEST", "request body must be a JSON object"); return body; }

function hostPlatform() {
  const osName = process.platform === "darwin" ? "macos" : process.platform === "linux" ? "linux" : process.platform === "win32" ? "windows" : process.platform;
  const arch = process.arch === "x64" ? "x86_64" : process.arch;
  return `${osName}-${arch}`;
}

function profileSupportsHost(manifest, profile) {
  const definition = (manifest.profiles || []).find((item) => item.id === profile);
  return !!definition && (!Array.isArray(definition.platforms) || definition.platforms.includes(hostPlatform()));
}

async function modelPreflight(manifest, state) {
  const freeBytes = await freeDiskBytes(paths().root);
  const variants = manifest.variants.map((variant) => ({ id: variant.id, installed: !!state.models[idFor(manifest.id, manifest.version, variant.id)], disk_bytes: variant.artifacts.reduce((sum, artifact) => sum + artifact.size_bytes, 0) }));
  return { platform: hostPlatform(), memory_bytes: os.totalmem(), free_disk_bytes: freeBytes, profiles: (manifest.profiles || []).map((profile) => ({ id: profile.id, supported: profileSupportsHost(manifest, profile.id), device: profile.device })), variants };
}

async function freeDiskBytes(directory) {
  try { const stat = await fsp.statfs(directory); return Number(stat.bavail) * Number(stat.bsize); } catch { return null; }
}

export async function shutdownInstances() {
  const state = await readState();
  const active = Object.values(state.instances).filter((instance) => instance.pid && ["starting", "ready", "stopping"].includes(instance.status));
  await Promise.all(active.map((instance) => stopProcess(instance.pid).catch(() => undefined)));
  if (active.length) await updateState((next) => { for (const instance of active) if (next.instances[instance.id]) next.instances[instance.id].status = "stopped"; });
}
