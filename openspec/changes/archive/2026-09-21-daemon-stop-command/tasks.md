## 1. Pidfile plumbing

- [x] 1.1 Add `config.DaemonPidPath()` (or equivalent helper alongside `UserConfigDir`): returns `<data-dir>/daemon.pid` where data dir is `$TR_HOME` when set else `~/.tr`; respects existing 0700-dir creation. Verify via unit tests in `internal/config` (`t.Setenv("TR_HOME", ...)` pattern).
- [x] 1.2 Wire pidfile write into `serve`: after successful bind in `engine.Server.Start()`, write PID to `config.DaemonPidPath()`; remove after drain completes. Failure to write/removal SHALL log an advisory but not abort serve. Verify: integration test starts daemon via `startTestDaemon`, asserts pidfile exists with the live PID and is removed on shutdown.
- [x] 1.3 Ensure Ctrl+C path also removes the pidfile (cleanup in the drain goroutine, before process exit). Verify: send SIGINT to a test daemon process, assert pidfile gone.

## 2. torec stop command

- [x] 2.1 Create `cmd/torec/stop.go`: cobra `stopCmd()` reading `config.DaemonPidPath()`, identity-probing the PID, signaling (SIGTERM / `taskkill /PID`), bounded liveness wait, pidfile cleanup, and the four outcomes (healthy stop / no daemon / stale pid / unexpected error). Wire into `root.AddCommand` in `main.go`. Verify: `go build ./...`
- [x] 2.2 Implement Unix identity probe (ps/procfs comm check) and Windows `tasklist` name check per design D2; indeterminate probe results MUST be treated as stale. Verify: unit tests with fake pidfiles.
- [x] 2.3 Add tests in `cmd/torec/stop_test.go` covering: healthy stop (use a started test daemon), stop with no pidfile → advisory exit 0, stale pid (dead pid) → advisory + pidfile removed + exit 0, recycled pid (spawn a `sleep`-equivalent process — identity mismatch) → no signal sent + advisory + exit 0. Verify: `go test ./cmd/torec/ -run TestStop`.
- [x] 2.4 Exercise the end-to-end stop on Unix in an integration test: start daemon, POST a slow request, invoke stop logic, assert in-flight completes within drain window, pidfile cleared, `/health` fails afterwards. Verify: `go test ./cmd/torec/ -run TestStopIntegration`.

## 3. torec status pidfile surfacing

- [x] 3.1 Extend `runStatus()`: when healthy and pidfile readable, append `(pid <pid>)` to the healthy line; when unreachable and a stale pidfile exists, append a stale-pid note to the failure advisory (exit code unchanged). Verify: `go test ./cmd/torec/ -run TestStatus`.
- [x] 3.2 Update status tests asserting the new output bytes; ensure golden/view tests unaffected. Verify: `go test ./...`.

## 4. Docs

- [x] 4.1 README setup block: add `torec stop` next to `torec serve`; note the restart clause for prompt-asset overrides ("restart 'torec serve'" → now doable as `torec stop && torec serve`).
- [x] 4.2 `docs/ARCHITECTURE/DAEMON/INDEX.md` + AGENTS.md daemon line: document the pidfile (`<data-dir>/daemon.pid`) and stop semantics.
- [x] 4.3 CHANGELOG: add `[Unreleased] → Added` entry for `torec stop`/pidfile.
- [x] 4.4 Verify docs grep: `grep -rn "restart 'torec serve'" --include='*.go' --include='*.md'` sites remain consistent (stop mention optional; no stale `tr` regressions).

## 5. Final verification

- [x] 5.1 Full gate: `go build ./... && go vet ./... && go test ./...` green; golden view tests unaffected.
- [x] 5.2 Manual smoke on Linux: start daemon (background), `torec status` shows `(pid ...)`, `torec stop` drains it, pidfile gone, `command -v tr` still `/usr/bin/tr`.
