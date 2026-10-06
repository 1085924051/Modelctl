import test, { after, beforeEach } from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import fsp from "node:fs/promises";
import { createServer } from "../src/server.js";
import { idFor, updateState } from "../src/core.js";

const oldDataDir = process.env.MODELCTL_DATA_DIR;
let root;
let server;
let base;
let adapter;

beforeEach(async () => {
  if (server) await new Promise((resolve) => server.close(resolve));
  if (adapter) await new Promise((resolve) => adapter.close(resolve));
  if (root) await fsp.rm(root, { recursive: true, force: true });
  root = await fsp.mkdtemp(path.join(os.tmpdir(), "modelctl-capability-"));
  process.env.MODELCTL_DATA_DIR = root;
  server = await createServer({ port: 0 });
  const address = server.address();
  base = `http://127.0.0.1:${address.port}`;
});

after(async () => {
  if (server) await new Promise((resolve) => server.close(resolve));
  if (adapter) await new Promise((resolve) => adapter.close(resolve));
  if (root) await fsp.rm(root, { recursive: true, force: true });
  if (oldDataDir === undefined) delete process.env.MODELCTL_DATA_DIR;
  else process.env.MODELCTL_DATA_DIR = oldDataDir;
});

function request(route, method = "GET", body) {
  return new Promise((resolve, reject) => {
    const payload = body === undefined ? null : Buffer.from(JSON.stringify(body));
    const req = http.request(new URL(route, base), { method, headers: payload ? { "content-type": "application/json", "content-length": payload.length } : {} }, (res) => {
      const chunks = [];
      res.on("data", (chunk) => chunks.push(chunk));
      res.on("end", () => resolve({ status: res.statusCode, body: JSON.parse(Buffer.concat(chunks).toString("utf8")) }));
    });
    req.on("error", reject);
    if (payload) req.write(payload);
    req.end();
  });
}

const capability = {
  id: "refund-check",
  name: "Refund check",
  description: "Decide whether the customer asks for a refund.",
  model: { model_id: "convaiinnovations/laya", variant: "english", profile: "auto" },
  input: { type: "text", field: "text" },
  questions: { refund: { type: "noul", instructions: "Does the customer ask for a refund?" } },
};

test("capabilities can be published and listed", async () => {
  const created = await request("/v1/capabilities", "POST", capability);
  assert.equal(created.status, 201);
  assert.equal(created.body.id, "refund-check");
  assert.equal(created.body.version, "1.0.0");
  const listed = await request("/v1/capabilities");
  assert.equal(listed.status, 200);
  assert.deepEqual(listed.body.items.map((item) => item.id), ["refund-check"]);
  const schema = await request("/v1/capabilities/refund-check/schema");
  assert.equal(schema.status, 200);
  assert.deepEqual(schema.body.request_schema.properties.input.required, ["text"]);
  assert.deepEqual(schema.body.response_schema.properties.output.required, ["refund"]);
  assert.deepEqual(schema.body.response_schema.properties.output.properties.refund.required, ["type", "value", "noul"]);
});

test("capability integration endpoint returns a versioned handoff contract", async () => {
  await request("/v1/capabilities", "POST", capability);
  const integration = await request("/v1/capabilities/refund-check/integration?version=1.0.0");
  assert.equal(integration.status, 200);
  assert.equal(integration.body.capability.id, "refund-check");
  assert.equal(integration.body.capability.version, "1.0.0");
  assert.match(integration.body.endpoint, /\/v1\/capabilities\/refund-check\/invoke\?version=1\.0\.0$/);
  assert.equal(integration.body.auth.required, false);
  assert.match(integration.body.examples.curl, /refund-check/);
  assert.doesNotMatch(integration.body.examples.curl, /\\n\+/);
  assert.match(integration.body.examples.python, /client\.invoke/);
  assert.match(integration.body.examples.javascript, /client\.invoke/);
  assert.deepEqual(integration.body.request_schema.properties.input.required, ["text"]);
});

