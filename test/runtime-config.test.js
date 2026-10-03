import test from "node:test";
import assert from "node:assert/strict";
import os from "node:os";
import path from "node:path";
import fsp from "node:fs/promises";

test("packaged control plane resolves assets from MODELCTL_RUNTIME_ROOT", async () => {
  const runtimeRoot = await fsp.mkdtemp(path.join(os.tmpdir(), "modelctl-runtime-root-"));
  const previous = process.env.MODELCTL_RUNTIME_ROOT;
  process.env.MODELCTL_RUNTIME_ROOT = runtimeRoot;
  try {
    const core = await import(`../src/core.js?runtime-root=${encodeURIComponent(runtimeRoot)}`);
    assert.equal(core.rootDir, path.join(runtimeRoot, "control-plane"));
    assert.equal(core.catalogDir, path.join(runtimeRoot, "control-plane", "catalog"));
  } finally {
    if (previous === undefined) delete process.env.MODELCTL_RUNTIME_ROOT;
    else process.env.MODELCTL_RUNTIME_ROOT = previous;
    await fsp.rm(runtimeRoot, { recursive: true, force: true });
  }
});

test("packaged Python takes precedence over a managed development venv", async () => {
  const previous = process.env.MODELCTL_PYTHON;
  process.env.MODELCTL_PYTHON = "/bundled/runtime/python/bin/python";
  try {
    const core = await import(`../src/core.js?python-path=${Date.now()}`);
    assert.equal(core.pythonExecutable(), "/bundled/runtime/python/bin/python");
  } finally {
    if (previous === undefined) delete process.env.MODELCTL_PYTHON;
    else process.env.MODELCTL_PYTHON = previous;
  }
});
