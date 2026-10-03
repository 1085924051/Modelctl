# Packaged Runtime Layout

The desktop release builder places a read-only runtime tree beside the native
client. The client verifies `runtime-manifest.json` before starting the local
control plane.

Required entries:

- `node/bin/node` on macOS/Linux or `node/node.exe` on Windows, plus the Node runtime libraries
- `control-plane/bin/modelctl.js`
- `python/bin/python` or `python/python.exe`
- `runtime-manifest.json`
- `licenses/NOTICE.txt` and the dependency license index

The runtime tree must not contain model weights or user settings. Model files
are downloaded into `MODELCTL_DATA_DIR` after the application starts.
