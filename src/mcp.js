import readline from "node:readline";

const base = process.env.MODELCTL_URL || "http://127.0.0.1:11435";

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

async function handle(message) {
  const id = message.id ?? null;
  if (message.jsonrpc !== "2.0" || !message.method) return { jsonrpc: "2.0", id, error: { code: -32600, message: "invalid JSON-RPC request" } };
  if (message.method === "initialize") return { jsonrpc: "2.0", id, result: { protocolVersion: message.params?.protocolVersion || "2024-11-05", capabilities: { tools: {} }, serverInfo: { name: "modelctl", version: "0.1.0" } } };
  if (message.method === "ping") return { jsonrpc: "2.0", id, result: {} };
  if (message.method === "tools/list") return { jsonrpc: "2.0", id, result: { tools: [{ name: "laya_system_one", description: "Run Laya structured System One decisions through modelctl.", inputSchema: { type: "object", required: ["state", "questions"], properties: { state: {}, questions: { type: "object" }, instance_id: { type: "string" } } } }] } };
  if (message.method === "tools/call") return callTool(id, message.params || {});
  return { jsonrpc: "2.0", id, error: { code: -32601, message: `method not found: ${message.method}` } };
}

async function callTool(id, params) {
  if (params.name !== "laya_system_one") return { jsonrpc: "2.0", id, error: { code: -32602, message: "unknown tool" } };
  const args = params.arguments || {};
  if (args.state === undefined || typeof args.questions !== "object" || args.questions === null) return { jsonrpc: "2.0", id, error: { code: -32602, message: "state and questions are required" } };
  const path = args.instance_id ? `/v1/instances/${encodeURIComponent(args.instance_id)}/operations/system_one` : "/v1/systemone";
  const response = await fetch(`${base}${path}`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ state: args.state, questions: args.questions }), signal: AbortSignal.timeout(120000) });
  const text = await response.text();
  let value; try { value = JSON.parse(text); } catch { value = text; }
  if (!response.ok) return { jsonrpc: "2.0", id, result: { isError: true, content: [{ type: "text", text: JSON.stringify(value) }] } };
  return { jsonrpc: "2.0", id, result: { content: [{ type: "text", text: JSON.stringify(value) }], structuredContent: value } };
}
