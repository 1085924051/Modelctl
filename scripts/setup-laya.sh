#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="${MODELCTL_DATA_DIR:-$HOME/.modelctl}"
PYTHON_BIN="${PYTHON_BIN:-python3}"
LAYA_VERSION="${LAYA_VERSION:-0.3.18}"

command -v node >/dev/null || { echo "node >= 20 is required" >&2; exit 1; }
node_major="$(node -p 'process.versions.node.split(".")[0]')"
if [ "$node_major" -lt 20 ]; then echo "Node.js 20+ is required" >&2; exit 1; fi
command -v "$PYTHON_BIN" >/dev/null || { echo "$PYTHON_BIN (Python 3.10+) is required" >&2; exit 1; }

python_version="$($PYTHON_BIN -c 'import sys; print("%d.%d" % sys.version_info[:2])')"
python_major="${python_version%%.*}"
python_minor="${python_version##*.}"
if [ "$python_major" -lt 3 ] || { [ "$python_major" -eq 3 ] && [ "$python_minor" -lt 10 ]; }; then
  echo "Python 3.10+ is required (found $python_version)" >&2
  exit 1
fi

VENV_DIR="$DATA_DIR/venv"
mkdir -p "$DATA_DIR"
if [ ! -x "$VENV_DIR/bin/python" ]; then
  "$PYTHON_BIN" -m venv "$VENV_DIR"
fi

"$VENV_DIR/bin/python" -m pip install --upgrade pip
"$VENV_DIR/bin/python" -m pip install "laya[serve]==$LAYA_VERSION"

cat <<EOF
Laya runtime installed.
modelctl discovers this environment automatically at:
  $VENV_DIR/bin/python
After setup, run:
  node "$ROOT_DIR/bin/modelctl.js" pull convaiinnovations/laya --variant english
EOF
