$ErrorActionPreference = "Stop"
$RuntimeRoot = (Resolve-Path $args[0]).Path
$Port = if ($args.Count -gt 1) { $args[1] } else { 11435 }
$DataDir = Join-Path $env:TEMP ("modelctl-smoke-" + [guid]::NewGuid())
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
$Node = Join-Path $RuntimeRoot "node\node.exe"
$Python = Join-Path $RuntimeRoot "python\python.exe"
if (-not (Test-Path $Node) -or -not (Test-Path $Python)) {
  Write-Host "Expected Node: $Node"
  Write-Host "Expected Python: $Python"
  Get-ChildItem -Path $RuntimeRoot -Recurse -Depth 3 -File | Select-Object -ExpandProperty FullName | Select-Object -First 80
  throw "runtime executables are missing"
}
$env:MODELCTL_RUNTIME_ROOT = $RuntimeRoot
$env:MODELCTL_DATA_DIR = $DataDir
$env:MODELCTL_PYTHON = $Python
$env:MODELCTL_PORT = "$Port"
$process = Start-Process -FilePath $Node -ArgumentList (Join-Path $RuntimeRoot "control-plane\bin\modelctl.js"), "daemon" -PassThru -RedirectStandardOutput (Join-Path $DataDir "daemon.log") -RedirectStandardError (Join-Path $DataDir "daemon.err")
try {
  for ($i = 0; $i -lt 80; $i++) {
    try { Invoke-WebRequest -UseBasicParsing "http://127.0.0.1:$Port/health" -TimeoutSec 2 | Out-Null; break } catch { Start-Sleep -Milliseconds 250 }
  }
  Invoke-WebRequest -UseBasicParsing "http://127.0.0.1:$Port/health" -TimeoutSec 2 | Out-Null
  Invoke-WebRequest -UseBasicParsing "http://127.0.0.1:$Port/v1/models" -TimeoutSec 2 | Out-Null
  Write-Host "runtime smoke OK: $RuntimeRoot"
} finally {
  Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
  Remove-Item -Recurse -Force $DataDir
}
