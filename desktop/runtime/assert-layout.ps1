param(
  [Parameter(Mandatory = $true, Position = 0)][string]$RuntimeRoot,
  [Parameter(Mandatory = $true, Position = 1)][string]$Target
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path (Join-Path $RuntimeRoot "runtime-manifest.json") -PathType Leaf)) {
  throw "runtime manifest is missing: $RuntimeRoot\runtime-manifest.json"
}

switch ($Target) {
  "windows-amd64" { $required = @("node\node.exe", "python\python.exe", "control-plane\bin\modelctl.js") }
  "darwin-arm64" { $required = @("node/bin/node", "python/bin/python", "control-plane/bin/modelctl.js") }
  "linux-amd64" { $required = @("node/bin/node", "python/bin/python", "control-plane/bin/modelctl.js") }
  default { throw "unsupported runtime target: $Target" }
}

foreach ($relative in $required) {
  $path = Join-Path $RuntimeRoot $relative
  if (-not (Test-Path $path -PathType Leaf)) {
    throw "runtime file is missing: $relative"
  }
}

Write-Host "runtime layout OK: $Target"
