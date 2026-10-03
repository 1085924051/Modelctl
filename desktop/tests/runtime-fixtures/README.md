# Runtime Smoke Fixtures

The smoke tests use a built runtime tree, not the source checkout. A fixture
must contain a verified `runtime-manifest.json`, the platform Node/Python
executables, the control-plane files, and the license index. Model weights are
not needed for the startup smoke test.
