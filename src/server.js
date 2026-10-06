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
const capabilityStarts = new Map();

export async function createServer({ port = Number(process.env.MODELCTL_PORT || 11435), host = process.env.MODELCTL_HOST || "127.0.0.1" } = {}) {
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw new Error("MODELCTL_PORT must be an integer between 0 and 65535");
  const loopback = new Set(["127.0.0.1", "::1", "localhost"]).has(host);
  if (!loopback && !process.env.MODELCTL_API_TOKEN && !process.env.MODELCTL_API_INVOKE_TOKEN) throw new Error("remote binding requires MODELCTL_API_TOKEN or MODELCTL_API_INVOKE_TOKEN");
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
  if (req.method === "GET" && capabilityMatch) return getCapability(capabilityMatch[1], url.searchParams.get("version"), res);
  const capabilitySchemaMatch = pathname.match(/^\/v1\/capabilities\/([^/]+)\/schema$/);
  if (req.method === "GET" && capabilitySchemaMatch) return getCapabilitySchema(capabilitySchemaMatch[1], url.searchParams.get("version"), res);
  const capabilityIntegrationMatch = pathname.match(/^\/v1\/capabilities\/([^/]+)\/integration$/);
  if (req.method === "GET" && capabilityIntegrationMatch) return getCapabilityIntegration(capabilityIntegrationMatch[1], url.searchParams.get("version"), baseUrl, res);
  const capabilityOpenAPIMatch = pathname.match(/^\/v1\/capabilities\/([^/]+)\/openapi$/);
  if (req.method === "GET" && capabilityOpenAPIMatch) return getCapabilityOpenAPI(capabilityOpenAPIMatch[1], url.searchParams.get("version"), baseUrl, res);
  const capabilityStatusMatch = pathname.match(/^\/v1\/capabilities\/([^/]+)\/status$/);
  if (req.method === "GET" && capabilityStatusMatch) return getCapabilityStatus(capabilityStatusMatch[1], url.searchParams.get("version"), res);
  const invokeCapabilityMatch = pathname.match(/^\/v1\/capabilities\/([^/]+)\/invoke$/);
  if (req.method === "POST" && invokeCapabilityMatch) return invokeCapability(invokeCapabilityMatch[1], url.searchParams.get("version"), req, res);
  const batchCapabilityMatch = pathname.match(/^\/v1\/capabilities\/([^/]+)\/batch$/);
  if (req.method === "POST" && batchCapabilityMatch) return createCapabilityBatch(batchCapabilityMatch[1], url.searchParams.get("version"), req, res);
  const activateCapabilityMatch = pathname.match(/^\/v1\/capabilities\/([^/]+)\/activate$/);
  if (req.method === "POST" && activateCapabilityMatch) return activateCapability(activateCapabilityMatch[1], req, res);
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
  const body = objectBody(await readJson(req));
  return jsonResponse(res, 200, await startInstance(body));
}

async function startInstance(body) {
  if (typeof body.model_id !== "string" || !body.model_id) throw apiError(400, "INVALID_REQUEST", "model_id is required"); const manifests = await listCatalog(); const m = findManifest(manifests, body.model_id, body.version); if (body.revision && body.revision !== m.source?.revision) throw apiError(409, "REVISION_MISMATCH", "requested revision does not match catalog revision"); const v = findVariant(m, body.variant); const profile = body.profile || "auto";
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
  return (await readState()).instances[id];
}

