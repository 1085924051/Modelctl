import test from "node:test";
import assert from "node:assert/strict";
import fsp from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";

test("runtime builder refuses to create a package without real runtime inputs", async () => {
  const output = await fsp.mkdtemp(path.join(os.tmpdir(), "modelctl-runtime-test-"));
  const builder = path.resolve("desktop/runtime/build-runtime.mjs");
  const result = await run(process.execPath, [builder, "--target", "darwin-arm64", "--output", output]);
  assert.notEqual(result.code, 0);
  assert.match(result.stderr, /real runtime inputs are required/);
});

function run(command, args) {
  return new Promise((resolve) => {
    const child = spawn(command, args, { stdio: ["ignore", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk) => { stdout += chunk; });
    child.stderr.on("data", (chunk) => { stderr += chunk; });
    child.on("close", (code) => resolve({ code, stdout, stderr }));
  });
}
