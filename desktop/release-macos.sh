#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
VERSION="${MODELCTL_DESKTOP_VERSION:-0.1.0}"
ARCH="${MODELCTL_GOARCH:-arm64}"
DIST="$ROOT/dist"
APP="$DIST/Modelctl.app"
STAGE="$DIST/.stage"
IDENTITY="${MODELCTL_SIGNING_IDENTITY:-}"

if [ "$ARCH" != "arm64" ]; then
  echo "This MVP release script supports macOS arm64; set MODELCTL_GOARCH=arm64." >&2
  exit 2
fi

rm -rf "$STAGE"
mkdir -p "$DIST"
MODELCTL_DESKTOP_VERSION="$VERSION" MODELCTL_GOARCH="$ARCH" "$ROOT/build-macos.sh" >/dev/null

if [ -n "$IDENTITY" ]; then
  codesign --force --deep --options runtime --timestamp --sign "$IDENTITY" "$APP"
  SIGNING="Developer ID: $IDENTITY"
  LABEL="signed"
else
  codesign --force --deep --sign - "$APP"
  SIGNING="ad-hoc (no Developer ID configured)"
  LABEL="adhoc"
fi
codesign --verify --deep --strict "$APP"
node "$ROOT/runtime/tests/verify-runtime.mjs" "$APP/Contents/Resources/runtime"

PACKAGE="Modelctl-macOS-${ARCH}-v${VERSION}-${LABEL}"
ZIP="$DIST/${PACKAGE}.zip"
DMG="$DIST/${PACKAGE}.dmg"
SHA="$DIST/${PACKAGE}.sha256"

mkdir -p "$STAGE/Modelctl"
ditto "$APP" "$STAGE/Modelctl/Modelctl.app"
cp "$ROOT/../README.md" "$STAGE/Modelctl/README.md"
cp "$ROOT/RELEASE_NOTES.md" "$STAGE/RELEASE_NOTES.md"
ln -s /Applications "$STAGE/Applications"

ditto -c -k --keepParent "$STAGE/Modelctl" "$ZIP"
hdiutil create -volname "Modelctl" -srcfolder "$STAGE" -ov -format UDZO "$DMG" >/dev/null

cd "$DIST"
LC_ALL=C shasum -a 256 "$(basename "$ZIP")" "$(basename "$DMG")" > "$(basename "$SHA")"
rm -rf "$STAGE"

cat <<EOF
Release artifacts:
  $APP
  $ZIP
  $DMG
  $SHA
Signing: $SIGNING
EOF
