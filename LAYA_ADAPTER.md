# Laya Adapter

Status: implemented control-plane adapter. The target runtime is Linux and
macOS with Python 3.10+, PyTorch, and the pinned Laya package.

## Model contract

The catalog entry `catalog/laya-0.3.18.json` pins:

- Hugging Face repository: `convaiinnovations/laya`
- Revision: `cf7c54c0586eede67d827dfaab8cd2d2007273e`
- Capability: `system_one`
- Variants: `english`, `multilingual`
- Per-file SHA-256 and sizes for weights, encoder configuration, and tokenizer

The model downloader verifies every file before recording the variant as
installed. Runtime files are copied from the content-addressed artifact store;
the original verified artifacts are never modified.

## Adapter process

`adapters/laya/serve.py` receives a local model directory, variant, device,
host, and port. It creates a Laya `Agent` directly from that filesystem path and
passes a small facade to `laya.serve.create_app`. This is intentional: using
the upstream `Router` with its default model map could resolve a missing model
from Hugging Face during language routing. The facade ensures every request
uses the selected, verified local checkpoint.

The adapter binds to `127.0.0.1` and exposes:

```text
GET  /health
POST /v1/systemone
```

The control plane waits for `/health`, records stdout/stderr in the instance
log, and reports adapter startup or exit errors through the instance resource.

## Runtime setup

Run `modelctl setup` (or `scripts/setup-laya.sh`) on Linux/macOS. It creates a
private virtual environment in `$MODELCTL_DATA_DIR/venv` and installs
`laya[serve]==0.3.18`. modelctl discovers that interpreter automatically. Set
`MODELCTL_PYTHON` only to override it. The Windows PowerShell helper is
provided for development only.

CUDA and MPS are explicit profiles in the catalog. The control plane checks the
host platform before starting a profile. Actual driver, PyTorch build, memory,
and performance support must still be verified on the target machine.

## Request shape

The adapter preserves Laya's structured decision protocol:

```json
{
  "state": "We were charged twice.",
  "questions": {
    "refund": {
      "type": "noul",
      "instructions": "Does the customer ask for a refund?"
    }
  }
}
```

The returned `answers`, `usage`, and `routing` fields are passed through. MCP
and Skill integrations call the control API and never access adapter files.
