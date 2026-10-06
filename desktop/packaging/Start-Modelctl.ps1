$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$client = Join-Path $root "Modelctl.exe"

if (-not (Test-Path $client)) {
    Write-Host "Modelctl.exe was not found next to this launcher."
    exit 1
}

# Modelctl.exe verifies and starts the packaged daemon/runtime through its
# Supervisor. No global Node.js, Python, or modelctl CLI installation is
# required for the self-contained Windows package.
Start-Process -FilePath $client -WorkingDirectory $root | Out-Null
