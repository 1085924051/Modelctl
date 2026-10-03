Modelctl Desktop 0.1.0 - Windows x64

Requirements
- Windows 10 or 11, 64-bit.
- The archive includes the Modelctl daemon, Node/Python/Laya runtime, and a verified runtime manifest.
- No Node.js, Python, pip, Laya, or project checkout is required on the target machine.
- Default daemon address: http://127.0.0.1:11435.

Install and run
1. Extract this ZIP to a folder such as %LOCALAPPDATA%\Programs\Modelctl.
2. Double-click `Modelctl.exe`; it verifies and starts the packaged runtime automatically.

Model weights are not included. They are downloaded on first use through the
proxy configured in the application settings.

If the client reports a damaged runtime, extract the archive again. If it reports
a model download problem, configure the proxy in Settings and retry.

This build is unsigned. Windows may display SmartScreen because this project
does not yet have a code-signing certificate.
