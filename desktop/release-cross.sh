#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DIST="$ROOT/dist"
RELEASE="$DIST/release"
VERSION="${MODELCTL_DESKTOP_VERSION:-0.1.0}"
FYNE_CROSS="${FYNE_CROSS:-$(go env GOPATH)/bin/fyne-cross}"
mkdir -p "$DIST"
STAGE=$(mktemp -d "$DIST/.release-cross.XXXXXX")
trap 'rm -rf "$STAGE"' EXIT HUP INT TERM

if [ ! -x "$FYNE_CROSS" ]; then
  echo "fyne-cross is required. Install it with: go install github.com/fyne-io/fyne-cross@v1.6.2" >&2
  exit 2
fi
if ! docker info >/dev/null 2>&1; then
  echo "A running Docker-compatible engine is required by fyne-cross." >&2
  exit 2
fi

mkdir -p "$RELEASE"
rm -f "$RELEASE/Modelctl-windows-x64-v${VERSION}.zip" \
  "$RELEASE/Modelctl-windows-amd64-v${VERSION}.zip" \
  "$RELEASE/Modelctl-linux-x86_64-v${VERSION}.tar.xz" \
  "$RELEASE/SHA256SUMS" \
  "$RELEASE/README.txt"

build_target() {
  target=$1
  arch=$2
  set -- "$FYNE_CROSS" "$target" "-arch=$arch" "-name=Modelctl" "-app-id=com.modelctl.desktop" "-app-version=$VERSION"
  if [ -n "${FYNE_CROSS_HTTP_PROXY:-}" ]; then set -- "$@" "-env=HTTP_PROXY=$FYNE_CROSS_HTTP_PROXY"; fi
  if [ -n "${FYNE_CROSS_HTTPS_PROXY:-}" ]; then set -- "$@" "-env=HTTPS_PROXY=$FYNE_CROSS_HTTPS_PROXY"; fi
  if [ -n "${FYNE_CROSS_GOPROXY:-}" ]; then set -- "$@" "-env=GOPROXY=$FYNE_CROSS_GOPROXY"; fi
  (cd "$ROOT" && "$@" .)
}

if [ ! -f "$ROOT/Icon.png" ]; then (cd "$ROOT" && go run ./tools/generate_icon.go); fi

if [ "${MODELCTL_CROSS_SKIP_BUILD:-0}" != "1" ]; then
  build_target windows amd64
  build_target linux amd64
fi

for target in windows-amd64 linux-amd64; do
  if [ ! -f "$DIST/runtime/$target/runtime-manifest.json" ]; then
    echo "A verified $target runtime is required at $DIST/runtime/$target" >&2
    exit 2
  fi
done

WINDOWS_STAGE="$STAGE/Modelctl-Windows-x64"
mkdir -p "$WINDOWS_STAGE"
unzip -q -o "$ROOT/fyne-cross/dist/windows-amd64/Modelctl.zip" -d "$WINDOWS_STAGE"
cp "$ROOT/packaging/README-windows.txt" "$WINDOWS_STAGE/README.txt"
cp "$ROOT/RELEASE_NOTES.md" "$WINDOWS_STAGE/RELEASE_NOTES.txt"
cp "$ROOT/packaging/Start-Modelctl.ps1" "$WINDOWS_STAGE/Start-Modelctl.ps1"
cp "$ROOT/packaging/Start-Modelctl.cmd" "$WINDOWS_STAGE/Start-Modelctl.cmd"
cp -R "$DIST/runtime/windows-amd64" "$WINDOWS_STAGE/runtime"
WINDOWS_PACKAGE="$RELEASE/Modelctl-windows-x64-v${VERSION}.zip"
(cd "$STAGE" && zip -qry "$WINDOWS_PACKAGE" "Modelctl-Windows-x64")

LINUX_STAGE="$STAGE/Modelctl-Linux-x86_64"
mkdir -p "$LINUX_STAGE"
tar -xJf "$ROOT/fyne-cross/dist/linux-amd64/Modelctl.tar.xz" -C "$LINUX_STAGE"
cp "$ROOT/packaging/README-linux.txt" "$LINUX_STAGE/README.txt"
cp "$ROOT/RELEASE_NOTES.md" "$LINUX_STAGE/RELEASE_NOTES.txt"
cp "$ROOT/packaging/install-linux.sh" "$LINUX_STAGE/install.sh"
chmod 755 "$LINUX_STAGE/install.sh"
cp -R "$DIST/runtime/linux-amd64" "$LINUX_STAGE/runtime"
LINUX_PACKAGE="$RELEASE/Modelctl-linux-x86_64-v${VERSION}.tar.xz"
tar -cJf "$LINUX_PACKAGE" -C "$STAGE" "Modelctl-Linux-x86_64"

MAC_DMG="$DIST/Modelctl-macOS-arm64-v${VERSION}-adhoc.dmg"
MAC_ZIP="$DIST/Modelctl-macOS-arm64-v${VERSION}-adhoc.zip"
for file in "$MAC_DMG" "$MAC_ZIP"; do
  if [ -f "$file" ]; then cp "$file" "$RELEASE/"; fi
done

cat > "$RELEASE/README.txt" <<EOF
Modelctl Desktop $VERSION packages

Windows: Modelctl-windows-x64-v$VERSION.zip (Windows 10/11 x64, unsigned)
Linux:   Modelctl-linux-x86_64-v$VERSION.tar.xz (Linux x86_64)
macOS:   Modelctl-macOS-arm64-v$VERSION-adhoc.dmg / .zip (Apple Silicon, ad-hoc signed)

The macOS files are included when release-macos.sh has been run before this script.
All packages contain the client only. Install the Modelctl daemon/runtime separately.
The Windows package includes Start-Modelctl.cmd, which starts the local daemon when the modelctl command is installed.
Verify the platform package against SHA256SUMS before installing or sharing.

Windows is unsigned. macOS is ad-hoc signed when no Developer ID identity is
configured. Public distribution requires platform signing and macOS notarization.
EOF

cd "$RELEASE"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum Modelctl-* > SHA256SUMS
else
  LC_ALL=C shasum -a 256 Modelctl-* > SHA256SUMS
fi

printf 'Release artifacts (%s):\n' "$VERSION"
ls -lh "$RELEASE"/Modelctl-* "$RELEASE/SHA256SUMS"
