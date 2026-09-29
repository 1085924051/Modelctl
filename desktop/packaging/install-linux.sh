#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PREFIX="${XDG_DATA_HOME:-$HOME/.local/share}/modelctl"
BIN_DIR="$HOME/.local/bin"
APP_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
ICON_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor/256x256/apps"

mkdir -p "$PREFIX" "$BIN_DIR" "$APP_DIR" "$ICON_DIR"
install -m 0755 "$ROOT/usr/local/bin/desktop" "$PREFIX/modelctl-desktop"
ln -sf "$PREFIX/modelctl-desktop" "$BIN_DIR/modelctl-desktop"
install -m 0644 "$ROOT/usr/local/share/pixmaps/com.modelctl.desktop.png" "$ICON_DIR/modelctl.png"
cat > "$APP_DIR/modelctl.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Modelctl
Comment=Local structured decision models
Exec=$PREFIX/modelctl-desktop
Icon=modelctl
Terminal=false
Categories=Development;Utility;
StartupWMClass=modelctl
EOF
printf 'Installed for current user. Launch Modelctl from the application menu or %s/modelctl-desktop\n' "$BIN_DIR"
