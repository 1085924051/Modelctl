# Go Native Desktop Client MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a simple native Go/Fyne macOS client for the existing Modelctl daemon.

**Architecture:** Keep the current Node daemon and Python adapter unchanged. Add an isolated `desktop/` Go module with an HTTP API client and a Fyne UI that manages models, instances, downloads, and Laya structured decisions.

**Tech Stack:** Go 1.25+, Fyne v2, Go standard library HTTP/JSON/testing, macOS arm64 app bundle.

**Spec:** `docs/superpowers/specs/2026-09-28-go-native-desktop-mvp-design.md`

## Global Constraints

- Do not modify or replace the existing Web UI.
- Do not rewrite the Node daemon or Python adapter.
- Use only the public daemon HTTP API.
- Default daemon URL is `http://127.0.0.1:11435`; honor `MODELCTL_URL`.
- Preserve Laya `system_one` request and response shapes.
- Keep the desktop UI compact and Ollama-inspired.
- Do not add credentials or remote networking to the desktop client.

---

### Task 1: Add Go module and daemon API client

**Files:**
- Create: `desktop/go.mod`
- Create: `desktop/internal/api/client.go`
- Create: `desktop/internal/api/client_test.go`

**Interfaces:**
- Produces `Client`, `NewClient(baseURL string)`, `Health`, `Models`, `ModelDetail`, `Instances`, `Pull`, `Task`, `StartInstance`, `StopInstance`, and `InvokeSystemOne`.

- [ ] Write tests for URL construction, JSON decoding, HTTP errors, and `MODELCTL_URL` defaulting.
- [ ] Run `cd desktop && go test ./internal/api` and verify failure before implementation.
- [ ] Implement typed DTOs and a JSON request helper using `net/http` and a 120 second request timeout.
- [ ] Implement pull task polling data without embedding UI logic.
- [ ] Run `cd desktop && go test ./internal/api` and verify all tests pass.
- [ ] Commit `feat: add Go Modelctl API client`.

### Task 2: Build native Fyne application shell

**Files:**
- Create: `desktop/main.go`
- Create: `desktop/internal/ui/app.go`
- Create: `desktop/internal/ui/theme.go`
- Create: `desktop/internal/ui/app_test.go`

**Interfaces:**
- Consumes `api.Client` and exposes a `New(client *api.Client) fyne.App` constructor plus refreshable UI state.

- [ ] Write tests for initial empty state and model state transformation.
- [ ] Run `cd desktop && go test ./internal/ui` and verify failure before implementation.
- [ ] Initialize a Fyne app with a fixed compact desktop window, light theme, left navigation, connection status, and refresh button.
- [ ] Implement model cards and running instance rows from API DTOs.
- [ ] Implement asynchronous refresh with UI updates on the main Fyne thread.
- [ ] Run `cd desktop && go test ./...` and verify the shell tests pass.
- [ ] Commit `feat: add native Go desktop shell`.

### Task 3: Add model lifecycle and playground actions

**Files:**
- Modify: `desktop/internal/ui/app.go`
- Create: `desktop/internal/ui/actions.go`
- Create: `desktop/internal/ui/actions_test.go`

**Interfaces:**
- Consumes `api.Client` lifecycle methods.
- Produces Pull/Run/Stop buttons, progress display, instance selection, and structured judgment result view.

- [ ] Write tests for pull progress percentage, variant payloads, and structured answer rendering data.
- [ ] Run `cd desktop && go test ./...` and verify the new tests fail before implementation.
- [ ] Implement Pull action with one task and a 750 ms polling ticker; disable conflicting controls while active.
- [ ] Implement Run action with selected model, variant, profile, and `default=true`.
- [ ] Implement Stop action and refresh state after completion.
- [ ] Implement a compact Laya playground using `state.body` and one `noul`, `choice`, or `score` question.
- [ ] Display answer values, confidence, probabilities, and raw JSON in a scrollable result area.
- [ ] Run `cd desktop && go test ./...` and verify all tests pass.
- [ ] Commit `feat: manage Laya instances from desktop client`.

### Task 4: Package and verify the macOS MVP

**Files:**
- Create: `desktop/build-macos.sh`
- Create: `desktop/README.md`
- Modify: `README.md`

**Interfaces:**
- Produces `desktop/dist/Modelctl.app` and documented build/run commands.

- [ ] Implement an app bundle build script using `go build`, `Info.plist`, and `Contents/MacOS/modelctl-desktop`.
- [ ] Run `cd desktop && ./build-macos.sh` and verify the app bundle exists.
- [ ] Run `cd desktop && go test ./...` and `go build ./...`.
- [ ] Start the desktop binary against the real daemon and verify it opens without a browser.
- [ ] Use API-backed smoke verification to confirm two variants and one ready MPS instance are visible.
- [ ] Document `MODELCTL_URL`, build, and launch instructions.
- [ ] Commit `feat: package Go desktop MVP`.

### Task 5: Publish the desktop MVP

**Files:**
- No additional source files.

- [ ] Run the full repository checks for unchanged Node/Web functionality.
- [ ] Review `git diff --check` and preserve the existing CLI executable-bit changes.
- [ ] Push `main` to `origin`.
- [ ] Report the desktop app path, build command, daemon URL, test results, and known MVP limits.