async function cancelTask(id, res) {
  const state = await readState();
  const task = state.tasks[id];
  if (!task) throw apiError(404, "TASK_NOT_FOUND", "task not found");
  if (["succeeded", "failed", "cancelled"].includes(task.status)) return jsonResponse(res, 200, task);
  if (task.kind === "pull") cancelPull(id);
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
  const adminToken = process.env.MODELCTL_API_TOKEN;
  const invokeToken = process.env.MODELCTL_API_INVOKE_TOKEN;
  if ((!adminToken && !invokeToken) || pathname === "/health") return;
  const provided = req.headers.authorization?.replace(/^Bearer\s+/i, "");
  if (provided && adminToken && provided === adminToken) return;
  if (provided && invokeToken && provided === invokeToken) {
    if (isInvokeOnlyRoute(req.method, pathname)) return;
    throw apiError(403, "FORBIDDEN", "the invoke token cannot manage models or capabilities");
  }
  throw apiError(401, "UNAUTHORIZED", "a valid Modelctl API token is required");
}

function isInvokeOnlyRoute(method, pathname) {
  if (method === "GET" && (pathname === "/v1/capabilities" || /^\/v1\/capabilities\/[^/]+(?:\/schema|\/integration|\/openapi|\/status)?$/.test(pathname) || /^\/v1\/tasks\/[^/]+$/.test(pathname))) return true;
  if (method === "POST" && /^\/v1\/capabilities\/[^/]+\/(invoke|batch)$/.test(pathname)) return true;
  return false;
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
  if (!/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(version)) throw apiError(422, "CAPABILITY_VERSION_INVALID", "version must use semantic version format, for example 1.0.0");
  const input = validateInputContract(body.input);
  return { schema_version: 1, id: body.id, version, name: body.name.trim(), description: body.description.trim(), model: { model_id: body.model.model_id, version: body.model.version || undefined, variant: body.model.variant, profile: body.model.profile || "auto" }, input, questions: body.questions, created_at: body.created_at || new Date().toISOString(), updated_at: new Date().toISOString() };
}

function validateInputContract(input) {
  if (input === undefined) return { type: "text", field: "text" };
  if (!input || typeof input !== "object" || Array.isArray(input)) throw apiError(422, "CAPABILITY_INVALID", "input must be an object");
  if (input.fields !== undefined) {
    if (!input.fields || typeof input.fields !== "object" || Array.isArray(input.fields) || !Object.keys(input.fields).length) throw apiError(422, "CAPABILITY_INVALID", "input.fields must be a non-empty object");
    if (typeof input.template !== "string" || !input.template.trim()) throw apiError(422, "CAPABILITY_INVALID", "input.template is required when input.fields is used");
    const fields = {};
    for (const [id, definition] of Object.entries(input.fields)) {
      if (!/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/.test(id) || !definition || typeof definition !== "object" || Array.isArray(definition)) throw apiError(422, "CAPABILITY_INVALID", `invalid input field ${id}`);
      const type = definition.type || "string";
      if (!["string", "number", "boolean"].includes(type)) throw apiError(422, "CAPABILITY_INVALID", `input field ${id} has unsupported type`);
      if (definition.required !== undefined && typeof definition.required !== "boolean") throw apiError(422, "CAPABILITY_INVALID", `input field ${id}.required must be boolean`);
      if (definition.description !== undefined && typeof definition.description !== "string") throw apiError(422, "CAPABILITY_INVALID", `input field ${id}.description must be a string`);
      fields[id] = { type, required: definition.required !== false, ...(definition.description?.trim() ? { description: definition.description.trim() } : {}) };
    }
    const placeholders = [...input.template.matchAll(/\{\{\s*([a-zA-Z][a-zA-Z0-9_-]{0,63})\s*\}\}/g)].map((match) => match[1]);
    if (!placeholders.length) throw apiError(422, "CAPABILITY_INVALID", "input.template must reference at least one field");
    for (const placeholder of placeholders) if (!Object.hasOwn(fields, placeholder)) throw apiError(422, "CAPABILITY_INVALID", `input.template references unknown field ${placeholder}`);
    return { type: "object", fields, template: input.template.trim() };
  }
  const field = input.field || "text";
  if (typeof field !== "string" || !/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/.test(field)) throw apiError(422, "CAPABILITY_INVALID", "input.field must be a valid field name");
  return { type: "text", field };
}

