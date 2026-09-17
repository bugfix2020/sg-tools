# Main Window Follow Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a keyboard-only main-window/follow-window synchronization feature whose runtime handles are rebound after restart, whose Go core is independently testable, and whose existing clicker feature remains unchanged in behavior.

**Architecture:** Add `pkg/windowsync` as a platform-neutral state machine. It owns filter validation, runtime target binding, input event consumption, broadcast ordering, lifecycle cleanup, invalid-target failure, and the running-state mutation lock. Extend the existing clicker sender contract with a key-transition method so Windows continues to use the current `PostMessageW` implementation for both clicker presses and synchronized key down/up events. Add a Wails `FollowSyncService` facade and a separate React feature/tab; the Windows adapter supplies a low-level keyboard hook scoped to the selected foreground/root window and skips the current process.

**Tech Stack:** Go 1.26, Wails v2.15 typed bindings/events, Win32 `SetWindowsHookExW`/`PostMessageW`, React 18, Ant Design 5, Vitest, existing pnpm workspace.

**Spec:** Delegated user request in this conversation dated 2026-09-17; no repository spec file exists.

## Global Constraints

- Existing `pkg/clicker` behavior and the existing `clicker` tab must remain compatible.
- Main and follow window handles are runtime-only; persisted configuration contains only include/exclude rules.
- Only the main window may start and stop synchronization; follow windows have no independent controls.
- Exclude rules have higher evaluation priority than include rules; overlapping or duplicate rules are rejected before mutation.
- An empty include set means all keyboard codes are eligible unless excluded; a non-empty include set is an allowlist.
- Runtime edits to targets or rules are rejected while synchronization is running.
- Windows provides the real adapter; non-Windows builds return `ErrUnsupportedPlatform` without pretending to capture or send input.
- The implementation is keyboard-only but uses a generic `InputEvent` abstraction so mouse events can be added later without changing the controller contract.
- During implementation, do not modify `Desktop/clicker` reference sources; final integration/release is performed only after explicit user authorization.

### Task 1: Define the transition and synchronization core contracts

**Files:**
- Modify: `pkg/clicker/model.go`
- Create: `pkg/windowsync/model.go`
- Create: `pkg/windowsync/model_test.go`

**Interfaces:**
- `clicker.KeyTransition` carries `VirtualKey`, `ScanCode`, `Extended`, `Down`, and `Repeat`; `clicker.KeyEventSender` exposes `SendKey(context.Context, WindowTarget, KeyTransition) error`.
- `windowsync.FilterConfig` carries `Include []string` and `Exclude []string`.
- `windowsync.InputEvent` carries `Kind`, `Code`, `Label`, `SourceHandle`, and `Transition`; `InputKindKeyboard` is the first supported kind.
- `windowsync.InputCapture` exposes `Start(context.Context, CaptureSpec) (<-chan InputEvent, error)` and `Stop() error`.
- `windowsync.Dependencies` carries `Capture`, `Sender`, optional `ValidateTarget`, and `OnStateChanged`.
- `windowsync.Service` exposes `Configure`, `SetMainTarget`, `SetFollowTargets`, `AddFollowTarget`, `RemoveFollowTarget`, `Start`, `Stop`, `Shutdown`, and `Snapshot`.

- [x] **Step 1: Write the failing filter and conflict tests**

  Add tests for: empty include allowing an unexcluded key; non-empty include rejecting an unlisted key; exclude winning over include; duplicate include rejection; duplicate exclude rejection; include/exclude overlap rejection; and empty/whitespace code rejection.

- [x] **Step 2: Run the new tests and confirm the feature is absent**

  Run `go test ./pkg/windowsync -run 'TestFilter|TestValidate'`. Expected result: compilation or undefined-symbol failure because `pkg/windowsync` does not yet exist.

- [x] **Step 3: Add the minimal types, errors, normalization, and filter implementation**

  Normalize rule codes with `strings.ToLower(strings.TrimSpace(code))`. Validate every rule before constructing lookup maps. Return typed/sentinel errors for invalid rules, duplicate rules, conflicts, and unsupported input kinds. Make `FilterConfig.Allows(code)` apply exclude first, then include only when include is non-empty.

- [x] **Step 4: Run the focused tests and the existing clicker tests**

  Run `go test ./pkg/windowsync ./pkg/clicker`. Expected result: all focused and existing clicker tests pass.

### Task 2: Implement the synchronization state machine with TDD

**Files:**
- Create: `pkg/windowsync/service.go`
- Create: `pkg/windowsync/service_test.go`

**Interfaces:**
- `Service.Snapshot()` returns a copy containing `State`, `Main`, `Follows`, `Filter`, `CapturedCount`, `LastCode`, and `LastError`; `WindowTarget.Handle` is never serialized by the facade.
- `Service.Start` requires a valid main target and at least one follow target, validates all targets, starts capture once, and transitions to `running` only after capture succeeds.
- `Service.Stop` cancels capture, waits for the event loop, clears all runtime cancellation state, and returns to `ready`; sender/capture errors transition to `error` and stop further broadcasts.

