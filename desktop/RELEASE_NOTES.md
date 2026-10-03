# Modelctl Desktop v0.1.0

Modelctl Desktop is a native client with a packaged local Modelctl daemon and
runtime. Model weights remain separate and download on first use.

## Requirements

- macOS 13 or newer on Apple Silicon.
- Modelctl daemon running at `http://127.0.0.1:11435`, or set `MODELCTL_URL`.
- A configured Modelctl runtime and model are required for inference.

The desktop package is the client UI. It does not contain the daemon, Python
runtime, PyTorch, or model weights.

## Install

1. Open the DMG.
2. Drag `Modelctl.app` into `Applications`.
3. Start the Modelctl daemon.
4. Open `Modelctl.app`.

This local build is ad-hoc signed because no Developer ID identity was
available on the build machine. macOS may require opening it from Finder with
Control-click -> Open the first time. A public release should be rebuilt with
a Developer ID Application certificate and notarized.

## Proxy

Model downloads use the daemon's configured proxy. Configure it from the
Modelctl settings API or the Modelctl Web UI before pulling a model:

```bash
curl -X PUT http://127.0.0.1:11435/v1/settings \
  -H 'content-type: application/json' \
  --data '{"proxy":{"http":"http://127.0.0.1:7897","https":"http://127.0.0.1:7897"}}'
```