function resolveCapability(state, id, version) {
  const item = version ? state.capability_versions?.[id]?.[version] : state.capabilities?.[id];
  if (!item) throw apiError(404, "CAPABILITY_NOT_FOUND", version ? `capability ${id}@${version} not found` : "capability not found");
  return item;
}

async function listCapabilities(res) {
  const state = await readState();
  const items = Object.values(state.capabilities || {}).sort((a, b) => a.id.localeCompare(b.id)).map((item) => ({ ...item, active: true, available_versions: Object.keys(state.capability_versions?.[item.id] || {}).sort() }));
  return jsonResponse(res, 200, { items });
}
async function getCapability(id, version, res) { const state = await readState(); const item = resolveCapability(state, id, version); return jsonResponse(res, 200, { ...item, active: state.capabilities[id]?.version === item.version, available_versions: Object.keys(state.capability_versions?.[id] || {}).sort() }); }
async function getCapabilitySchema(id, version, res) {
  const state = await readState(); const capability = resolveCapability(state, id, version);
  const field = capability.input?.field || "text";
  const outputProperties = Object.fromEntries(Object.entries(capability.questions || {}).map(([questionID, question]) => [questionID, answerSchema(question)]));
  const inputSchema = capability.input?.fields ? structuredInputSchema(capability.input) : { type: "object", required: [field], properties: { [field]: { type: "string", description: "Business text sent to the capability." } } };
  return jsonResponse(res, 200, {
    capability: { id: capability.id, version: capability.version, name: capability.name },
    request_schema: { type: "object", required: ["input"], properties: { input: inputSchema, metadata: { type: "object" } } },
    response_schema: { type: "object", required: ["output", "run_id"], properties: { output: { type: "object", required: Object.keys(outputProperties), properties: outputProperties }, raw: { type: "object" }, model: { type: "object" }, metadata: { type: "object" }, run_id: { type: "string" } } },
  });
}

async function getCapabilityIntegration(id, version, baseUrl, res) {
  const state = await readState();
  const capability = resolveCapability(state, id, version);
  const schema = await capabilitySchema(capability);
  const endpoint = `${baseUrl}/v1/capabilities/${encodeURIComponent(capability.id)}/invoke?version=${encodeURIComponent(capability.version)}`;
  const input = sampleCapabilityInput(capability.input);
  const payload = JSON.stringify({ input, metadata: { ticket_id: "T-100" } }, null, 2);
  const authRequired = Boolean(process.env.MODELCTL_API_TOKEN || process.env.MODELCTL_API_INVOKE_TOKEN);
  const tokenEnvironmentVariable = process.env.MODELCTL_API_INVOKE_TOKEN ? "MODELCTL_API_INVOKE_TOKEN" : "MODELCTL_API_TOKEN";
  const authHeader = authRequired ? `Authorization: Bearer $${tokenEnvironmentVariable}\n` : "";
  const curlAuth = authRequired ? `  -H 'Authorization: Bearer $${tokenEnvironmentVariable}' \\\n` : "";
  const examples = {
    curl: `curl ${endpoint} \\\n${curlAuth}  -H 'Content-Type: application/json' \\\n  -d '${payload.replaceAll("'", "'\\\"'\\\"'")}'`,
    python: `${authRequired ? "import os\n" : ""}from modelctl_client import Modelctl\n\nclient = Modelctl(${JSON.stringify(baseUrl)}${authRequired ? `, token=os.environ.get("${tokenEnvironmentVariable}")` : ""})\nresult = client.invoke(${JSON.stringify(capability.id)}, ${JSON.stringify(input)}, {"ticket_id": "T-100"}, ${JSON.stringify(capability.version)})\nprint(result["output"])`,
    javascript: `import { Modelctl } from "modelctl-client";\n\nconst client = new Modelctl(${JSON.stringify(baseUrl)}${authRequired ? `, process.env.${tokenEnvironmentVariable}` : ""});\nconst result = await client.invoke(${JSON.stringify(capability.id)}, ${JSON.stringify(input)}, { ticket_id: "T-100" }, ${JSON.stringify(capability.version)});\nconsole.log(result.output);`,
  };
  return jsonResponse(res, 200, {
    capability: { id: capability.id, version: capability.version, name: capability.name, description: capability.description },
    base_url: baseUrl,
    endpoint,
    auth: { required: authRequired, scheme: authRequired ? "bearer" : "none", header: authHeader.trim() || null, environment_variable: authRequired ? tokenEnvironmentVariable : null, invoke_only: Boolean(process.env.MODELCTL_API_INVOKE_TOKEN) },
    request_schema: schema.request_schema,
    response_schema: schema.response_schema,
    examples,
    errors: ["CAPABILITY_NOT_FOUND", "MODEL_NOT_INSTALLED", "INPUT_INVALID", "UNAUTHORIZED"],
  });
}

