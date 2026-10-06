#!/bin/sh
set -eu

ROOT=${1:?usage: assert-layout.sh <runtime-dir> <target>}
TARGET=${2:?usage: assert-layout.sh <runtime-dir> <target>}

[ -d "$ROOT" ] || { echo "runtime directory is missing: $ROOT" >&2; exit 2; }
[ -f "$ROOT/runtime-manifest.json" ] || { echo "runtime manifest is missing: $ROOT/runtime-manifest.json" >&2; exit 2; }

case "$TARGET" in
  windows-amd64) NODE="node/node.exe"; PYTHON="python/python.exe" ;;
  darwin-arm64|linux-amd64) NODE="node/bin/node"; PYTHON="python/bin/python" ;;
  *) echo "unsupported runtime target: $TARGET" >&2; exit 2 ;;
esac

for required in "$NODE" "$PYTHON" "control-plane/bin/modelctl.js"; do
  if [ ! -f "$ROOT/$required" ]; then
    echo "runtime file is missing: $required" >&2
    exit 2
  fi
done

echo "runtime layout OK: $TARGET"
