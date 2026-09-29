Modelctl Desktop 0.1.0 - Windows x64

Requirements
- Windows 10 or 11, 64-bit.
- Modelctl daemon installed and running on this computer.
- Default daemon address: http://127.0.0.1:11435.

Install and run
1. Extract this ZIP to a folder such as %LOCALAPPDATA%\Programs\Modelctl.
2. Double-click Modelctl.exe.

The client does not include the daemon, Python runtime, or model weights. Install
Modelctl separately and run `modelctl setup` before pulling Laya. Model downloads
use the proxy configured in the daemon's local settings. Set MODELCTL_URL before
launching Modelctl.exe only when the daemon uses a different address.

This build is unsigned. Windows may display SmartScreen because this project
does not yet have a code-signing certificate.