async function getCapabilityOpenAPI(id, version, baseUrl, res) {
  const state = await readState();
  const capability = resolveCapability(state, id, version);
  const schema = await capabilitySchema(capability);
  const requestSchemaName = `${capability.id.replaceAll(/[^A-Za-z0-9]/g, "_")}_invoke_request`;
  const responseSchemaName = `${capability.id.replaceAll(/[^A-Za-z0-9]/g, "_")}_invoke_response`;
  const authRequired = Boolean(process.env.MODELCTL_API_TOKEN || process.env.MODELCTL_API_INVOKE_TOKEN);
  const security = authRequired ? [{ bearerAuth: [] }] : undefined;
  const versionParameter = { name: "version", in: "query", required: true, description: "Capability version. The generated contract is pinned to this version.", schema: { type: "string", enum: [capability.version] } };
  const operationSecurity = security ? { security } : {};
  const document = {
    openapi: "3.1.0",
    info: { title: `${capability.name} · Modelctl capability`, version: capability.version, description: capability.description },
    servers: [{ url: baseUrl }],
    paths: {
      [`/v1/capabilities/${encodeURIComponent(capability.id)}/invoke`]: {
        post: { operationId: `${capability.id.replaceAll(/[^A-Za-z0-9]/g, "_")}_invoke`, summary: capability.name, parameters: [versionParameter], ...operationSecurity, requestBody: { required: true, content: { "application/json": { schema: { $ref: `#/components/schemas/${requestSchemaName}` } } } }, responses: { "200": { description: "Capability result", content: { "application/json": { schema: { $ref: `#/components/schemas/${responseSchemaName}` } } } }, "400": { description: "Invalid request" }, "401": { description: "Unauthorized" }, "409": { description: "Model is not installed" }, "422": { description: "Input contract validation failed" } } },
      },
      [`/v1/capabilities/${encodeURIComponent(capability.id)}/batch`]: {
        post: { operationId: `${capability.id.replaceAll(/[^A-Za-z0-9]/g, "_")}_batch`, summary: `${capability.name} batch`, parameters: [versionParameter], ...operationSecurity, requestBody: { required: true, content: { "application/json": { schema: { type: "object", required: ["items"], properties: { items: { type: "array", minItems: 1, maxItems: 1000, items: { $ref: `#/components/schemas/${requestSchemaName}` } } } } } } }, responses: { "202": { description: "Batch task accepted" }, "422": { description: "Invalid batch" } } },
      },
    },
    components: {
      schemas: { [requestSchemaName]: schema.request_schema, [responseSchemaName]: schema.response_schema },
      ...(authRequired ? { securitySchemes: { bearerAuth: { type: "http", scheme: "bearer", bearerFormat: "token" } } } : {}),
    },
  };
  return jsonResponse(res, 200, document);
}

