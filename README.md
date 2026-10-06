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

Developer-facing capability commands are also available after a capability is
published in Desktop:

```bash
modelctl capabilities list
modelctl capabilities schema refund-check
modelctl capabilities invoke refund-check --json request.json
modelctl capabilities batch refund-check --json batch.json
```

客户业务系统接入请先阅读
[`docs/customer-integration.md`](docs/customer-integration.md)。它按本地开发、随桌面产品交付、企业内网和批处理四种场景说明部署边界、调用流程、版本策略和 REST/SDK 接口。

Set `MODELCTL_URL` and `MODELCTL_API_TOKEN` for an internal enterprise daemon;
the CLI forwards the bearer token on every control-plane request.

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
Concurrent pulls for the same model variant share one task and content-addressed
artifact download instead of downloading the same bytes twice.

Downloads use a content-addressed SHA-256 cache and resumable partial files.
Transient HTTP failures (including 429 and 5xx responses) retry with exponential
backoff; cancelling a pull or losing the daemon keeps the `.part` file so the
next pull can continue with an HTTP Range request. A model is marked installed
only after its declared size and SHA-256 have been verified.

The downloader follows the standard proxy variables used by Ollama:

```bash
export HTTPS_PROXY=http://127.0.0.1:7897
export HTTP_PROXY=http://127.0.0.1:7897
export NO_PROXY=127.0.0.1,localhost
modelctl pull convaiinnovations/laya --variant english
```

`MODELCTL_HTTPS_PROXY`, `MODELCTL_HTTP_PROXY`, and `MODELCTL_NO_PROXY` take
precedence when set. Proxy values are passed to the Python downloader without
being written to task errors or `doctor` output. Use
`MODELCTL_DOWNLOAD_RETRIES`, `MODELCTL_DOWNLOAD_CONNECT_TIMEOUT`, and
`MODELCTL_DOWNLOAD_READ_TIMEOUT` to tune retry and timeout values.

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

The sidebar opens model management, running instances, the structured decision
playground, the community registry, and settings. Settings can persist proxy
URLs and a default runtime profile under `$MODELCTL_DATA_DIR/settings.json`.
Environment variables take precedence over saved proxy values. Values that
contain proxy authentication are stored locally but shown without credentials.

The Community page lists entries from the reviewed `community/index.json` file,
including their source revision and declared permissions. Enabling an MCP item
adds it to the configuration snippet on the Settings page; use **Copy MCP
config** to paste it into the MCP client's own configuration. The snippet uses
the installed `modelctl-mcp` command and the local daemon URL. Community entries
are metadata in this release: browsing or enabling an entry does not download,
install, or execute third-party code.

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

Clients that want Ollama-style newline-delimited progress can send the same
pull body to `POST /v1/pulls/stream`. Each line reports `queued`,
`downloading` with `completed` and `total`, then `success`, `error`, or
`cancelled`. Repeating a pull for the same variant joins the existing task.

MCP and Skill integrations should call this API instead of reading the model
store or starting adapters directly. See [ADAPTER_PROTOCOL.md](ADAPTER_PROTOCOL.md)
and [LAYA_ADAPTER.md](LAYA_ADAPTER.md).

Local administration endpoints include `GET/PUT /v1/settings`,
`GET /v1/community`, and `GET/PUT /v1/mcp/config`. See
[API_CONTRACT.md](API_CONTRACT.md) for request and response shapes.

For MCP clients, configure the stdio command `modelctl-mcp` (or
`node bin/modelctl-mcp.js`). It keeps the raw `laya_system_one` tool for
advanced users and exposes every published capability as a typed
`modelctl_capability_<id>` tool. Set `MODELCTL_URL` and, for a remote daemon,
`MODELCTL_API_TOKEN` in the MCP process environment. The example Skill contract is in
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

## Desktop Packages

The Go/Fyne client source is under `desktop/`. macOS Apple Silicon builds use
`desktop/release-macos.sh`; Windows x64 and Linux x86_64 packages use
`desktop/release-cross.sh` with Docker and `fyne-cross` installed. Packages and
checksums are written under `desktop/dist/release/`. See
[`desktop/README.md`](desktop/README.md) for platform prerequisites and
installation steps. These packages contain the client only; install the
Modelctl daemon/runtime and model separately.

Application planning for finance research, embodied decision support, and
LLM/VLM/vLLM preprocessing is documented in
[`docs/plans/laya-jev-real-world-applications.md`](docs/plans/laya-jev-real-world-applications.md).

The English checkpoint has been downloaded, hash verified, and used for a
real CPU `system_one` request in a Windows development environment. Linux and
macOS target-machine validation remains open; see
[DEVELOPMENT_PREREQUISITES.md](DEVELOPMENT_PREREQUISITES.md).

## Native Go Desktop Client

The repository also contains a native macOS MVP under `desktop/`. It uses
Fyne for the desktop window and talks to the existing local daemon; it does not
open the Web UI or start a second model runtime.

```bash
cd desktop
./build-macos.sh
open dist/Modelctl.app
```

To create a versioned macOS arm64 distribution package:

```bash
cd desktop
./release-macos.sh
```

This produces a DMG, ZIP, and SHA-256 file under `desktop/dist/`. The current
local build is ad-hoc signed because a Developer ID certificate is not stored
on the development machine; public distribution requires signing with
`MODELCTL_SIGNING_IDENTITY` and Apple notarization. The app is a client and
requires the Modelctl daemon, runtime, and selected model to be installed
separately.

Set `MODELCTL_URL` when the daemon is not using the default
`http://127.0.0.1:11435`. The desktop MVP supports model variants, verified
pulls, MPS/CPU instance lifecycle, and a structured Laya decision playground.

## Application integration modes

Modelctl supports two deployment modes for application teams:

- Local developer mode: keep the daemon on `127.0.0.1:11435`; a desktop app or local service calls the capability API without network exposure.
- Enterprise server mode: bind explicitly to an internal interface with `MODELCTL_HOST` and set `MODELCTL_API_TOKEN`. Remote binding is rejected unless a token is configured. Clients send `Authorization: Bearer <token>`.

Example server launch:

```bash
MODELCTL_HOST=0.0.0.0 MODELCTL_API_TOKEN=replace-me node bin/modelctl.js daemon
```

The desktop console remains the administration surface. Business systems call
`/v1/capabilities/{id}/invoke`, while model files and adapter ports stay private
inside the Modelctl runtime.

Once the selected variant has been downloaded, the first capability request
starts its matching runtime automatically. The application only needs the
capability URL and its input contract; it does not need to start an instance or
manage an adapter port. A request for an undownloaded variant returns
`MODEL_NOT_INSTALLED` so deployment automation can prepare the exact model.
