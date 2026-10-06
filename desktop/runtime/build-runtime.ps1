$ErrorActionPreference = "Stop"

$RootDir = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$Target = if ($env:MODELCTL_RUNTIME_TARGET) { $env:MODELCTL_RUNTIME_TARGET } else { throw "MODELCTL_RUNTIME_TARGET is required" }
$Output = if ($env:MODELCTL_RUNTIME_OUTPUT) { $env:MODELCTL_RUNTIME_OUTPUT } else { Join-Path $RootDir "desktop\dist\runtime\$Target" }
$NodeBinary = if ($env:MODELCTL_NODE_BINARY) { $env:MODELCTL_NODE_BINARY } else { throw "MODELCTL_NODE_BINARY is required" }
$NodeRoot = if ($env:MODELCTL_NODE_ROOT) { $env:MODELCTL_NODE_ROOT } else { throw "MODELCTL_NODE_ROOT is required" }
$PythonBinary = if ($env:MODELCTL_PYTHON_BINARY) { $env:MODELCTL_PYTHON_BINARY } else { throw "MODELCTL_PYTHON_BINARY is required" }
$PythonRoot = if ($env:MODELCTL_PYTHON_ROOT) { $env:MODELCTL_PYTHON_ROOT } else { throw "MODELCTL_PYTHON_ROOT is required" }

& node (Join-Path $RootDir "desktop\runtime\build-runtime.mjs") `
  --target $Target --output $Output --source $RootDir `
  --node-binary $NodeBinary --node-root $NodeRoot --python-binary $PythonBinary --python-root $PythonRoot
& node (Join-Path $RootDir "desktop\runtime\tests\verify-runtime.mjs") $Output
& powershell -ExecutionPolicy Bypass -File (Join-Path $RootDir "desktop\runtime\assert-layout.ps1") $Output $Target
