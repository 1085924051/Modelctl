import readline from "node:readline";

function daemonBase() { return (process.env.MODELCTL_URL || "http://127.0.0.1:11435").replace(/\/$/, ""); }

function requestHeaders(json = false) {
  const headers = { accept: "application/json" };
  if (json) headers["content-type"] = "application/json";
  const token = process.env.MODELCTL_API_INVOKE_TOKEN || process.env.MODELCTL_API_TOKEN;
  if (token) headers.authorization = `Bearer ${token}`;
  return headers;
}

async function daemonRequest(path, options = {}) {
  const response = await fetch(`${daemonBase()}${path}`, { ...options, headers: { ...requestHeaders(!!options.body), ...(options.headers || {}) }, signal: AbortSignal.timeout(120000) });
  const text = await response.text();
  let value;
  try { value = JSON.parse(text); } catch { value = text; }
  if (!response.ok) throw new Error(value?.error?.message || `Modelctl returned HTTP ${response.status}`);
  return value;
}

export function startMcp(input = process.stdin, output = process.stdout) {
  const rl = readline.createInterface({ input, crlfDelay: Infinity });
  rl.on("line", async (line) => {
    if (!line.trim()) return;
    let message;
    try { message = JSON.parse(line); } catch { return; }
    if (message.method?.startsWith("notifications/")) return;
    const response = await handle(message).catch((error) => ({ jsonrpc: "2.0", id: message.id ?? null, error: { code: -32603, message: error.message } }));
    output.write(`${JSON.stringify(response)}\n`);
  });
  return rl;
}

export async function handle(message) {
  const id = message.id ?? null;
  if (message.jsonrpc !== "2.0" || !message.method) return { jsonrpc: "2.0", id, error: { code: -32600, message: "invalid JSON-RPC request" } };
  if (message.method === "initialize") return { jsonrpc: "2.0", id, result: { protocolVersion: message.params?.protocolVersion || "2024-11-05", capabilities: { tools: {} }, serverInfo: { name: "modelctl", version: "0.1.0" } } };
  if (message.method === "ping") return { jsonrpc: "2.0", id, result: {} };
  if (message.method === "tools/list") return { jsonrpc: "2.0", id, result: { tools: await listTools() } };
  if (message.method === "tools/call") return callTool(id, message.params || {});
  return { jsonrpc: "2.0", id, error: { code: -32601, message: `method not found: ${message.method}` } };
}

async function listTools() {
  const tools = [{ name: "laya_system_one", description: "Run Laya structured System One decisions through modelctl.", inputSchema: { type: "object", required: ["state", "questions"], properties: { state: {}, questions: { type: "object" }, instance_id: { type: "string" } } } }];
  const capabilities = (await daemonRequest("/v1/capabilities")).items || [];
  const dynamic = await Promise.all(capabilities.map(async (capability) => {
    let inputSchema = { type: "object", required: ["input"], properties: { input: { type: "object" }, metadata: { type: "object" }, version: { type: "string" } } };
    try {
      const schema = await daemonRequest(`/v1/capabilities/${encodeURIComponent(capability.id)}/schema?version=${encodeURIComponent(capability.version)}`);
      if (schema.request_schema) inputSchema = { ...schema.request_schema, properties: { ...schema.request_schema.properties, version: { type: "string", description: "Optional capability version override." } } };
    } catch { /* The capability remains discoverable with the safe fallback schema. */ }
    return { name: capabilityToolName(capability.id), description: capability.description || capability.name || `Invoke ${capability.id}`, inputSchema };
  }));
  return tools.concat(dynamic);
}

function capabilityToolName(id) { return `modelctl_capability_${id}`; }

async function callTool(id, params) {
  if (params.name !== "laya_system_one") return callCapabilityTool(id, params);
  const args = params.arguments || {};
  if (args.state === undefined || typeof args.questions !== "object" || args.questions === null) return { jsonrpc: "2.0", id, error: { code: -32602, message: "state and questions are required" } };
  const path = args.instance_id ? `/v1/instances/${encodeURIComponent(args.instance_id)}/operations/system_one` : "/v1/systemone";
  const value = await daemonRequest(path, { method: "POST", headers: requestHeaders(true), body: JSON.stringify({ state: args.state, questions: args.questions }) }).catch((error) => ({ __error: error.message }));
  if (value.__error) return { jsonrpc: "2.0", id, result: { isError: true, content: [{ type: "text", text: value.__error }] } };
  return { jsonrpc: "2.0", id, result: { content: [{ type: "text", text: JSON.stringify(value) }], structuredContent: value } };
}

async function callCapabilityTool(id, params) {
  const toolName = params.name || "";
  if (!toolName.startsWith("modelctl_capability_")) return { jsonrpc: "2.0", id, error: { code: -32602, message: "unknown tool" } };
  const capabilityID = toolName.slice("modelctl_capability_".length);
  const args = params.arguments || {};
  if (!args.input || typeof args.input !== "object" || Array.isArray(args.input)) return { jsonrpc: "2.0", id, error: { code: -32602, message: "capability tools require an input object" } };
  const capabilities = await daemonRequest("/v1/capabilities").catch(() => ({ items: [] }));
  if (!(capabilities.items || []).some((capability) => capability.id === capabilityID)) return { jsonrpc: "2.0", id, error: { code: -32602, message: "unknown capability tool" } };
  const version = typeof args.version === "string" && args.version ? `?version=${encodeURIComponent(args.version)}` : "";
  const { version: _version, ...body } = args;
  const value = await daemonRequest(`/v1/capabilities/${encodeURIComponent(capabilityID)}/invoke${version}`, { method: "POST", headers: requestHeaders(true), body: JSON.stringify(body) }).catch((error) => ({ __error: error.message }));
  if (value.__error) return { jsonrpc: "2.0", id, result: { isError: true, content: [{ type: "text", text: value.__error }] } };
  return { jsonrpc: "2.0", id, result: { content: [{ type: "text", text: JSON.stringify(value) }], structuredContent: value } };
}
