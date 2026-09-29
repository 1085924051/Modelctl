#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
APP="$ROOT/dist/Modelctl.app"
BIN="$APP/Contents/MacOS/modelctl-desktop"
VERSION="${MODELCTL_DESKTOP_VERSION:-0.1.0}"
ARCH="${MODELCTL_GOARCH:-arm64}"

mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDisplayName</key>
	<string>Modelctl</string>
	<key>CFBundleExecutable</key>
	<string>modelctl-desktop</string>
	<key>CFBundleIdentifier</key>
	<string>com.modelctl.desktop</string>
	<key>CFBundleName</key>
	<string>Modelctl</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>__MODELCTL_VERSION__</string>
	<key>CFBundleVersion</key>
	<string>__MODELCTL_VERSION__</string>
	<key>LSMinimumSystemVersion</key>
	<string>13.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
PLIST

sed -i '' "s/__MODELCTL_VERSION__/$VERSION/g" "$APP/Contents/Info.plist"

cd "$ROOT"
GOOS=darwin GOARCH="$ARCH" CGO_ENABLED=1 go build -o "$BIN" .
echo "$APP"
