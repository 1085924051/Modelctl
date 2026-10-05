#!/usr/bin/env bash
set -euo pipefail

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
VERSION="${MODELCTL_DESKTOP_VERSION:-0.1.0}"
DIST="$ROOT/dist"
APPIMAGE_TOOL="${APPIMAGE_TOOL:-appimagetool}"
RUNTIME="$DIST/runtime/linux-amd64"
SOURCE_TAR="$ROOT/fyne-cross/dist/linux-amd64/Modelctl.tar.xz"
STAGE="$DIST/.appimage-stage"
APPDIR="$STAGE/Modelctl.AppDir"

command -v "$APPIMAGE_TOOL" >/dev/null 2>&1 || { echo "appimagetool is required" >&2; exit 2; }
[ -f "$SOURCE_TAR" ] || { echo "Linux Fyne package is missing: $SOURCE_TAR" >&2; exit 2; }
[ -f "$RUNTIME/runtime-manifest.json" ] || { echo "Linux runtime is missing: $RUNTIME" >&2; exit 2; }

rm -rf "$STAGE"
mkdir -p "$APPDIR/usr/bin" "$APPDIR/usr/share/modelctl" "$APPDIR/usr/share/icons/hicolor/256x256/apps"
tar -xJf "$SOURCE_TAR" -C "$STAGE"
install -m 0755 "$STAGE/usr/local/bin/desktop" "$APPDIR/usr/bin/modelctl-desktop"
cp -R "$RUNTIME" "$APPDIR/usr/share/modelctl/runtime"
if [ -f "$STAGE/usr/local/share/pixmaps/com.modelctl.desktop.png" ]; then
  install -m 0644 "$STAGE/usr/local/share/pixmaps/com.modelctl.desktop.png" "$APPDIR/usr/share/icons/hicolor/256x256/apps/modelctl.png"
  install -m 0644 "$STAGE/usr/local/share/pixmaps/com.modelctl.desktop.png" "$APPDIR/modelctl.png"
fi
cp "$ROOT/packaging/appimage/AppRun" "$APPDIR/AppRun"
cp "$ROOT/packaging/appimage/modelctl.desktop" "$APPDIR/modelctl.desktop"
chmod 0755 "$APPDIR/AppRun"
"$APPIMAGE_TOOL" "$APPDIR" "$DIST/Modelctl-linux-x86_64-v${VERSION}.AppImage"
rm -rf "$STAGE"
printf '%s\n' "$DIST/Modelctl-linux-x86_64-v${VERSION}.AppImage"
