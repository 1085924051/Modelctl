import test from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import { main } from "../src/cli.js";

test("capabilities CLI forwards the enterprise bearer token", async () => {
  let seenAuthorization = "";
  const server = http.createServer((req, res) => {
    seenAuthorization = req.headers.authorization || "";
    res.setHeader("content-type", "application/json");
    if (req.url === "/health") return res.end(JSON.stringify({ status: "ok" }));
    if (req.url === "/v1/capabilities") return res.end(JSON.stringify({ items: [{ id: "refund-check" }] }));
    res.statusCode = 404;
    return res.end(JSON.stringify({ error: { message: "not found" } }));
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  const oldURL = process.env.MODELCTL_URL;
  const oldToken = process.env.MODELCTL_API_TOKEN;
  process.env.MODELCTL_URL = `http://127.0.0.1:${address.port}`;
  process.env.MODELCTL_API_TOKEN = "cli-secret";
  try {
    await main(["capabilities", "list"]);
  } finally {
    if (oldURL === undefined) delete process.env.MODELCTL_URL; else process.env.MODELCTL_URL = oldURL;
    if (oldToken === undefined) delete process.env.MODELCTL_API_TOKEN; else process.env.MODELCTL_API_TOKEN = oldToken;
    await new Promise((resolve) => server.close(resolve));
  }
  assert.equal(seenAuthorization, "Bearer cli-secret");
});
