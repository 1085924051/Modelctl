import fs from "node:fs";
import fsp from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import crypto from "node:crypto";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
export const rootDir = path.resolve(here, "..");
export const catalogDir = path.join(rootDir, "catalog");
let stateWriteQueue = Promise.resolve();
const managedChildren = new Map();
const pullLocks = new Map();
const activePulls = new Map();

export function dataRoot() {
  return process.env.MODELCTL_DATA_DIR || path.join(os.homedir(), ".modelctl");
}

export function pythonExecutable() {
  if (process.env.MODELCTL_PYTHON) return process.env.MODELCTL_PYTHON;
  const managed = process.platform === "win32"
    ? path.join(dataRoot(), "venv", "Scripts", "python.exe")
    : path.join(dataRoot(), "venv", "bin", "python");
  if (fs.existsSync(managed)) return managed;
  return process.platform === "win32" ? "python" : "python3";
}

export function paths() {
  const root = dataRoot();
  return {
    root,
    state: path.join(root, "state.json"),
    artifacts: path.join(root, "artifacts", "sha256"),
    models: path.join(root, "models"),
    instances: path.join(root, "instances"),
    tasks: path.join(root, "tasks"),
  };
}

export async function ensureDirs() {
  const p = paths();
  await Promise.all([p.root, p.artifacts, p.models, p.instances, p.tasks].map((x) => fsp.mkdir(x, { recursive: true })));
  if (await exists(p.state)) {
    const stat = await fsp.stat(p.state);
    if (stat.isDirectory()) {
      // Early builds accidentally created an empty directory named state.json.
      // Only migrate that exact empty case; never delete user data in it.
      const contents = await fsp.readdir(p.state);
      if (contents.length) throw new Error(`state path is a non-empty directory: ${p.state}`);
      await fsp.rmdir(p.state);
      await atomicWrite(p.state, { models: {}, tasks: {}, instances: {}, events: [] });
    } else if (!stat.isFile()) {
      throw new Error(`state path is not a file: ${p.state}`);
    }
  } else {
    await atomicWrite(p.state, { models: {}, tasks: {}, instances: {}, events: [] });
  }
}

export async function exists(file) {
  try { await fsp.access(file); return true; } catch { return false; }
}

export async function readState() {
  await ensureDirs();
  return JSON.parse(await fsp.readFile(paths().state, "utf8"));
}

export async function updateState(mutator) {
  const operation = stateWriteQueue.then(async () => {
    const state = await readState();
    const result = await mutator(state) || state;
    await atomicWrite(paths().state, result);
    return result;
  });
  stateWriteQueue = operation.catch(() => undefined);
  return operation;
}

export async function atomicWrite(file, value) {
  await fsp.mkdir(path.dirname(file), { recursive: true });
  const tmp = `${file}.${process.pid}.${Date.now()}.tmp`;
  await fsp.writeFile(tmp, JSON.stringify(value, null, 2) + "\n", "utf8");
  for (let attempt = 0; ; attempt++) {
    try { await fsp.rename(tmp, file); break; }
    catch (error) {
      if (process.platform !== "win32" || !["EPERM", "EACCES"].includes(error.code) || attempt >= 9) { await fsp.rm(tmp, { force: true }); throw error; }
      await new Promise((resolve) => setTimeout(resolve, 25 * (attempt + 1)));
    }
  }
}

