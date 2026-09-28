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
