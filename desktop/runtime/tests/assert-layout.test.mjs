import assert from "node:assert/strict";
import { mkdir, mkdtemp, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";
import test from "node:test";

const script = path.resolve("desktop/runtime/assert-layout.sh");

function run(root, target) {
  return new Promise((resolve) => {
    const child = spawn("sh", [script, root, target], { stdio: ["ignore", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk) => { stdout += chunk; });
    child.stderr.on("data", (chunk) => { stderr += chunk; });
    child.on("close", (code) => resolve({ code, stdout, stderr }));
  });
}

const runLayoutTest = process.platform === "win32" ? test.skip : test;

runLayoutTest("runtime layout verifier accepts a complete Linux tree", async () => {
  const root = await mkdtemp(path.join(os.tmpdir(), "modelctl-layout-"));
  await mkdir(path.join(root, "node/bin"), { recursive: true });
  await mkdir(path.join(root, "python/bin"), { recursive: true });
  await mkdir(path.join(root, "control-plane/bin"), { recursive: true });
  await writeFile(path.join(root, "runtime-manifest.json"), "{}\n");
  await writeFile(path.join(root, "node/bin/node"), "node\n");
  await writeFile(path.join(root, "python/bin/python"), "python\n");
  await writeFile(path.join(root, "control-plane/bin/modelctl.js"), "console.log('ok')\n");
  const result = await run(root, "linux-amd64");
  assert.equal(result.code, 0, result.stderr);
  assert.match(result.stdout, /runtime layout OK/);
});

runLayoutTest("runtime layout verifier rejects a missing control plane", async () => {
  const root = await mkdtemp(path.join(os.tmpdir(), "modelctl-layout-"));
  await mkdir(path.join(root, "node/bin"), { recursive: true });
  await mkdir(path.join(root, "python/bin"), { recursive: true });
  await writeFile(path.join(root, "runtime-manifest.json"), "{}\n");
  await writeFile(path.join(root, "node/bin/node"), "node\n");
  await writeFile(path.join(root, "python/bin/python"), "python\n");
  const result = await run(root, "linux-amd64");
  assert.notEqual(result.code, 0);
  assert.match(result.stderr, /control-plane\/bin\/modelctl\.js/);
});