export function idFor(modelId, version, variant = undefined) { return `${modelId}@${version}${variant ? `#${variant}` : ""}`; }
export function safeId(value) { return value.replace(/[^a-zA-Z0-9._-]+/g, "_"); }
export function safeRelativePath(value) {
  if (typeof value !== "string" || !value || value.includes("\\") || /[\u0000<>:"|?*]/.test(value) || path.posix.isAbsolute(value)) throw new Error(`invalid artifact path ${value}`);
  if (value.split("/").some((part) => !part || part === "." || part === "..")) throw new Error(`invalid artifact path ${value}`);
  return value;
}

export async function listCatalog() {
  await ensureDirs();
  const names = (await fsp.readdir(catalogDir)).filter((x) => x.endsWith(".json"));
  const result = [];
  for (const name of names) {
    try {
      const manifest = JSON.parse(await fsp.readFile(path.join(catalogDir, name), "utf8"));
      validateManifest(manifest);
      result.push(manifest);
    } catch (error) {
      result.push({ _invalid: name, error: error.message });
    }
  }
  return result;
}

export function validateManifest(m) {
  const required = ["schema_version", "id", "version", "license", "runtime", "variants"];
  for (const key of required) if (m?.[key] === undefined) throw new Error(`manifest missing ${key}`);
  if (m.schema_version !== 1) throw new Error("unsupported manifest schema_version");
  if (typeof m.id !== "string" || !/^[a-z0-9][a-z0-9._-]*(\/[a-z0-9][a-z0-9._-]*)+$/.test(m.id)) throw new Error("invalid manifest id");
  if (typeof m.version !== "string" || !/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(m.version)) throw new Error("invalid manifest version");
  if (!m.license || typeof m.license !== "object" || typeof m.license.spdx !== "string") throw new Error("invalid manifest license");
  if (!m.runtime || typeof m.runtime !== "object" || typeof m.runtime.id !== "string" || typeof m.runtime.version !== "string") throw new Error("invalid manifest runtime");
  if (!Array.isArray(m.variants) || m.variants.length === 0) throw new Error("manifest needs variants");
  const variantIds = new Set(); let defaults = 0;
  for (const variant of m.variants) {
    if (!variant.id || !Array.isArray(variant.artifacts) || variant.artifacts.length === 0) throw new Error(`variant ${variant.id || "?"} has no artifacts`);
    if (!/^[a-z0-9][a-z0-9._-]*$/.test(variant.id) || variantIds.has(variant.id)) throw new Error(`invalid or duplicate variant ${variant.id}`);
    variantIds.add(variant.id); if (variant.default) defaults += 1;
    for (const artifact of variant.artifacts) {
      safeRelativePath(artifact.path);
      if (!/^https:\/\//.test(artifact.uri)) throw new Error(`artifact URI must use HTTPS: ${artifact.path}`);
      if (!/^[a-f0-9]{64}$/i.test(artifact.sha256)) throw new Error(`invalid sha256 for ${artifact.path}`);
      if (!Number.isInteger(artifact.size_bytes) || artifact.size_bytes < 0) throw new Error(`invalid size for ${artifact.path}`);
      if (artifact.executable !== false) throw new Error(`artifact must not be executable: ${artifact.path}`);
    }
  }
  if (defaults > 1) throw new Error("manifest may have at most one default variant");
  for (const profile of m.profiles || []) {
    if (!profile.id || !/^[a-z0-9][a-z0-9._-]*$/.test(profile.id)) throw new Error(`invalid profile ${profile.id || "?"}`);
    if (!["auto", "cpu", "cuda", "mps", "metal", "vulkan"].includes(profile.device)) throw new Error(`invalid profile device ${profile.device}`);
  }
  return true;
}

export function findManifest(manifests, modelId, version) {
  const m = manifests.find((x) => x.id === modelId && (!version || x.version === version));
  if (!m || m._invalid) throw apiError(404, "MODEL_NOT_FOUND", `model not found: ${modelId}`);
  return m;
}

export function findVariant(manifest, variantId) {
  const v = variantId ? manifest.variants.find((x) => x.id === variantId) : manifest.variants.find((x) => x.default);
  if (!v) throw apiError(404, "VARIANT_NOT_FOUND", `variant not found: ${variantId}`);
  return v;
}

export function apiError(status, code, message, details = undefined, retryable = false) {
  const error = new Error(message); error.status = status; error.code = code; error.details = details; error.retryable = retryable; return error;
}

export function jsonResponse(res, status, body, headers = {}) {
  const payload = JSON.stringify(body);
  res.writeHead(status, { "content-type": "application/json", "content-length": Buffer.byteLength(payload), ...headers });
  res.end(payload);
}

export function errorResponse(res, error, requestId) {
  const status = error.status || 500;
  jsonResponse(res, status, { error: { code: error.code || "INTERNAL_ERROR", message: error.message || "internal error", request_id: requestId, retryable: !!error.retryable, ...(error.details ? { details: error.details } : {}) } });
}

export function requestId(req) { return req.headers["x-request-id"] || `req_${crypto.randomUUID()}`; }

export async function readJson(req, maxBytes = 2 * 1024 * 1024) {
  const chunks = []; let size = 0;
  for await (const chunk of req) { size += chunk.length; if (size > maxBytes) throw apiError(413, "INPUT_TOO_LARGE", "request body too large"); chunks.push(chunk); }
  try { return JSON.parse(Buffer.concat(chunks).toString("utf8")); } catch { throw apiError(400, "INVALID_REQUEST", "request body must be valid JSON"); }
}

export async function hashFile(file) {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash("sha256"); const stream = fs.createReadStream(file);
    stream.on("data", (chunk) => hash.update(chunk)); stream.on("error", reject); stream.on("end", () => resolve(hash.digest("hex")));
  });
}

async function download(url, destination, expected, expectedSize, onProgress, signal) {
  await fsp.mkdir(path.dirname(destination), { recursive: true });
  const temp = `${destination}.part`;
  let resume = await exists(temp);
  if (resume) {
    const partial = await fsp.stat(temp);
    if (partial.size >= expectedSize) { await fsp.rm(temp, { force: true }); resume = false; }
  }
  let response;
  try { if (resume) throw Object.assign(new Error("resume with Python"), { cause: { code: "UND_ERR_CONNECT_TIMEOUT" } }); response = await fetch(url, { redirect: "follow", signal }); }
  catch (error) {
    if (error.cause?.code !== "UND_ERR_CONNECT_TIMEOUT" && error.cause?.code !== "ENETUNREACH") throw error;
    return downloadWithPython(url, destination, temp, expected, expectedSize, onProgress, resume, signal);
  }
  if (!response.ok || !response.body || !String(response.url || url).startsWith("https://")) throw apiError(502, "DOWNLOAD_FAILED", `download failed (${response.status}): ${url}`, undefined, true);
  const total = Number(response.headers.get("content-length") || 0); let received = 0;
  const out = fs.createWriteStream(temp); const hash = crypto.createHash("sha256");
  try {
    for await (const chunk of response.body) { if (signal?.aborted) throw apiError(499, "TASK_CANCELLED", "pull task cancelled"); const b = Buffer.from(chunk); received += b.length; if (expectedSize > 0 && received > expectedSize) throw apiError(422, "ARTIFACT_SIZE_MISMATCH", `download exceeded declared size for ${path.basename(destination)}`); hash.update(b); if (!out.write(b)) await new Promise((resolve) => out.once("drain", resolve)); await onProgress?.(received, total); }
  } catch (error) {
    await new Promise((resolve) => out.end(resolve));
    await fsp.rm(temp, { force: true });
    throw error;
  }
  await new Promise((resolve) => out.end(resolve));
  const actual = hash.digest("hex");
  if (signal?.aborted) { await fsp.rm(temp, { force: true }); throw apiError(499, "TASK_CANCELLED", "pull task cancelled"); }
  if (received !== expectedSize) { await fsp.rm(temp, { force: true }); throw apiError(422, "ARTIFACT_SIZE_MISMATCH", `size mismatch for ${path.basename(destination)}`, { expected_size: expectedSize, actual_size: received }); }
  if (actual !== expected) { await fsp.rm(temp, { force: true }); throw apiError(422, "ARTIFACT_CORRUPT", `sha256 mismatch for ${path.basename(destination)}`, { expected_sha256: expected, actual_sha256: actual }); }
  await fsp.rename(temp, destination); return received;
}

async function downloadWithPython(url, destination, temp, expected, expectedSize, onProgress, resume = false, signal) {
  const python = pythonExecutable();
  const helper = path.join(rootDir, "scripts", "download-artifact.py");
  const child = spawn(python, [helper, url, temp, ...(resume ? ["--resume"] : [])], { stdio: ["ignore", "pipe", "pipe"] });
  if (signal?.aborted) child.kill();
  signal?.addEventListener("abort", () => child.kill(), { once: true });
  let stderr = ""; let lines = "";
  const progressWrites = [];
  child.stderr.setEncoding("utf8"); child.stderr.on("data", (chunk) => { stderr += chunk; });
  let lastProgress = 0;
  child.stdout.setEncoding("utf8"); child.stdout.on("data", (chunk) => {
    lines += chunk;
    const parts = lines.split("\n"); lines = parts.pop();
    for (const part of parts) { const count = Number(part.trim()); if (Number.isFinite(count) && (count - lastProgress >= 16 * 1024 * 1024 || count === expectedSize)) { lastProgress = count; progressWrites.push(Promise.resolve().then(() => onProgress?.(count, expectedSize)).catch((error) => { stderr += `\nprogress update failed: ${error.message}`; })); } }
  });
  const code = await new Promise((resolve, reject) => { child.once("error", reject); child.once("close", resolve); });
  await Promise.all(progressWrites);
  if (signal?.aborted) { await fsp.rm(temp, { force: true }); throw apiError(499, "TASK_CANCELLED", "pull task cancelled"); }
  if (code !== 0) throw apiError(502, "DOWNLOAD_FAILED", `Python download failed: ${stderr.trim() || code}`, undefined, true);
  const stat = await fsp.stat(temp);
  if (stat.size !== expectedSize) { await fsp.rm(temp, { force: true }); throw apiError(422, "ARTIFACT_SIZE_MISMATCH", `size mismatch for ${path.basename(destination)}`, { expected_size: expectedSize, actual_size: stat.size }); }
  const actual = await hashFile(temp);
  if (actual !== expected) { await fsp.rm(temp, { force: true }); throw apiError(422, "ARTIFACT_CORRUPT", `sha256 mismatch for ${path.basename(destination)}`, { expected_sha256: expected, actual_sha256: actual }); }
  await fsp.rename(temp, destination);
  return stat.size;
}

export async function pullModel(manifest, variant, taskId) {
  const lockKey = idFor(manifest.id, manifest.version, variant.id);
  const previous = pullLocks.get(lockKey) || Promise.resolve();
  const controller = new AbortController();
  activePulls.set(taskId, controller);
  const current = previous.catch(() => undefined).then(() => pullModelUnlocked(manifest, variant, taskId, controller.signal));
  pullLocks.set(lockKey, current);
  current.finally(() => { if (pullLocks.get(lockKey) === current) pullLocks.delete(lockKey); }).catch(() => undefined);
  return current.finally(() => activePulls.delete(taskId));
}

export function cancelPull(taskId) { activePulls.get(taskId)?.abort(); }

async function pullModelUnlocked(manifest, variant, taskId, signal) {
  const p = paths(); const modelKey = `${safeId(manifest.id)}@${safeId(manifest.version)}`; const variantDir = path.join(p.models, modelKey, safeId(variant.id));
  await updateState((s) => { if (s.tasks[taskId]?.status === "cancelled" || signal.aborted) return; s.tasks[taskId] = { id: taskId, kind: "pull", status: "running", model_id: manifest.id, version: manifest.version, variant: variant.id, progress: { bytes_done: 0, bytes_total: variant.artifacts.reduce((n, a) => n + a.size_bytes, 0) }, started_at: new Date().toISOString(), updated_at: new Date().toISOString() }; });
  let done = 0; try {
    for (const artifact of variant.artifacts) {
      const current = (await readState()).tasks[taskId];
      if (current?.status === "cancelled" || signal.aborted) throw apiError(499, "TASK_CANCELLED", "pull task cancelled");
      const target = path.join(variantDir, ...safeRelativePath(artifact.path).split("/")); const artifactStore = path.join(p.artifacts, artifact.sha256);
      let lastReported = 0;
      const reportProgress = async (n) => { if (n - lastReported >= 16 * 1024 * 1024 || n === artifact.size_bytes) { lastReported = n; await updateTask(taskId, { progress: { bytes_done: done + n, bytes_total: variant.artifacts.reduce((x, a) => x + a.size_bytes, 0) } }); } };
      if (!(await exists(artifactStore)) || (await hashFile(artifactStore)) !== artifact.sha256) await download(artifact.uri, artifactStore, artifact.sha256, artifact.size_bytes, reportProgress, signal);
      await fsp.mkdir(path.dirname(target), { recursive: true });
      if (await exists(target)) {
        if (await hashFile(target) !== artifact.sha256) await fsp.rm(target, { force: true });
      }
      if (!(await exists(target))) await fsp.copyFile(artifactStore, target);
      done += artifact.size_bytes; await updateTask(taskId, { progress: { bytes_done: done, bytes_total: variant.artifacts.reduce((x, a) => x + a.size_bytes, 0) } });
    }
    if (signal.aborted) throw apiError(499, "TASK_CANCELLED", "pull task cancelled");
    await updateState((s) => { if (s.tasks[taskId].status === "cancelled" || signal.aborted) return; s.tasks[taskId].status = "succeeded"; s.tasks[taskId].updated_at = new Date().toISOString(); s.models[idFor(manifest.id, manifest.version, variant.id)] = { id: manifest.id, version: manifest.version, revision: manifest.source.revision, variant: variant.id, path: variantDir, installed_at: new Date().toISOString() }; });
  } catch (error) { await updateState((s) => { if (s.tasks[taskId]?.status !== "cancelled") { s.tasks[taskId].status = "failed"; s.tasks[taskId].error = { code: error.code || "DOWNLOAD_FAILED", message: error.message }; } s.tasks[taskId].updated_at = new Date().toISOString(); }); }
}

async function updateTask(taskId, patch) { await updateState((s) => { if (s.tasks[taskId]) Object.assign(s.tasks[taskId], patch, { updated_at: new Date().toISOString() }); }); }

export function newId(prefix) { return `${prefix}_${crypto.randomUUID()}`; }

export async function startLaya(instance, modelPath, variant, profile) {
  const adapter = path.join(rootDir, "adapters", "laya", "serve.py");
  const python = pythonExecutable();
  const child = spawn(python, [adapter, "--model-dir", modelPath, "--variant", variant, "--device", profile === "auto" ? "" : profile, "--host", "127.0.0.1", "--port", String(instance.port)], { cwd: rootDir, env: { ...process.env, PYTHONUNBUFFERED: "1" }, stdio: ["ignore", "pipe", "pipe"] });
  const log = fs.createWriteStream(instance.log_path, { flags: "a" }); child.stdout.pipe(log); child.stderr.pipe(log); instance.pid = child.pid;
  if (child.pid) {
    managedChildren.set(child.pid, child);
    await updateState((state) => { if (state.instances[instance.id]) state.instances[instance.id].pid = child.pid; });
  }
  child.on("error", (error) => updateState((s) => { const i = s.instances[instance.id]; if (i && !["stopped", "stopping"].includes(i.status)) { i.status = "failed"; i.error = { code: "ADAPTER_START_FAILED", message: error.code === "ENOENT" ? `Python executable not found: ${python}` : error.message }; } }));
  child.on("exit", (code, signal) => { if (child.pid) managedChildren.delete(child.pid); updateState((s) => { const i = s.instances[instance.id]; if (i && !["stopped", "stopping", "failed"].includes(i.status)) { i.status = "failed"; i.error = { code: "WORKER_EXITED", message: `adapter exited (${code ?? signal})` }; } }); });
  return child;
}

export async function stopProcess(pid) {
  if (!pid) return;
  const child = managedChildren.get(pid);
  if (!child) return false;
  if (child.exitCode !== null || child.signalCode !== null) return true;
  const exited = new Promise((resolve) => child.once("exit", resolve));
  try { child.kill("SIGTERM"); } catch (error) { if (error.code !== "ESRCH") throw error; }
  const graceful = await waitFor(exited.then(() => true), 5000, false);
  if (graceful) return true;
  try { child.kill("SIGKILL"); } catch (error) { if (error.code !== "ESRCH") throw error; }
  await waitFor(exited, 1000, undefined);
  return true;
}

function waitFor(promise, milliseconds, timeoutValue) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => resolve(timeoutValue), milliseconds);
    promise.then((value) => { clearTimeout(timer); resolve(value); }, (error) => { clearTimeout(timer); reject(error); });
  });
}

export function isManagedProcess(pid) { return managedChildren.has(pid); }

export async function fetchHealth(instance) {
  try {
    const response = await fetch(`http://${instance.host}:${instance.port}/health`, { signal: AbortSignal.timeout(500) });
    if (!response.ok) return null;
    const body = await response.json();
    return body && typeof body === "object" && body.status === "ok" ? body : null;
  } catch { return null; }
}
