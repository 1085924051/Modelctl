Modelctl Desktop 0.1.0 - Linux x86_64

Requirements
- Linux x86_64 with an X11 desktop session.
- The Fyne package is built for the target Linux architecture; confirm graphics
  driver and display-server support on the target distribution.
- Modelctl daemon installed and running, defaulting to 127.0.0.1:11435.

Install for the current user
1. Extract this archive.
2. Run `./install.sh` from the extracted Modelctl-Linux-x86_64 directory.
3. Launch Modelctl Desktop from the application menu or run
   `~/.local/bin/modelctl-desktop`.

The client does not include the daemon, Python runtime, or model weights. Install
Modelctl separately and run `modelctl setup` before pulling Laya. Model downloads
use the proxy configured in the daemon's local settings. Set MODELCTL_URL before
launching the client only when the daemon uses a different address.
