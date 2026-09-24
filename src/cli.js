import { createServer, shutdownInstances } from "./server.js";
import { listCatalog, readState, dataRoot } from "./core.js";
import { spawn, spawnSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import os from "node:os";

const cliPath = fileURLToPath(import.meta.url);
const binPath = path.resolve(path.dirname(cliPath), "..", "bin", "modelctl.js");

export async function main(argv = process.argv.slice(2)) {
  const [command, subcommand, ...rest] = argv;
  if (!command || command === "help" || command === "--help") return usage();
  if (command === "daemon") {
    const server = await createServer();
    const address = server.address();
    const host = process.env.MODELCTL_HOST || "127.0.0.1";
    console.log(`modelctl listening on http://${host}:${typeof address === "object" ? address.port : address}`);
    let stopping = false;
    const shutdown = async () => {
      if (stopping) return;
      stopping = true;
      await shutdownInstances().catch(() => undefined);
      await new Promise((resolve) => server.close(resolve));
    };
    process.once("SIGINT", () => shutdown().finally(() => process.exit(0)));
    process.once("SIGTERM", () => shutdown().finally(() => process.exit(0)));
    return;
  }
  if (command === "catalog" && subcommand === "validate") { const ms = await listCatalog(); for (const m of ms) console.log(m._invalid ? `INVALID ${m._invalid}: ${m.error}` : `OK ${m.id}@${m.version}`); if (ms.some((x) => x._invalid)) process.exitCode = 1; return; }
  const base = process.env.MODELCTL_URL || "http://127.0.0.1:11435";
  if (command === "data") { console.log(dataRoot()); return; }
  if (command === "doctor") return doctor();
  if (command === "setup") return setup();
  await ensureDaemon(base);
  if (command === "catalog" && subcommand === "list") return print(await get(`${base}/v1/models`));
  if (command === "inspect") {
    const model = subcommand && !subcommand.startsWith("--") ? subcommand : rest[0];
    if (!model) return print(await get(`${base}/v1/models`));
    const version = option(rest, "version");
    const suffix = version ? `?version=${encodeURIComponent(version)}` : "";
    const detail = await get(`${base}/v1/models/${encodeURIComponent(model)}${suffix}`);
    const variant = option(rest, "variant");
    if (variant) detail.variants = detail.variants.filter((item) => item.id === variant);
    return print(detail);
  }
  if (command === "ps") return print(await get(`${base}/v1/instances`));
  if (command === "tasks") return print(await get(`${base}/v1/tasks`));
  if (command === "cancel") { const task = subcommand; if (!task) throw new Error("cancel requires a task id"); return print(await post(`${base}/v1/tasks/${encodeURIComponent(task)}/cancel`, {})); }
  if (command === "pull") { const model = subcommand; if (!model || model.startsWith("--")) throw new Error("pull requires a model id"); const variant = option(rest, "variant") || "english"; const version = option(rest, "version"); const revision = option(rest, "revision"); return pullFromCatalog(base, { model_id: model, ...(version ? { version } : {}), ...(revision ? { revision } : {}), variant }, rest.includes("--detach")); }
  if (command === "run") {
    const model = subcommand;
    if (!model || model.startsWith("--")) throw new Error("run requires a model id");
    const variant = option(rest, "variant") || "english";
    const profile = option(rest, "profile") || "auto";
    const version = option(rest, "version");
    const revision = option(rest, "revision");
    const modelInfo = await get(`${base}/v1/models/${encodeURIComponent(model)}${version ? `?version=${encodeURIComponent(version)}` : ""}`);
    const selected = modelInfo.variants.find((item) => item.id === variant);
    if (!selected) throw new Error(`unknown variant: ${variant}`);
    if (!selected.installed) await pullFromCatalog(base, { model_id: model, ...(version ? { version } : {}), ...(revision ? { revision } : {}), variant }, false);
    return print(await post(`${base}/v1/instances`, { model_id: model, ...(version ? { version } : {}), ...(revision ? { revision } : {}), variant, profile, default: true }));
  }
  if (command === "stop") { const instance = subcommand; if (!instance) throw new Error("stop requires an instance id"); return print(await del(`${base}/v1/instances/${instance}`)); }
  if (command === "remove") { const model = subcommand; if (!model) throw new Error("remove requires a model id"); const variant = option(rest, "variant"); const purge = rest.includes("--purge"); const query = new URLSearchParams({ ...(variant ? { variant } : {}), ...(purge ? { purge: "true" } : {}) }); return print(await del(`${base}/v1/models/${encodeURIComponent(model)}${query.toString() ? `?${query}` : ""}`)); }
  if (command === "logs") { const instance = subcommand; if (!instance) throw new Error("logs requires an instance id"); const details = await get(`${base}/v1/instances/${encodeURIComponent(instance)}`); console.log(await (await import("node:fs/promises")).readFile(details.log_path, "utf8").catch(() => "")); return; }
  if (command === "invoke") { const instance = subcommand; const operation = rest[0] || "system_one"; const file = option(rest, "json"); if (!instance || !file) throw new Error("invoke requires <instance-id> <operation> --json <file>"); const body = JSON.parse(await (await import("node:fs/promises")).readFile(file, "utf8")); return print(await post(`${base}/v1/instances/${instance}/operations/${operation}`, body)); }
  throw new Error(`unknown command: ${command}`);
}

function option(args, name) { const i = args.indexOf(`--${name}`); return i >= 0 ? args[i + 1] : undefined; }
async function get(url) { const r = await fetch(url); return parse(r); }
async function post(url, body) { const r = await fetch(url, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) }); return parse(r); }
async function del(url) { const r = await fetch(url, { method: "DELETE" }); return parse(r); }
async function parse(r) { const text = await r.text(); let body; try { body = JSON.parse(text); } catch { body = text; } if (!r.ok) throw new Error(body?.error?.message || `${r.status} request failed`); return body; }
function print(value) { console.log(JSON.stringify(value, null, 2)); }
async function ensureDaemon(base) {
  if (await daemonReady(base)) return;
  const target = new URL(base);
  if (!["127.0.0.1", "localhost", "::1", "[::1]"].includes(target.hostname)) return;
  const child = spawn(process.execPath, [binPath, "daemon"], {
    cwd: path.resolve(path.dirname(cliPath), ".."),
    env: { ...process.env },
    detached: true,
    stdio: "ignore",
  });
  child.unref();
  for (let attempt = 0; attempt < 60; attempt++) {
    if (await daemonReady(base)) return;
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error("could not start the local modelctl service; run `modelctl daemon` to inspect startup errors");
}

async function daemonReady(base) {
  try {
    const response = await fetch(`${base}/health`, { signal: AbortSignal.timeout(500) });
    return response.ok;
  } catch { return false; }
}

async function pullFromCatalog(base, body, detach) {
  const task = await post(`${base}/v1/pulls`, body);
  if (detach) return print(task);
  let lastPercent = -1;
  while (true) {
    const current = await get(`${base}/v1/tasks/${encodeURIComponent(task.task_id)}`);
    const done = current.progress?.bytes_done || 0;
    const total = current.progress?.bytes_total || 0;
    const percent = total ? Math.floor(done * 100 / total) : 0;
    if (percent !== lastPercent) {
      process.stdout.write(`\rPulling ${body.model_id}:${body.variant} ${percent}%`);
      lastPercent = percent;
    }
    if (["succeeded", "failed", "cancelled"].includes(current.status)) {
      process.stdout.write("\n");
      if (current.status !== "succeeded") throw new Error(current.error?.message || `pull ${current.status}`);
      print(current);
      return current;
    }
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
}

function setup() {
  const script = path.resolve(path.dirname(cliPath), "..", "scripts", process.platform === "win32" ? "setup-laya.ps1" : "setup-laya.sh");
  const dataDir = dataRoot();
  const env = { ...process.env, MODELCTL_DATA_DIR: dataDir };
  const result = process.platform === "win32"
    ? spawnSync("powershell", ["-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script], { stdio: "inherit", env })
    : spawnSync("bash", [script], { stdio: "inherit", env });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exitCode = result.status || 1;
}

async function doctor() {
  const nodeVersion = process.versions.node;
  const managedPython = process.platform === "win32" ? path.join(dataRoot(), "venv", "Scripts", "python.exe") : path.join(dataRoot(), "venv", "bin", "python");
  const candidates = process.env.MODELCTL_PYTHON ? [process.env.MODELCTL_PYTHON] : [managedPython, ...(process.platform === "win32" ? ["python"] : ["python3", "python"])];
  let python = null;
  for (const candidate of candidates) {
    const result = spawnSync(candidate, ["--version"], { encoding: "utf8" });
    if (!result.error && result.status === 0) {
      const version = `${result.stdout || result.stderr}`.trim();
      const match = version.match(/Python\s+(\d+)\.(\d+)/i);
      python = { command: candidate, version, supported: !!match && (Number(match[1]) > 3 || (Number(match[1]) === 3 && Number(match[2]) >= 10)) };
      break;
    }
  }
  const manifests = await listCatalog();
  const platform = `${process.platform === "darwin" ? "macos" : process.platform}-${process.arch === "x64" ? "x86_64" : process.arch}`;
  const profiles = manifests.filter((m) => !m._invalid).flatMap((m) => (m.profiles || []).filter((profile) => !profile.platforms || profile.platforms.includes(platform)).map((profile) => `${m.id}:${profile.id}`));
  const report = { node: { version: nodeVersion, supported: Number(nodeVersion.split(".")[0]) >= 20 }, python, platform, platform_supported: platform.startsWith("linux-") || platform.startsWith("macos-"), data_dir: dataRoot(), catalog_valid: manifests.every((m) => !m._invalid), supported_profiles: profiles, memory_bytes: os.totalmem() };
  print(report);
  if (!report.node.supported || !report.python?.supported || !report.platform_supported || !report.catalog_valid) process.exitCode = 1;
}
function usage() { console.log(`modelctl\n\n  setup                  install the private Laya runtime\n  doctor                 check local runtime prerequisites\n  catalog validate|list  validate or list model catalog\n  inspect                list models and install state\n  pull <model> [--variant v] [--detach]\n  run <model> [--variant v] [--profile p]\n  ps | tasks | cancel <task-id> | logs <id> | stop <id>\n  remove <model> [--variant v] [--purge]\n  invoke <instance> <operation> --json file\n  daemon                 start local API in the foreground\n  data                   show data directory\n\n  modelctl-mcp           start MCP stdio proxy`); }