async function getCapabilityStatus(id, version, res) {
  const state = await readState();
  const capability = resolveCapability(state, id, version);
  const installed = Object.values(state.models || {}).find((item) => item.id === capability.model.model_id && item.variant === capability.model.variant && (!capability.model.version || item.version === capability.model.version));
  const ready = matchingCapabilityInstances(state, capability);
  const selected = ready.find((instance) => instance.default) || ready[0];
  const status = selected ? "ready" : installed ? "not_started" : "model_not_installed";
  const nextAction = status === "ready" ? "ready" : status === "model_not_installed" ? "download_model" : "invoke_capability_or_start_runtime";
  return jsonResponse(res, 200, {
    capability: { id: capability.id, version: capability.version, active: state.capabilities?.[capability.id]?.version === capability.version },
    model: { model_id: capability.model.model_id, version: capability.model.version || installed?.version || null, variant: capability.model.variant, installed: Boolean(installed) },
    runtime: { ready: Boolean(selected), instance_id: selected?.id || null, status: selected?.status || null },
    ready: status === "ready",
    status,
    next_action: nextAction,
    checked_at: new Date().toISOString(),
  });
}

async function capabilitySchema(capability) {
  const field = capability.input?.field || "text";
  const outputProperties = Object.fromEntries(Object.entries(capability.questions || {}).map(([questionID, question]) => [questionID, answerSchema(question)]));
  const inputSchema = capability.input?.fields ? structuredInputSchema(capability.input) : { type: "object", required: [field], properties: { [field]: { type: "string" } } };
  return {
    request_schema: { type: "object", required: ["input"], properties: { input: inputSchema, metadata: { type: "object" } } },
    response_schema: { type: "object", required: ["output", "run_id"], properties: { output: { type: "object", required: Object.keys(outputProperties), properties: outputProperties }, raw: { type: "object" }, model: { type: "object" }, metadata: { type: "object" }, run_id: { type: "string" } } },
  };
}

function sampleCapabilityInput(inputContract) {
  if (inputContract?.fields) {
    return Object.fromEntries(Object.entries(inputContract.fields).map(([id, definition]) => [id, definition.type === "number" ? 0 : definition.type === "boolean" ? false : "your text here"]));
  }
  return { [inputContract?.field || "text"]: "your text here" };
}

function answerSchema(question) {
  const common = { type: { type: "string", enum: [question.type] }, value: {}, confidence: { type: "number" }, probabilities: { type: "object" } };
  const description = question.instructions?.trim() || undefined;
  const criteria = question.criteria === undefined ? undefined : { "x-modelctl-criteria": question.criteria };
  if (question.type === "choice") {
    const labels = Array.isArray(question.criteria) ? question.criteria : Object.keys(question.criteria || {});
    const value = { type: "string", ...(labels.length ? { enum: labels } : {}) };
    return { type: "object", ...(description ? { description } : {}), ...(criteria || {}), required: ["type", "value", "choice"], properties: { ...common, value, choice: value } };
  }
  if (question.type === "score") {
    const value = { type: ["number", "string", "object", "array", "null"] };
    return { type: "object", ...(description ? { description } : {}), ...(criteria || {}), required: ["type", "value", "score"], properties: { ...common, value, score: value } };
  }
  const value = { type: "number", minimum: 0, maximum: 1 };
  return { type: "object", ...(description ? { description } : {}), ...(criteria || {}), required: ["type", "value", "noul"], properties: { ...common, value, noul: value } };
}
async function createCapability(req, res) {
  const body = objectBody(await readJson(req)); const activate = body.activate !== false; const capability = validateCapability(body);
  await updateState((state) => {
    state.capability_versions[capability.id] ||= {};
    if (state.capability_versions[capability.id][capability.version]) throw apiError(409, "CAPABILITY_VERSION_EXISTS", `capability ${capability.id}@${capability.version} already exists; publish a new version`);
    state.capability_versions[capability.id][capability.version] = capability;
    if (activate || !state.capabilities[capability.id]) state.capabilities[capability.id] = capability;
  });
  const state = await readState(); return jsonResponse(res, 201, { ...capability, active: state.capabilities[capability.id]?.version === capability.version, available_versions: Object.keys(state.capability_versions[capability.id]).sort() });
}

