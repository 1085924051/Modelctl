$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$client = Join-Path $root "Modelctl.exe"
$healthUrl = "http://127.0.0.1:11435/health"

if (-not (Test-Path $client)) {
    Write-Host "Modelctl.exe was not found next to this launcher."
    exit 1
}

$daemonCommand = Get-Command modelctl -ErrorAction SilentlyContinue
if ($null -eq $daemonCommand) {
    Write-Host "The modelctl command was not found. Install Node.js 20+, install Modelctl, and run modelctl setup first."
    exit 1
}

$daemonPath = $daemonCommand.Source
if ([string]::IsNullOrWhiteSpace($daemonPath)) {
    $daemonPath = $daemonCommand.Path
}

$ready = $false
try {
    $response = Invoke-WebRequest -UseBasicParsing -Uri $healthUrl -TimeoutSec 2
    $ready = $response.StatusCode -eq 200
} catch {
    Start-Process -FilePath $daemonPath -ArgumentList @("daemon") -WorkingDirectory $root -WindowStyle Minimized | Out-Null
}

for ($attempt = 0; $attempt -lt 40 -and -not $ready; $attempt++) {
    Start-Sleep -Milliseconds 250
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $healthUrl -TimeoutSec 2
        $ready = $response.StatusCode -eq 200
    } catch {
        $ready = $false
    }
}

if (-not $ready) {
    Write-Host "Modelctl daemon did not become ready at $healthUrl. Run 'modelctl daemon' in PowerShell to inspect the error."
    exit 1
}

Start-Process -FilePath $client -WorkingDirectory $root | Out-Null
