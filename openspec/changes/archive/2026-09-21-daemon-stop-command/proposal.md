# Proposal: daemon-stop-command

## Why

Today the only way to stop the daemon is Ctrl+C in the terminal that started it, or manual `pkill torec` — there is no first-class way to stop a daemon running in another tmux pane/window. Worse, the shipped prompt-asset workflow explicitly instructs users to *"restart 'torec serve' to pick up the change"* (`assets.go`, `asset.go` advisories), but the tooling provides no stop half of that restart loop. `torec status` already frames the daemon as a managed entity ("✓ Daemon running on localhost:7331"); lifecycle management should be symmetric.

## What Changes

- Add a `torec stop` subcommand that gracefully stops a running daemon: read a pidfile, validate the PID belongs to a `torec` process, send TERM (SIGTERM on Unix; best-effort `taskkill /PID <pid>` on Windows), wait for the daemon to drain (health endpoint goes dark or port frees) up to a deadline, then clear the pidfile.
- `torec serve` writes the pidfile `<data-dir>/daemon.pid` at startup (data dir is `$TR_HOME` when set, else `~/.tr` — the retained data-layer naming) and removes it on shutdown. The pidfile must not conflict with concurrent lockless readers: a crashed daemon leaves a stale PID that `stop`/`status` treat as advisory, never as truth.
- Stale pidfile handling: if the PID is dead (or recycled to a non-torec process), `torec stop` clears the pidfile, prints a short advisory, and exits 0 (idempotent).
- `torec status` gains a one-line daemon-pidfile awareness: when the daemon is healthy it MAY print the owning PID from the pidfile; when unhealthy and a pidfile exists, it reports a stale-pid advisory.
- Windows support: `stop` uses `taskkill /PID <pid>`; PATH/TTY behaviors are irrelevant to this feature. Windows CI coverage: keep the new tests skip-unless-platform-consistent (pidfile semantics are cross-platform; signal sending is platform-specific).

## Capabilities

### New Capabilities

- `daemon-stop`: the daemon lifecycle contract — pidfile ownership, graceful shutdown via the CLI, stale-PID safety, and the cross-platform STOP mechanism.

### Modified Capabilities

None — the pidfile contract lives entirely in the new `daemon-stop` capability; existing config/loading behavior is unchanged.

## Impact

- **Code**: new `cmd/torec/stop.go` (cobra wiring + pid handling) with a `cmd/torec/stop_test.go`; small changes in `cmd/torec/main.go` (`root.AddCommand`, serve pidfile write/removal), `internal/config` (optional `DaemonPidPath()` helper next to `UserConfigDir`, `$TR_HOME`-aware), `internal/engine/server.go` (pidfile write/cleanup around `Start`), `internal/engine` health endpoint untouched.
- **Behavioral risk surface**: PID recycling — mitigated by process-identity validation (`ps`/comm check on Unix, `tasklist` on Windows) before signaling.
- **Compat**: pidfile is additive state; older daemons simply lack it, and `stop` degrades to a health-endpoint probe error ("daemon not running") rather than failing.
- **Windows**: `stop` is implemented via `taskkill /PID`; tests skip on platforms where pidfile probing is unreliable.
- **CI**: new unit tests run under existing `go test ./...`; no new workflow steps needed.