async function activateCapability(id, req, res) {
  const body = objectBody(await readJson(req)); if (typeof body.version !== "string" || !body.version.trim()) throw apiError(422, "CAPABILITY_VERSION_INVALID", "version is required");
  await updateState((state) => { const capability = state.capability_versions?.[id]?.[body.version]; if (!capability) throw apiError(404, "CAPABILITY_NOT_FOUND", `capability ${id}@${body.version} not found`); state.capabilities[id] = capability; });
  return getCapability(id, body.version, res);
}

async function invokeCapability(id, version, req, res) {
  return jsonResponse(res, 200, await executeCapability(id, version, objectBody(await readJson(req))));
}

async function executeCapability(id, version, body) {
  const state = await readState(); const capability = resolveCapability(state, id, version);
  const input = objectBody(body.input || body); const { text, normalizedInput } = normalizeCapabilityInput(capability.input, input);
  const metadata = body.metadata === undefined ? undefined : objectBody(body.metadata);
  const ready = matchingCapabilityInstances(state, capability);
  let selected = ready.find((instance) => instance.default) || ready[0];
  if (!selected) selected = await ensureCapabilityInstance(capability);
  const response = await invokeInstanceBody(selected.id, "system_one", { state: { body: text.trim() }, questions: capability.questions });
  const run = { id: newId("run"), capability_id: capability.id, capability_version: capability.version, instance_id: selected.id, input: normalizedInput, ...(metadata ? { metadata } : {}), response, created_at: new Date().toISOString() };
  await updateState((next) => { next.runs[run.id] = run; });
  return { request_id: run.id, capability: { id: capability.id, version: capability.version }, output: normalizeAnswers(response.answers || response), raw: response, model: selected.model, ...(metadata ? { metadata } : {}), run_id: run.id };
}

function normalizeAnswers(answers) {
  if (!answers || typeof answers !== "object" || Array.isArray(answers)) return answers;
  return Object.fromEntries(Object.entries(answers).map(([id, answer]) => {
    if (!answer || typeof answer !== "object") return [id, answer];
    const value = answer.type === "choice" ? answer.choice : answer.type === "score" ? answer.score : answer.noul;
    return [id, { ...answer, value }];
  }));
}

function normalizeCapabilityInput(contract, input) {
  if (contract?.fields) {
    const fields = contract.fields;
    const unknown = Object.keys(input).filter((key) => !Object.hasOwn(fields, key));
    if (unknown.length) throw apiError(422, "INPUT_INVALID", `unknown input field: ${unknown[0]}`);
    const values = {};
    for (const [id, definition] of Object.entries(fields)) {
      const value = input[id];
      if (value === undefined || value === null || value === "") {
        if (definition.required) throw apiError(422, "INPUT_INVALID", `${id} is required`);
        values[id] = "";
        continue;
      }
      if (definition.type === "number" && (typeof value !== "number" || !Number.isFinite(value))) throw apiError(422, "INPUT_INVALID", `${id} must be a number`);
      if (definition.type === "boolean" && typeof value !== "boolean") throw apiError(422, "INPUT_INVALID", `${id} must be a boolean`);
      if (definition.type === "string" && typeof value !== "string") throw apiError(422, "INPUT_INVALID", `${id} must be a string`);
      values[id] = value;
    }
    const text = contract.template.replace(/\{\{\s*([a-zA-Z][a-zA-Z0-9_-]{0,63})\s*\}\}/g, (_, id) => String(values[id] ?? "")).trim();
    if (!text) throw apiError(422, "INPUT_INVALID", "input template produced an empty model input");
    return { text, normalizedInput: values };
  }
  const field = contract?.field || "text";
  const value = input[field];
  if (typeof value !== "string" || !value.trim()) throw apiError(422, "INPUT_INVALID", `${field} must be a non-empty string`);
  return { text: value.trim(), normalizedInput: { [field]: value.trim() } };
}

