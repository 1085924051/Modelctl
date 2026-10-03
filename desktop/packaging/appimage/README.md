# Linux AppImage Layout

The release workflow creates an AppDir with:

- `usr/bin/modelctl-desktop`
- `usr/share/modelctl/runtime/`
- `usr/share/applications/modelctl.desktop`
- `usr/share/icons/hicolor/256x256/apps/modelctl.png`
- `AppRun`

The AppImage must be built only after `runtime-manifest.json` has passed the
offline verifier. Model weights remain outside the AppImage.
