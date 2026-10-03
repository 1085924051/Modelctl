Modelctl Desktop 0.1.0 - Windows x64

Requirements
- Windows 10 or 11, 64-bit.
- Modelctl daemon installed and running on this computer.
- Node.js 20 or newer, with the `modelctl` command available in PowerShell.
- Default daemon address: http://127.0.0.1:11435.

Install and run
1. Extract this ZIP to a folder such as %LOCALAPPDATA%\Programs\Modelctl.
2. If Modelctl is not installed yet, install it from the repository with `npm install -g .`, then run `modelctl setup` once.
3. Double-click `Start-Modelctl.cmd`. It checks the daemon, starts `modelctl daemon` when needed, waits for `/health`, and then opens `Modelctl.exe`.
4. You can still double-click `Modelctl.exe` directly, but it requires the daemon to be running already.

The Windows desktop package contains the client and launcher only. It does not
include Node.js, the daemon source/runtime, Python, or model weights. Install
Modelctl separately and run `modelctl setup` before pulling Laya. Model downloads
use the proxy configured in the daemon's local settings. Set MODELCTL_URL before
launching Modelctl.exe only when the daemon uses a different address.

If the client shows `Offline` or `connection refused`, run `modelctl daemon` in a
PowerShell window and confirm that http://127.0.0.1:11435/health returns a healthy
response. The same check is performed by `Start-Modelctl.cmd`.

This build is unsigned. Windows may display SmartScreen because this project
does not yet have a code-signing certificate.