function structuredInputSchema(input) {
  const properties = Object.fromEntries(Object.entries(input.fields).map(([id, definition]) => [id, { type: definition.type, ...(definition.description ? { description: definition.description } : {}) }]));
  const required = Object.entries(input.fields).filter(([, definition]) => definition.required).map(([id]) => id);
  return { type: "object", ...(required.length ? { required } : {}), properties };
}

function matchingCapabilityInstances(state, capability) {
  return Object.values(state.instances).filter((instance) => {
    if (instance.status !== "ready" || !instance.capabilities?.includes("system_one")) return false;
    if (instance.model?.id !== capability.model.model_id || instance.model?.variant !== capability.model.variant) return false;
    return !capability.model.version || instance.model?.version === capability.model.version;
  });
}

async function ensureCapabilityInstance(capability) {
  const version = capability.model.version || "catalog";
  const key = `${capability.model.model_id}@${version}#${capability.model.variant}#${capability.model.profile || "auto"}`;
  const existing = capabilityStarts.get(key);
  if (existing) return existing;
  const pending = (async () => {
    const state = await readState();
    const installed = Object.values(state.models).find((item) => item.id === capability.model.model_id && item.variant === capability.model.variant && (!capability.model.version || item.version === capability.model.version));
    if (!installed) throw apiError(409, "MODEL_NOT_INSTALLED", "download the capability model before invoking it", { model_id: capability.model.model_id, version: capability.model.version, variant: capability.model.variant });
    return startInstance({ model_id: installed.id, version: installed.version, variant: installed.variant, profile: capability.model.profile || "auto", default: true });
  })();
  capabilityStarts.set(key, pending);
  try { return await pending; } finally { if (capabilityStarts.get(key) === pending) capabilityStarts.delete(key); }
}

async function createCapabilityBatch(id, version, req, res) {
  const body = objectBody(await readJson(req));
  if (!Array.isArray(body.items) || body.items.length === 0 || body.items.length > 1000) throw apiError(422, "BATCH_INVALID", "items must contain between 1 and 1000 inputs");
  const state = await readState(); const capability = resolveCapability(state, id, version);
  const taskId = newId("task");
  await updateState((next) => { next.tasks[taskId] = { id: taskId, kind: "capability_batch", status: "queued", capability_id: id, capability_version: capability.version, progress: { items_done: 0, items_total: body.items.length }, results: [], errors: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() }; });
  void processCapabilityBatch(taskId, id, capability.version, body.items).catch(() => undefined);
  return jsonResponse(res, 202, { task_id: taskId, status: "queued", poll: `/v1/tasks/${taskId}` });
}

async function processCapabilityBatch(taskId, id, version, items) {
  await updateState((state) => { if (state.tasks[taskId]) { state.tasks[taskId].status = "running"; state.tasks[taskId].started_at = new Date().toISOString(); } });
  for (let index = 0; index < items.length; index += 1) {
    const current = (await readState()).tasks[taskId]; if (!current || current.status === "cancelled") return;
    try { const result = await executeCapability(id, version, objectBody(items[index])); await updateState((state) => { const task = state.tasks[taskId]; if (task) { task.results.push({ index, run_id: result.run_id, request_id: result.request_id, output: result.output, ...(result.metadata ? { metadata: result.metadata } : {}) }); task.progress.items_done = index + 1; task.updated_at = new Date().toISOString(); } }); }
    catch (error) { await updateState((state) => { const task = state.tasks[taskId]; if (task) { task.errors.push({ index, code: error.code || "BATCH_ITEM_FAILED", message: error.message }); task.progress.items_done = index + 1; task.updated_at = new Date().toISOString(); } }); }
  }
  await updateState((state) => { const task = state.tasks[taskId]; if (task && task.status !== "cancelled") { task.status = "succeeded"; task.finished_at = new Date().toISOString(); task.updated_at = task.finished_at; } });
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
