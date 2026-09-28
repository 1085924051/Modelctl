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

The client uses the daemon API only. It does not start a browser or a Python
runtime. The daemon must already be running; the default endpoint is
`http://127.0.0.1:11435`.

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