- [x] **Step 1: Write failing service tests**

  Add real-core tests with small fakes for: a keyboard event from the main window being sent to two follow targets in order; filtered events not being sent; starting with an invalid target failing; stopping cleaning up the capture and goroutine; target invalidation through a sender error transitioning to `error`; and `Configure`, `SetMainTarget`, `SetFollowTargets`, `AddFollowTarget`, and `RemoveFollowTarget` returning `ErrSyncRunning` while running.

- [x] **Step 2: Run the service tests to confirm the expected red state**

  Run `go test ./pkg/windowsync -run 'TestService'`. Expected result: failure because the service implementation is missing.

- [x] **Step 3: Implement the minimum synchronized event loop**

  Copy targets at mutation boundaries, start exactly one event loop, increment `CapturedCount` for keyboard events, apply the validated filter, and call `SendKey` once per follow target in stable slice order. Handle closed capture channels and sender errors as runtime failures. Never expose start/stop methods for an individual follow target.

- [x] **Step 4: Run focused tests, then race-check the core**

  Run `go test ./pkg/windowsync -run 'TestService'` and `go test -race ./pkg/windowsync`. Expected result: all service tests pass without data races.

### Task 3: Reuse the existing Win32 key lifecycle and add Windows input capture

**Files:**
- Modify: `internal/platform/windows/platform_windows.go`
- Modify: `internal/platform/windows/platform_stub.go`
- Create: `internal/platform/windows/input_capture_windows.go`
- Create: `internal/platform/windows/input_capture_stub.go`
- Modify: `internal/platform/windows/platform_windows_test.go`

**Interfaces:**
- `KeySender.SendKey` posts `WM_KEYDOWN` or `WM_KEYUP` with the captured scan-code/extended bits; `Press` calls `SendKey` down, waits for its existing hold duration, and calls `SendKey` up.
- `NewInputCapture()` returns an `windowsync.InputCapture` implementation. Windows installs `WH_KEYBOARD_LL`, pumps messages on its locked OS thread, and returns normalized DOM-style codes; non-Windows returns `ErrUnsupportedPlatform`.
- `ValidateWindow(clicker.WindowTarget) error` checks `IsWindow` on Windows and returns `ErrUnsupportedPlatform` on non-Windows.

- [x] **Step 1: Add build-safe tests for the transition contract**

  Extend the Windows-only tests to cover common virtual-key-to-code normalization and keep existing `ResolveVirtualKey` coverage intact. Add a non-Windows test for `NewInputCapture().Start` returning `ErrUnsupportedPlatform` if the package test build can exercise the stub.

- [x] **Step 2: Run platform tests before adding implementation**

  Run `go test ./internal/platform/windows`. Expected result: the new transition/capture symbols are absent or the new tests fail.

- [x] **Step 3: Implement `SendKey` by extracting the existing PostMessage lifecycle**

  Preserve the current `IsWindow`, scan-code, error, and hold behavior. Generate down/up `lParam` values from `KeyTransition`, including bit 24 for extended keys and bits 30/31 for key-up. Keep `Press` as the existing clicker-facing method.

- [x] **Step 4: Implement the Windows low-level hook adapter**

  Scope events to the selected target's root foreground window, ignore any foreground window belonging to `os.Getpid()` so the Wails UI cannot self-trigger, call `CallNextHookEx` for every hook callback, use a bounded non-blocking event queue, and unhook/close all channels on cancellation. Use a generic `InputEvent` output so future mouse capture can share the boundary.

- [x] **Step 5: Implement the non-Windows stubs and run compile tests**

  Add stub methods that return `ErrUnsupportedPlatform`, then run `go test ./internal/platform/windows`, `go test ./pkg/clicker`, and `GOOS=windows GOARCH=amd64 go test -c ./internal/platform/windows -o "$env:TEMP\\sg-tools-windows-platform.test.exe"`. The cross-compiled binary is only a compile check and is not executed.

### Task 4: Add persisted rules and the typed Wails facade

**Files:**
- Modify: `pkg/clicker/config.go`
- Create: `app/follow_sync_service.go`
- Modify: `app/app.go`
- Modify: `main.go`
- Create: `app/follow_sync_service_test.go`

**Interfaces:**
- `clicker.AppConfig` adds `FollowSync KeyRuleConfig`; old version-1 config files remain loadable with empty rules and no handles.
- `app.FollowSyncService` exposes `GetSnapshot`, `SetRules`, `BeginMainWindowPick`, `BeginFollowWindowPick`, `CancelWindowPick`, `RemoveFollowWindow`, `ClearTargets`, `Start`, and `Stop` as Wails methods.
- Wails events are `follow-sync:state`, `follow-sync:window-pick-preview`, `follow-sync:window-pick-complete`, `follow-sync:window-pick-cancelled`, and `follow-sync:error`; event payloads use `WindowInfo` and never include handles.

- [x] **Step 1: Write the failing config and facade contract tests**

  Test that `FollowSync` rules round-trip through `ConfigStore`, a running service rejects `SetRules` without changing the prior config, main/follow pick completion updates the correct runtime target, and app shutdown invokes sync shutdown without changing clicker profile behavior.

