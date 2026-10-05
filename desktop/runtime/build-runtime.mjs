#!/usr/bin/env node
import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, "../..");
const args = new Map();
for (let i = 2; i < process.argv.length; i += 1) {
  if (!process.argv[i].startsWith("--")) continue;
  args.set(process.argv[i].slice(2), process.argv[i + 1]);
  i += 1;
}
const target = args.get("target");
const output = path.resolve(args.get("output") || path.join(repo, "desktop/dist/runtime", target || "unknown"));
const source = path.resolve(args.get("source") || repo);
const nodeInput = args.get("node-binary") || process.env.MODELCTL_NODE_BINARY;
const nodeRoot = args.get("node-root") || process.env.MODELCTL_NODE_ROOT;
const pythonInput = args.get("python-binary") || process.env.MODELCTL_PYTHON_BINARY;
const pythonRoot = args.get("python-root") || process.env.MODELCTL_PYTHON_ROOT;
const supported = new Set(["darwin-arm64", "windows-amd64", "linux-amd64"]);
if (!supported.has(target)) throw new Error(`--target must be one of ${[...supported].join(", ")}`);

await fs.rm(output, { recursive: true, force: true });
await fs.mkdir(output, { recursive: true });

const files = new Map();
const copy = async (relative, destination = relative) => {
  const from = path.join(source, relative);
  const to = path.join(output, destination);
  await fs.mkdir(path.dirname(to), { recursive: true });
  await fs.cp(from, to, { recursive: true });
};
const copyIfExists = async (relative, destination = relative) => {
  try { await copy(relative, destination); } catch (error) { if (error.code !== "ENOENT") throw error; }
};

for (const directory of ["src", "bin", "web", "catalog", "schemas", "community", "adapters", "scripts"]) await copy(directory, path.join("control-plane", directory));
await copy("package.json", "control-plane/package.json");
await copyIfExists("package-lock.json", "control-plane/package-lock.json");

const nodeName = target === "windows-amd64" ? "node/node.exe" : "node/bin/node";
const pythonName = target === "windows-amd64" ? "python/python.exe" : "python/bin/python";
if (!nodeInput || !nodeRoot || !pythonInput || !pythonRoot) throw new Error("real runtime inputs are required: --node-binary, --node-root, --python-binary, and --python-root");
await fs.mkdir(path.dirname(path.join(output, nodeName)), { recursive: true });
await fs.mkdir(path.dirname(path.join(output, pythonName)), { recursive: true });
await fs.cp(path.resolve(nodeRoot), path.join(output, "node"), { recursive: true, dereference: true });
const bundledNode = path.join(output, nodeName);
if (!(await fileExists(bundledNode))) throw new Error(`node root does not contain expected executable ${nodeName}`);
await fs.cp(path.resolve(pythonRoot), path.join(output, "python"), { recursive: true, dereference: true });
const bundledPython = path.join(output, pythonName);
if (target === "windows-amd64" && !(await fileExists(bundledPython))) {
  const venvPython = path.join(output, "python", "Scripts", "python.exe");
  if (await fileExists(venvPython)) await fs.copyFile(venvPython, bundledPython);
}
if (!(await fileExists(bundledPython))) throw new Error(`python root does not contain expected executable ${pythonName}`);
await fs.chmod(path.join(output, nodeName), 0o755).catch(() => undefined);
await fs.chmod(path.join(output, pythonName), 0o755).catch(() => undefined);

await fs.mkdir(path.join(output, "licenses"), { recursive: true });
await runNodeScript(path.join(here, "licenses", "generate-notice.mjs"), [path.join(output, "licenses")]);
const relativeFiles = await walk(output);
for (const relative of relativeFiles) {
  const digest = crypto.createHash("sha256").update(await fs.readFile(path.join(output, relative))).digest("hex");
  files.set(relative, digest);
}
const commit = (await runGit(["rev-parse", "HEAD"], source)).trim();
const manifest = {
  schema_version: 1,
  software_version: "0.1.0",
  build_commit: commit,
  target,
  node_version: "22.23.3",
  python_version: "3.11.17",
  laya_version: "0.3.18",
  torch_version: "2.14.0",
  files: Object.fromEntries(files),
  licenses: ["licenses/NOTICE.txt", "licenses/index.json"],
};
await fs.writeFile(path.join(output, "runtime-manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
console.log(JSON.stringify({ target, output, files: Object.keys(manifest.files).length, commit }, null, 2));

async function walk(directory, prefix = "") {
  const entries = await fs.readdir(directory, { withFileTypes: true });
  const result = [];
  for (const entry of entries) {
    const relative = path.join(prefix, entry.name);
    if (entry.isDirectory()) result.push(...await walk(path.join(directory, entry.name), relative));
    else if (relative !== "runtime-manifest.json") result.push(relative.split(path.sep).join("/"));
  }
  return result;
}

async function sha256(file) { return crypto.createHash("sha256").update(await fs.readFile(file)).digest("hex"); }
async function fileExists(file) { try { return (await fs.stat(file)).isFile(); } catch { return false; } }
async function runGit(command, cwd) {
  const { execFile } = await import("node:child_process");
  return new Promise((resolve, reject) => execFile("git", command, { cwd }, (error, stdout, stderr) => error ? reject(new Error(stderr || error.message)) : resolve(stdout)));
}
async function runNodeScript(script, scriptArgs) {
  const { execFile } = await import("node:child_process");
  return new Promise((resolve, reject) => execFile(process.execPath, [script, ...scriptArgs], (error) => error ? reject(error) : resolve()));
}
