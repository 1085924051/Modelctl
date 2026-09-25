# modelctl

`modelctl` is a local model deployment control plane for Linux and macOS. It is
designed around the Ollama workflow (`catalog -> pull -> run -> invoke`) while
keeping model runtimes in separate adapter processes.

The current implementation is a Node.js 20+ control-plane MVP. Laya runs in a
Python adapter and is exposed through its native `/v1/systemone` protocol.

## Quick start

```bash
npm install -g .
modelctl setup
modelctl doctor
modelctl catalog validate
modelctl pull convaiinnovations/laya --variant english
modelctl run convaiinnovations/laya --variant english --profile cpu
modelctl ps
modelctl invoke <instance-id> system_one --json request.json
modelctl stop <instance-id>
modelctl remove convaiinnovations/laya --variant english --purge
```

`setup` is required once per machine. After it completes, modelctl discovers
the private Python environment automatically, starts the local daemon on demand,
waits for `pull` to finish, and pulls a missing variant automatically before
`run`. Use `--detach` on `pull` when you want to poll the task yourself.

`doctor` reports the detected Node/Python runtimes, host platform, catalog
validity, memory, and profiles available on that host. It exits nonzero when a
required runtime is missing.

`remove` stops no processes automatically. It refuses to remove a variant still
used by an active instance; stop that instance first. `--purge` also removes
unshared content-addressed artifacts.

`pull` downloads and SHA-256 verifies the pinned Laya revision. The English
variant is about 843 MB; the multilingual variant is smaller in weights but has
a larger tokenizer. Set `MODELCTL_DATA_DIR` to choose the local model store.

## Local Web UI

After starting the service (any CLI command starts it automatically), open
`http://127.0.0.1:11435/` in a browser. The page provides:

- model and variant download with live progress;
- CPU/MPS/CUDA profile selection where the host supports it;
- instance start, stop, device and health status;
- a Laya judgment playground for `noul`, `choice`, and `score` questions;
- raw JSON output for debugging and API integration.

The Web UI is served by the local daemon and uses only loopback API calls. It
does not add a separate frontend server or send model inputs to a third party.

## Laya runtime

Install Python 3.10+, PyTorch, Laya, and the serving dependencies in the target
Linux or macOS environment. The helper creates a private environment under
`$MODELCTL_DATA_DIR`:

```bash
bash scripts/setup-laya.sh
```

If you set a custom `MODELCTL_DATA_DIR`, modelctl looks for its managed
environment under `$MODELCTL_DATA_DIR/venv`.

The adapter only loads the verified local directory supplied by `modelctl`; it
does not download floating revisions. Set `MODELCTL_PYTHON` only when you want
to override the managed environment.

On Windows development machines, use `scripts/setup-laya.ps1`; production
support remains Linux/macOS.

## HTTP API

The daemon listens on `127.0.0.1:11435` by default. It provides health,
catalog, pull-task, instance lifecycle, generic operation, and Jev-compatible
`POST /v1/systemone` endpoints. See [API_CONTRACT.md](API_CONTRACT.md).

MCP and Skill integrations should call this API instead of reading the model
store or starting adapters directly. See [ADAPTER_PROTOCOL.md](ADAPTER_PROTOCOL.md)
and [LAYA_ADAPTER.md](LAYA_ADAPTER.md).

For MCP clients, configure the stdio command `modelctl-mcp` (or
`node bin/modelctl-mcp.js`). It exposes the `laya_system_one` tool and proxies
calls to the daemon. The example Skill contract is in
`skills/laya-system-one/SKILL.md`.

## Development

```bash
node --check src/core.js
node --check src/server.js
node --check src/cli.js
npm run catalog:validate
```

The catalog is pinned to Laya revision
`cf7c54c0586eede67d827dfaab8cd2d2007273e`; artifact metadata is in
`catalog/laya-0.3.18.json`.

The English checkpoint has been downloaded, hash verified, and used for a
real CPU `system_one` request in a Windows development environment. Linux and
macOS target-machine validation remains open; see
[DEVELOPMENT_PREREQUISITES.md](DEVELOPMENT_PREREQUISITES.md).
