#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TARGET="${MODELCTL_RUNTIME_TARGET:?set MODELCTL_RUNTIME_TARGET to darwin-arm64, windows-amd64, or linux-amd64}"
OUTPUT="${MODELCTL_RUNTIME_OUTPUT:-$ROOT_DIR/desktop/dist/runtime/$TARGET}"
NODE_BINARY="${MODELCTL_NODE_BINARY:?set MODELCTL_NODE_BINARY to a real target Node executable}"
NODE_ROOT="${MODELCTL_NODE_ROOT:?set MODELCTL_NODE_ROOT to the complete target Node runtime directory}"
PYTHON_BINARY="${MODELCTL_PYTHON_BINARY:?set MODELCTL_PYTHON_BINARY to a real target Python executable}"
PYTHON_ROOT="${MODELCTL_PYTHON_ROOT:?set MODELCTL_PYTHON_ROOT to the complete target Python runtime directory}"

node "$ROOT_DIR/desktop/runtime/build-runtime.mjs" \
  --target "$TARGET" \
  --output "$OUTPUT" \
  --source "$ROOT_DIR" \
  --node-binary "$NODE_BINARY" \
  --node-root "$NODE_ROOT" \
  --python-binary "$PYTHON_BINARY" \
  --python-root "$PYTHON_ROOT"
node "$ROOT_DIR/desktop/runtime/tests/verify-runtime.mjs" "$OUTPUT"
