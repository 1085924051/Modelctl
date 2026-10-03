Modelctl Desktop 0.1.0 - Linux x86_64

Requirements
- Linux x86_64 with an X11 desktop session.
- The Fyne package is built for the target Linux architecture; confirm graphics
  driver and display-server support on the target distribution.
- The archive includes the Modelctl daemon, Node/Python/Laya runtime, and a verified runtime manifest.
- No Node.js, Python, pip, Laya, or project checkout is required on the target machine.

Install for the current user
1. Extract this archive.
2. Run `./install.sh` from the extracted Modelctl-Linux-x86_64 directory.
3. Launch Modelctl Desktop from the application menu or run
   `~/.local/bin/modelctl-desktop`.

Model weights are not included. They are downloaded on first use through the
proxy configured in the application settings. The runtime manifest is checked
before startup; a damaged or incomplete archive fails with a repair message.
