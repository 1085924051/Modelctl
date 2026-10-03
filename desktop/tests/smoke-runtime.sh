#!/usr/bin/env bash
set -euo pipefail

RUNTIME_ROOT="${1:?usage: smoke-runtime.sh <runtime-dir> [port]}"
PORT="${2:-11435}"
DATA_DIR="$(mktemp -d "${TMPDIR:-/tmp}/modelctl-smoke.XXXXXX")"
cleanup() { if [ -n "${PID:-}" ]; then kill "$PID" 2>/dev/null || true; fi; rm -rf "$DATA_DIR"; }
trap cleanup EXIT HUP INT TERM

node_bin="$RUNTIME_ROOT/node/bin/node"
python_bin="$RUNTIME_ROOT/python/bin/python"
if [ ! -x "$node_bin" ] || [ ! -x "$python_bin" ]; then
  echo "runtime executables are missing" >&2
  exit 2
fi

MODELCTL_RUNTIME_ROOT="$RUNTIME_ROOT" MODELCTL_DATA_DIR="$DATA_DIR" MODELCTL_PYTHON="$python_bin" MODELCTL_PORT="$PORT" \
  "$node_bin" "$RUNTIME_ROOT/control-plane/bin/modelctl.js" daemon >"$DATA_DIR/daemon.log" 2>&1 &
PID=$!
for _ in $(seq 1 80); do
  if curl --fail --silent "http://127.0.0.1:$PORT/health" >/dev/null; then break; fi
  sleep 0.25
done
curl --fail --silent "http://127.0.0.1:$PORT/health" >/dev/null
curl --fail --silent "http://127.0.0.1:$PORT/v1/models" >/dev/null
echo "runtime smoke OK: $RUNTIME_ROOT"