test("capability OpenAPI endpoint describes invoke and batch routes", async () => {
  await request("/v1/capabilities", "POST", capability);
  const response = await request("/v1/capabilities/refund-check/openapi?version=1.0.0");
  assert.equal(response.status, 200);
  assert.equal(response.body.openapi, "3.1.0");
  assert.equal(response.body.info.version, "1.0.0");
  assert.ok(response.body.paths["/v1/capabilities/refund-check/invoke"].post);
  assert.ok(response.body.paths["/v1/capabilities/refund-check/batch"].post);
  assert.equal(response.body.components.schemas.refund_check_invoke_request.properties.input.required[0], "text");
});

test("capability schema exposes choice and score output contracts", async () => {
  await request("/v1/capabilities", "POST", {
    ...capability,
    id: "ticket-routing",
    questions: {
      team: { type: "choice", instructions: "Which team?", criteria: { billing: "Billing", support: "Support" } },
      priority: { type: "score", instructions: "How urgent?", criteria: ["low", "medium", "high"] },
    },
  });
  const schema = await request("/v1/capabilities/ticket-routing/schema");
  assert.deepEqual(schema.body.response_schema.properties.output.properties.team.properties.choice.enum, ["billing", "support"]);
  assert.deepEqual(schema.body.response_schema.properties.output.properties.priority.required, ["type", "value", "score"]);
});

test("run history is available as a replayable collection", async () => {
  const result = await request("/v1/runs");
  assert.equal(result.status, 200);
  assert.deepEqual(result.body.items, []);
});

test("capability versions can be published and activated explicitly", async () => {
  const first = await request("/v1/capabilities", "POST", capability);
  assert.equal(first.body.active, true);
  const second = await request("/v1/capabilities", "POST", { ...capability, version: "1.1.0", name: "Refund check v2", activate: false });
  assert.equal(second.status, 201);
  assert.equal(second.body.active, false);
  const current = await request("/v1/capabilities/refund-check");
  assert.equal(current.body.version, "1.0.0");
  const activated = await request("/v1/capabilities/refund-check/activate", "POST", { version: "1.1.0" });
  assert.equal(activated.status, 200);
  assert.equal(activated.body.version, "1.1.0");
  const selected = await request("/v1/capabilities/refund-check?version=1.0.0");
  assert.equal(selected.body.version, "1.0.0");
});

test("a published capability version cannot be overwritten", async () => {
  assert.equal((await request("/v1/capabilities", "POST", capability)).status, 201);
  const replacement = await request("/v1/capabilities", "POST", { ...capability, questions: { changed: { type: "noul", instructions: "A different contract?" } } });
  assert.equal(replacement.status, 409);
  assert.equal(replacement.body.error.code, "CAPABILITY_VERSION_EXISTS");
  const current = await request("/v1/capabilities/refund-check");
  assert.ok(current.body.questions.refund);
});

test("capability validation hides the raw protocol from invalid clients", async () => {
  const invalid = await request("/v1/capabilities", "POST", { id: "Bad ID", name: "x" });
  assert.equal(invalid.status, 422);
  assert.equal(invalid.body.error.code, "CAPABILITY_INVALID");
});

test("capability versions use semantic versioning", async () => {
  const invalid = await request("/v1/capabilities", "POST", { ...capability, version: "draft" });
  assert.equal(invalid.status, 422);
  assert.equal(invalid.body.error.code, "CAPABILITY_VERSION_INVALID");
});

test("structured capability input contracts reject invalid templates", async () => {
  const invalid = await request("/v1/capabilities", "POST", {
    ...capability,
    id: "invalid-input-contract",
    input: { type: "object", fields: { text: { type: "string" } }, template: "{{missing}}" },
  });
  assert.equal(invalid.status, 422);
  assert.equal(invalid.body.error.code, "CAPABILITY_INVALID");
});

