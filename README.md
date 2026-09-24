# modelctl

`modelctl` is a local model deployment control plane for Linux and macOS. It is
designed around the Ollama workflow (`catalog -> pull -> run -> invoke`) while
keeping model runtimes in separate adapter processes.

The current implementation is a Node.js 20+ control-plane MVP. Laya runs in a
Python adapter and is exposed through its native `/v1/systemone` protocol.

## Quick start

```bash
bash scripts/setup-laya.sh
export MODELCTL_PYTHON="$HOME/.modelctl/venv/bin/python"
npm start
# in another terminal
node bin/modelctl.js doctor
node bin/modelctl.js catalog validate
node bin/modelctl.js catalog list
node bin/modelctl.js inspect convaiinnovations/laya --variant english
node bin/modelctl.js pull convaiinnovations/laya --variant english
node bin/modelctl.js tasks
node bin/modelctl.js cancel <task-id>
node bin/modelctl.js run convaiinnovations/laya --variant english --profile cpu
node bin/modelctl.js ps
node bin/modelctl.js invoke <instance-id> system_one --json request.json
node bin/modelctl.js stop <instance-id>
node bin/modelctl.js remove convaiinnovations/laya --variant english --purge
```

`doctor` reports the detected Node/Python runtimes, host platform, catalog
validity, memory, and profiles available on that host. It exits nonzero when a
required runtime is missing.

`remove` stops no processes automatically. It refuses to remove a variant still
used by an active instance; stop that instance first. `--purge` also removes
unshared content-addressed artifacts.

`pull` downloads and SHA-256 verifies the pinned Laya revision. The English
variant is about 843 MB; the multilingual variant is smaller in weights but has
a larger tokenizer. Set `MODELCTL_DATA_DIR` to choose the local model store.

## Laya runtime

Install Python 3.10+, PyTorch, Laya, and the serving dependencies in the target
Linux or macOS environment. The helper creates a private environment under
`$MODELCTL_DATA_DIR`:

```bash
bash scripts/setup-laya.sh
export MODELCTL_PYTHON="$HOME/.modelctl/venv/bin/python"
```

If you set a custom `MODELCTL_DATA_DIR`, use its `venv/bin/python` path instead.

The adapter only loads the verified local directory supplied by `modelctl`; it
does not download floating revisions. Set `MODELCTL_PYTHON` when the Python
executable is not `python3`.

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
