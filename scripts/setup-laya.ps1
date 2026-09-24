$ErrorActionPreference = "Stop"

$RootDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$DataDir = if ($env:MODELCTL_DATA_DIR) { $env:MODELCTL_DATA_DIR } else { Join-Path $HOME ".modelctl" }
$PythonBin = if ($env:PYTHON_BIN) { $env:PYTHON_BIN } else { "python" }
$LayaVersion = if ($env:LAYA_VERSION) { $env:LAYA_VERSION } else { "0.3.18" }

if (-not (Get-Command node -ErrorAction SilentlyContinue)) { throw "node >= 20 is required" }
$NodeMajor = [int]((& node -p "process.versions.node.split('.')[0]").Trim())
if ($NodeMajor -lt 20) { throw "Node.js 20+ is required" }
if (-not (Get-Command $PythonBin -ErrorAction SilentlyContinue)) { throw "$PythonBin (Python 3.10+) is required" }

$PythonVersion = & $PythonBin -c "import sys; print('%d.%d' % sys.version_info[:2])"
$VersionParts = $PythonVersion.Trim().Split('.')
if ([int]$VersionParts[0] -lt 3 -or ([int]$VersionParts[0] -eq 3 -and [int]$VersionParts[1] -lt 10)) { throw "Python 3.10+ is required (found $PythonVersion)" }

$VenvDir = Join-Path $DataDir "venv"
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
if (-not (Test-Path (Join-Path $VenvDir "Scripts\python.exe"))) { & $PythonBin -m venv $VenvDir }
$VenvPython = Join-Path $VenvDir "Scripts\python.exe"
& $VenvPython -m pip install --upgrade pip
& $VenvPython -m pip install "laya[serve]==$LayaVersion"

Write-Output "Laya runtime installed."
Write-Output "Set MODELCTL_DATA_DIR=$DataDir"
Write-Output "Set MODELCTL_PYTHON=$VenvPython"
Write-Output "Then run: node `"$RootDir\bin\modelctl.js`" daemon"
