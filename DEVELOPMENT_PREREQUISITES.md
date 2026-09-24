# Development prerequisites

The current control plane requires Node.js 20 or newer. The Laya adapter
requires Python 3.10 or newer, PyTorch, and the pinned `laya[serve]==0.3.18`
package. Linux and macOS are the supported release targets.

## Local checks

```text
node --version             -> >= 20
node bin/modelctl.js catalog validate
node bin/modelctl.js daemon
GET http://127.0.0.1:11435/health
```

Install the Python environment with `scripts/setup-laya.sh`. It creates a
private environment under `$MODELCTL_DATA_DIR/venv` and prints the
`MODELCTL_PYTHON` value needed by the daemon.

## Target machine verification

On a Linux or macOS machine, record:

1. Python, PyTorch, Laya, and serving package versions.
2. Downloaded artifact SHA-256 values from `catalog/laya-0.3.18.json`.
3. Successful CPU startup, `/health`, and a real `system_one` call.
4. Clean stop, daemon restart, and adapter failure recovery.
5. CUDA or MPS device and latency data when that profile is tested.

Until those target-machine checks are recorded, release notes should describe
CUDA/MPS and real model inference as pending verification.

## Verification record

The control plane and Laya CPU adapter were exercised on 2026-09-24 with
Python 3.12.10, PyTorch 2.14.0, and `laya==0.3.18`. The English variant's
846,195,574 bytes were downloaded through the pinned revision and all five
artifacts passed size and SHA-256 checks. The adapter reported `loaded:
["english"]`, `device: cpu`, and returned HTTP 200 for a real
`POST /v1/systemone` request with `answers`, `usage`, and `routing`.

This validates the CPU adapter path on Windows for development. Linux/macOS,
CUDA, and MPS still require their own target-machine verification.