- [x] **Step 2: Run the focused tests and confirm missing symbols**

  Run `go test ./app ./pkg/clicker`. Expected result: the new config/facade tests fail because their types and methods are not yet present.

- [x] **Step 3: Add the persisted rule field without persisting handles**

  Validate `FollowSync` on save/load through the same `ConfigStore`; keep the existing config version and existing profile serialization intact. Store only `Include` and `Exclude` arrays.

- [x] **Step 4: Implement the facade and picker event routing**

  Construct `windows.NewInputCapture`, `windows.NewKeySender`, and the existing `WindowPicker` for the core service. Route main completion to `SetMainTarget`, follow completion to `AddFollowTarget`, save rules after successful mutation, and emit typed state/error/pick events through the Wails context. On startup load rules only; leave main and follow handles empty.

- [x] **Step 5: Bind the new service and regenerate/check bindings**

  Add `FollowSync` to `App`, call its startup/shutdown methods, bind it in `main.go`, run `wails generate module` (or the repository-supported binding generation command), and run `go test ./app ./pkg/clicker`.

### Task 5: Build the React feature and register a non-breaking tab

**Files:**
- Create: `frontend/src/features/follow-sync/follow-sync.utils.ts`
- Create: `frontend/src/features/follow-sync/follow-sync.utils.test.ts`
- Create: `frontend/src/features/follow-sync/follow-sync-api.ts`
- Create: `frontend/src/features/follow-sync/use-follow-sync-store.ts`
- Create: `frontend/src/features/follow-sync/FollowSyncFeature.tsx`
- Modify: `frontend/src/app/AppShell.tsx`
- Modify: `frontend/src/app/app-shell.test.tsx`
- Modify: `frontend/src/styles.css`
- Regenerate: `frontend/wailsjs/go/FollowSyncService.*`, `frontend/wailsjs/go/models.ts` as generated by Wails

**Interfaces:**
- `follow-sync.utils.ts` owns frontend rule types, default empty snapshot, normalized display labels, and comma/tag parsing; it does not own runtime handles or native calls.
- The store subscribes to the five `follow-sync:*` events, keeps picker UI state in sync with native cancellation/errors, disables rule/target mutations while `state === 'running'`, and exposes only `start`/`stop` for the whole sync session.
- The feature renders main binding, a removable list of follow windows, include/exclude tag inputs, an explicit “排除优先” explanation, and a single main-owned start/stop control. Existing clicker tabs remain available.

- [x] **Step 1: Write failing utility and registry tests**

  Test rule parsing trims and deduplicates display entries, empty/default snapshots contain no bound windows, and `AppShell` exposes both `键盘连点器` and `主窗口同步` while preserving the clicker step labels.

- [x] **Step 2: Run the frontend tests to verify the new tab is absent**

  Run `pnpm --dir frontend test -- --runInBand` or the repository-supported Vitest command. Expected result: the new utility/registry assertions fail before the feature is added.

- [x] **Step 3: Implement the native API wrapper and store**

  Follow the existing `clicker-api.ts` availability pattern, map generated Wails models to local types, set picker UI state immediately on mouse down, commit bound targets only after successful native completion, clear stale picker state on cancellation/errors, and surface native errors with Ant Design `message`.

- [x] **Step 4: Implement the bright Ant Design feature UI**

  Use existing theme tokens and icons. Make “添加跟随窗口” repeatable, show each follower as data only, render disabled inputs/buttons while running, and explain that handles must be rebound after restart. Make picker intent explicit with hold-to-select/release-to-complete copy, yellow waiting state, green candidate state, and a cancel action. Do not add controls that can start/stop/cancel a follower independently.

- [x] **Step 5: Register the feature and run the frontend checks**

  Change the feature registry to include both tabs and corresponding sidebar menu entries, then run `pnpm -r test`, `pnpm -r typecheck`, and `pnpm -r build`.

### Task 6: Documentation, full verification, and boundary report

**Files:**
- Modify: `README.md`
- Inspect only: all files in `pkg/clicker`, `internal/platform/windows`, `app`, and `frontend`

- [x] **Step 1: Document the runtime-only target model and capture boundary**

  Update the feature list and directory description to state that synchronization uses a Windows low-level keyboard hook scoped to the selected foreground/root window, broadcasts only keyboard down/up transitions to follow windows via `PostMessageW`, excludes the SG Tools process, and does not guarantee observation of arbitrary application-internal or injected input that never reaches the low-level hook.

- [x] **Step 2: Run the complete requested verification matrix**

  Run `go test ./...`, `go vet ./...`, `pnpm -r test`, `pnpm -r typecheck`, `pnpm -r build`, and `wails build -clean` (or `wails build -o sg-tools-follow-sync-test.exe` when the default binary is locked). Record each exit code and any platform limitation explicitly.

- [x] **Step 3: Inspect the final diff and protected-file status**

  Run `git status --short`, `git diff --stat`, and `git diff --check`; verify no `Desktop/clicker` reference source or unrelated user file changed, no handles/running state are persisted, and the existing clicker APIs/tests remain present.