test("capability invocation explains that its model must be downloaded", async () => {
  await request("/v1/capabilities", "POST", capability);
  const result = await request("/v1/capabilities/refund-check/invoke", "POST", { input: { text: "Please refund this order" } });
  assert.equal(result.status, 409);
  assert.equal(result.body.error.code, "MODEL_NOT_INSTALLED");
});

test("capability invocation returns normalized answers and preserves metadata", async () => {
  let adapterRequests = 0;
  adapter = http.createServer(async (req, res) => {
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    const received = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    if (adapterRequests === 0) {
      assert.equal(received.state.body, "Please refund this order");
      assert.ok(received.questions.refund);
      assert.ok(received.questions.team);
      assert.ok(received.questions.priority);
    }
    adapterRequests += 1;
    const response = {
      answers: {
        refund: { type: "noul", noul: 0.93, confidence: 0.91 },
        team: { type: "choice", choice: "billing", confidence: 0.88 },
        priority: { type: "score", score: 2, confidence: 0.77 },
      },
      usage: { latency_ms: 4 },
    };
    res.writeHead(200, { "content-type": "application/json" });
    res.end(JSON.stringify(response));
  });
  await new Promise((resolve) => adapter.listen(0, "127.0.0.1", resolve));
  const port = adapter.address().port;
  await request("/v1/capabilities", "POST", {
    ...capability,
    questions: {
      refund: { type: "noul", instructions: "Does the customer ask for a refund?" },
      team: { type: "choice", instructions: "Which team?", criteria: { billing: "Billing", support: "Support" } },
      priority: { type: "score", instructions: "How urgent?", criteria: ["low", "medium", "high"] },
    },
  });
  await updateState((state) => {
    state.models[idFor("convaiinnovations/laya", "0.3.18", "english")] = {
      id: "convaiinnovations/laya", version: "0.3.18", revision: "cf7c54c0586eede67d827dfaab8cd2d2007273e",
      variant: "english", path: root, installed_at: new Date().toISOString(),
    };
    state.instances["inst_fake"] = {
      id: "inst_fake", status: "ready", model: { id: "convaiinnovations/laya", version: "0.3.18", revision: "cf7c54c0586eede67d827dfaab8cd2d2007273e", variant: "english" },
      runtime: "laya-python@0.3.18", profile: "cpu", device: "cpu", host: "127.0.0.1", port,
      capabilities: ["system_one"], default: true,
    };
  });
  const result = await request("/v1/capabilities/refund-check/invoke", "POST", {
    input: { text: "Please refund this order" }, metadata: { ticket_id: "T-100", tenant_id: "acme" },
  });
  assert.equal(result.status, 200);
  assert.equal(result.body.output.refund.value, 0.93);
  assert.equal(result.body.output.team.value, "billing");
  assert.equal(result.body.output.priority.value, 2);
  assert.deepEqual(result.body.metadata, { ticket_id: "T-100", tenant_id: "acme" });
  assert.match(result.body.run_id, /^run_/);
  const history = await request("/v1/runs");
  assert.deepEqual(history.body.items[0].metadata, { ticket_id: "T-100", tenant_id: "acme" });
  assert.equal(history.body.items[0].response.answers.team.choice, "billing");
  const batch = await request("/v1/capabilities/refund-check/batch", "POST", {
    items: [
      { input: { text: "one" }, metadata: { ticket_id: "T-101" } },
      { input: { text: "two" }, metadata: { ticket_id: "T-102" } },
    ],
  });
  let task;
  for (let attempt = 0; attempt < 40; attempt += 1) {
    task = await request(batch.body.poll);
    if (["succeeded", "failed", "cancelled"].includes(task.body.status)) break;
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  assert.equal(task.body.status, "succeeded");
  assert.equal(task.body.results.length, 2);
  assert.equal(task.body.results[0].output.refund.value, 0.93);
  assert.deepEqual(task.body.results[1].metadata, { ticket_id: "T-102" });
});

test("Jev-compatible raw systemone requests remain available", async () => {
  let forwarded;
  adapter = http.createServer(async (req, res) => {
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    forwarded = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    res.writeHead(200, { "content-type": "application/json" });
    res.end(JSON.stringify({ answers: { topic: { type: "noul", noul: 0.71 } } }));
  });
  await new Promise((resolve) => adapter.listen(0, "127.0.0.1", resolve));
  await updateState((state) => {
    state.instances["inst_jev_compat"] = {
      id: "inst_jev_compat", status: "ready", model: { id: "convaiinnovations/laya", version: "0.3.18", revision: "cf7c54c0586eede67d827dfaab8cd2d2007273e", variant: "english" },
      runtime: "laya-python@0.3.18", profile: "cpu", device: "cpu", host: "127.0.0.1", port: adapter.address().port,
      capabilities: ["system_one"], default: true,
    };
  });
  const result = await request("/v1/systemone", "POST", {
    state: { body: "A customer wants to cancel the order." },
    questions: { topic: { type: "noul", instructions: "Is this about cancellation?" } },
  });
  assert.equal(result.status, 200);
  assert.equal(result.body.answers.topic.noul, 0.71);
  assert.equal(forwarded.questions.topic.type, "noul");
});

test("structured capability inputs validate fields and render the Laya prompt", async () => {
  let adapterInput;
  adapter = http.createServer(async (req, res) => {
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    adapterInput = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    res.writeHead(200, { "content-type": "application/json" });
    res.end(JSON.stringify({ answers: { refund: { type: "noul", noul: 0.8 } } }));
  });
  await new Promise((resolve) => adapter.listen(0, "127.0.0.1", resolve));
  await request("/v1/capabilities", "POST", {
    ...capability,
    id: "refund-structured",
    input: {
      type: "object",
      fields: {
        text: { type: "string", required: true, description: "Customer message" },
        order_id: { type: "string", required: false, description: "Order number" },
      },
      template: "Customer message: {{text}}\nOrder: {{order_id}}",
    },
  });
  const schema = await request("/v1/capabilities/refund-structured/schema");
  assert.deepEqual(schema.body.request_schema.properties.input.required, ["text"]);
  assert.equal(schema.body.request_schema.properties.input.properties.order_id.type, "string");
  const port = adapter.address().port;
  await updateState((state) => {
    state.models[idFor("convaiinnovations/laya", "0.3.18", "english")] = {
      id: "convaiinnovations/laya", version: "0.3.18", revision: "cf7c54c0586eede67d827dfaab8cd2d2007273e",
      variant: "english", path: root, installed_at: new Date().toISOString(),
    };
    state.instances["inst_structured"] = {
      id: "inst_structured", status: "ready", model: { id: "convaiinnovations/laya", version: "0.3.18", revision: "cf7c54c0586eede67d827dfaab8cd2d2007273e", variant: "english" },
      runtime: "laya-python@0.3.18", profile: "cpu", device: "cpu", host: "127.0.0.1", port,
      capabilities: ["system_one"], default: true,
    };
  });
  const result = await request("/v1/capabilities/refund-structured/invoke", "POST", { input: { text: "charged twice", order_id: "O-42" } });
  assert.equal(result.status, 200);
  assert.equal(adapterInput.state.body, "Customer message: charged twice\nOrder: O-42");
  const history = await request("/v1/runs");
  assert.deepEqual(history.body.items[0].input, { text: "charged twice", order_id: "O-42" });
});

test("first capability invocation automatically starts the matching adapter", async () => {
  const fakeDir = await fsp.mkdtemp(path.join(os.tmpdir(), "modelctl-fake-adapter-"));
  const adapterScript = path.join(fakeDir, "fake-adapter.py");
  await fsp.writeFile(adapterScript, `import json, sys\nfrom http.server import BaseHTTPRequestHandler, ThreadingHTTPServer\nport = int(sys.argv[sys.argv.index("--port") + 1])\nclass Handler(BaseHTTPRequestHandler):\n    def do_GET(self):\n        self.send_response(200); self.send_header("Content-Type", "application/json"); self.end_headers(); self.wfile.write(json.dumps({"status": "ok", "loaded": ["english"], "device": "cpu"}).encode())\n    def do_POST(self):\n        size = int(self.headers.get("Content-Length", "0")); self.rfile.read(size)\n        self.send_response(200); self.send_header("Content-Type", "application/json"); self.end_headers(); self.wfile.write(json.dumps({"answers": {"refund": {"type": "noul", "noul": 0.96}}}).encode())\n    def log_message(self, *args): pass\nThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()\n`, "utf8");
  const launcher = process.env.MODELCTL_PYTHON || (process.platform === "win32" ? "python" : "python3");
  const previousPython = process.env.MODELCTL_PYTHON;
  const previousAdapter = process.env.MODELCTL_LAYA_ADAPTER;
  process.env.MODELCTL_PYTHON = launcher;
  process.env.MODELCTL_LAYA_ADAPTER = adapterScript;
  try {
    await request("/v1/capabilities", "POST", capability);
    await updateState((state) => {
      state.models[idFor("convaiinnovations/laya", "0.3.18", "english")] = {
        id: "convaiinnovations/laya", version: "0.3.18", revision: "cf7c54c0586eede67d827dfaab8cd2d2007273e",
        variant: "english", path: root, installed_at: new Date().toISOString(),
      };
    });
    const result = await request("/v1/capabilities/refund-check/invoke", "POST", { input: { text: "Please refund this order" } });
    assert.equal(result.status, 200, JSON.stringify(result.body));
    assert.equal(result.body.output.refund.value, 0.96);
    const instances = await request("/v1/instances");
    const started = instances.body.items.find((instance) => instance.status === "ready" && instance.model.variant === "english");
    assert.ok(started, JSON.stringify(instances.body.items));
    const stopped = await request(`/v1/instances/${started.id}`, "DELETE");
    assert.equal(stopped.status, 200);
  } finally {
    if (previousPython === undefined) delete process.env.MODELCTL_PYTHON; else process.env.MODELCTL_PYTHON = previousPython;
    if (previousAdapter === undefined) delete process.env.MODELCTL_LAYA_ADAPTER; else process.env.MODELCTL_LAYA_ADAPTER = previousAdapter;
    await fsp.rm(fakeDir, { recursive: true, force: true });
  }
});

test("capability metadata must be an object", async () => {
  await request("/v1/capabilities", "POST", capability);
  const result = await request("/v1/capabilities/refund-check/invoke", "POST", { input: { text: "Please refund this order" }, metadata: ["T-100"] });
  assert.equal(result.status, 400);
  assert.equal(result.body.error.code, "INVALID_REQUEST");
});

test("capability batches return a task that records per-item errors", async () => {
  await request("/v1/capabilities", "POST", capability);
  const created = await request("/v1/capabilities/refund-check/batch", "POST", { items: [{ input: { text: "one" } }, { input: { text: "two" } }] });
  assert.equal(created.status, 202);
  let task;
  for (let attempt = 0; attempt < 30; attempt += 1) {
    task = await request(created.body.poll);
    if (["succeeded", "failed", "cancelled"].includes(task.body.status)) break;
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  assert.equal(task.body.status, "succeeded");
  assert.equal(task.body.capability_version, "1.0.0");
  assert.equal(task.body.progress.items_done, 2);
  assert.equal(task.body.errors.length, 2);
});

test("remote binding requires an explicit API token", async () => {
  const previous = process.env.MODELCTL_API_TOKEN;
  delete process.env.MODELCTL_API_TOKEN;
  await assert.rejects(() => createServer({ host: "0.0.0.0", port: 0 }), /MODELCTL_API_TOKEN/);
  if (previous === undefined) delete process.env.MODELCTL_API_TOKEN;
  else process.env.MODELCTL_API_TOKEN = previous;
});
