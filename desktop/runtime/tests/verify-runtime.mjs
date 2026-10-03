#!/usr/bin/env node
import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const root = path.resolve(process.argv[2] || "");
if (!root) throw new Error("usage: verify-runtime.mjs <runtime-dir>");
const manifest = JSON.parse(await fs.readFile(path.join(root, "runtime-manifest.json"), "utf8"));
if (manifest.schema_version !== 1) throw new Error("unsupported runtime manifest schema");
if (!manifest.files || Object.keys(manifest.files).length < 3) throw new Error("runtime manifest has too few files");
for (const [relative, expected] of Object.entries(manifest.files)) {
  if (path.isAbsolute(relative) || relative.includes("..")) throw new Error(`unsafe runtime path: ${relative}`);
  const actual = crypto.createHash("sha256").update(await fs.readFile(path.join(root, relative))).digest("hex");
  if (actual !== expected) throw new Error(`hash mismatch: ${relative}`);
}
for (const required of [manifest.target === "windows-amd64" ? "node/node.exe" : "node/bin/node", "control-plane/bin/modelctl.js", manifest.target === "windows-amd64" ? "python/python.exe" : "python/bin/python"]) {
  if (!manifest.files[required]) throw new Error(`required runtime file is not listed: ${required}`);
}
console.log(`runtime OK: ${manifest.target} ${manifest.software_version} ${manifest.build_commit}`);
