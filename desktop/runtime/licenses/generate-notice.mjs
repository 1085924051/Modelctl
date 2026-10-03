#!/usr/bin/env node
import fs from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const output = path.resolve(process.argv[2] || "desktop/dist/runtime/licenses");
await fs.mkdir(output, { recursive: true });
const entries = [
  { name: "Modelctl first-party control plane", version: "0.1.0", license: "Application source; see repository license policy" },
  { name: "Node.js", version: process.env.MODELCTL_NODE_VERSION || "22.23.3", license: "Node.js license and bundled notices" },
  { name: "Python", version: process.env.MODELCTL_PYTHON_VERSION || "3.11.17", license: "Python Software Foundation License" },
  { name: "Laya", version: "0.3.18", license: "Apache-2.0 metadata from upstream package" },
  { name: "PyTorch", version: process.env.MODELCTL_TORCH_VERSION || "2.14.0", license: "BSD-style license and bundled notices from upstream wheel" },
  { name: "Fyne", version: "2.6.1", license: "BSD 3-Clause" },
];
await fs.writeFile(path.join(output, "index.json"), JSON.stringify({ generated_at: new Date().toISOString(), entries }, null, 2) + "\n");
await fs.writeFile(path.join(output, "NOTICE.txt"), entries.map((entry) => `${entry.name} ${entry.version}\n${entry.license}\n`).join("\n"));
console.log(`wrote ${entries.length} runtime license entries to ${output}`);
