# Modelctl Desktop

Native Go/Fyne macOS client for the local Modelctl daemon.

## Run

```bash
cd desktop
MODELCTL_URL=http://127.0.0.1:11435 go run .
```

## Build

```bash
cd desktop
./build-macos.sh
open dist/Modelctl.app
```

## Windows and Linux packages

Cross-building the GUI apps uses Docker and Fyne's pinned cross-build images.
Install `fyne-cross` once:

```bash
go install github.com/fyne-io/fyne-cross@v1.6.2
```

Then build Windows x64 and Linux x86_64 packages:

```bash
./release-cross.sh
```

Packages and `SHA256SUMS` are written to `desktop/dist/release/`. If Docker
cannot reach the internet directly, set container-reachable proxy values, for
example on this Mac:

```bash
FYNE_CROSS_HTTP_PROXY=http://host.docker.internal:7897 \
FYNE_CROSS_HTTPS_PROXY=http://host.docker.internal:7897 \
./release-cross.sh
```

If the Docker VM cannot keep both cross-compiler images at once, build each
target separately with `fyne-cross` and then stage the packages with
`MODELCTL_CROSS_SKIP_BUILD=1 ./release-cross.sh`. The official repository
workflow also builds Windows/Linux packages on a clean Linux runner.

Windows and Linux packages include the client, daemon, Node/Python/Laya runtime,
and a verified runtime manifest. Model weights remain separate and download on
first use. Windows builds are unsigned; Linux builds target x86_64 and provide
an AppImage plus a tar.xz fallback. Both require an X11/OpenGL desktop runtime.
Use the packaged platform README for installation steps.

To create distributable artifacts:

```bash
cd desktop
./release-macos.sh
```

This creates a versioned ZIP, DMG, and SHA-256 file under `desktop/dist/`.
The package includes and supervises its local daemon and Python runtime. Without
`MODELCTL_SIGNING_IDENTITY`, the local package uses an ad-hoc signature; public
distribution requires a Developer ID certificate and notarization.

## Decisions

Open **Playground** after starting a model instance. **Form** mode accepts
multiple named questions in one request:

- `noul`: yes/no probability; no criteria needed.
- `choice`: one `key: description` option per line.
- `score`: one ordered level per line, lowest first.

**JSON** mode accepts the full Laya `state` and `questions` payload, including
conversation turns as the state, optional `noul` labels, and `choice` labels
as either an object or array. Use **Example** to load a two-question request.
The client validates the input before sending it. Results show the chosen
values, score levels, and probabilities; expand **Raw JSON** to inspect the
full model response.

The selected model variant, device, decision draft, and answers remain in the
window when switching between Models, Running, and Playground. Refresh reloads
model and instance status from the daemon.

## Business capability integration

The desktop client now exposes a Capabilities page above the raw Laya playground.
Create a capability such as `refund-check`, bind it to an installed model variant,
and publish a human-readable question. The daemon stores the capability contract
and exposes it to application developers:

```http
POST http://127.0.0.1:11435/v1/capabilities/refund-check/invoke
Content-Type: application/json

{"input":{"text":"The customer was charged twice and wants a refund."}}
```

The response contains a stable capability version, typed answers, model metadata,
and a replayable run id. Business applications should call this endpoint or use a
small SDK generated from the capability contract. They should not read the model
store or construct Laya `state/questions` payloads themselves.

The Capabilities page lets an administrator publish semantic versions, keep a
version inactive for testing, activate a version, run single or batch samples,
copy REST/SDK examples, and replay a historical run. After a model variant is
downloaded, the first capability request starts its matching runtime
automatically. If the variant is missing, the API reports the exact model and
variant that must be downloaded.

The capability editor includes starter templates for refund detection, support
ticket routing, and document risk triage. A non-specialist can choose a
workflow, adjust its business fields and decisions, and publish it without
writing raw Laya/Jev questions.

The self-contained Windows launcher starts `Modelctl.exe` directly. The desktop
Supervisor verifies and starts the bundled Node/Python runtime, so a packaged
installation does not require a global `modelctl`, Node.js, or Python command.
