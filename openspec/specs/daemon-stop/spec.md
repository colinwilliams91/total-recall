## Purpose

Give users first-class daemon lifecycle control: a pidfile-backed `torec stop` that gracefully stops a running daemon from any terminal, with stale-PID safety and cross-platform stop mechanics.

## Requirements

### Requirement: torec serve records its PID in a daemon pidfile
On startup, `torec serve` SHALL write the daemon's own process ID to `<data-dir>/daemon.pid` (the Total Recall data dir is `$TR_HOME` when set, else `~/.tr`). On graceful shutdown (SIGTERM/SIGINT drain completion, whether via signal or `torec stop`), `torec serve` SHALL remove that pidfile. A pidfile written by a previous run that crashed without cleanup is INERT state: subsequent commands treat a stale or absent pidfile as an advisory condition, never a hard error.

#### Scenario: Daemon writes pidfile at startup
- **WHEN** `torec serve` successfully binds to the daemon port and starts
- **THEN** `<data-dir>/daemon.pid` exists and contains the daemon's PID

#### Scenario: Daemon removes pidfile on graceful exit
- **WHEN** `torec serve` is running and receives a TERM signal (or the `torec stop` equivalent) and completes graceful shutdown
- **THEN** `<data-dir>/daemon.pid` no longer exists

#### Scenario: Stale pidfile after a crashed daemon
- **WHEN** the daemon process dies without cleanup and `torec stop` or `torec status` reads the leftover pidfile
- **THEN** the commands treat the PID as invalid/stale (see the stale handling requirement) — the pidfile alone is never a source of truth that fails a command

---

### Requirement: torec stop stops a healthy daemon gracefully
`torec stop` SHALL stop a running daemon by identifying the daemon process from `<data-dir>/daemon.pid`, sending the platform TERM signal, and waiting up to a bounded deadline for the daemon to drain in-flight requests and release its port. On a locally run daemon the behavior is equivalent to the daemon receiving SIGTERM directly (5-second graceful drain, in-flight hook/MCP requests served before stop). On success `torec stop` SHALL print a short confirmation and exit 0. When the daemon is not running at all (no pidfile or dead pid), `torec stop` SHALL print a "daemon not running" advisory and exit 0 — stopping a stopped daemon is not an error.

#### Scenario: Stop a healthy daemon
- **WHEN** `torec serve` is running and `torec stop` is invoked
- **THEN** the daemon begins graceful drain (in-flight requests complete), `/health` stops responding, the pidfile is removed, and `torec stop` exits 0 with a confirmation line

#### Scenario: Stop with no daemon running
- **WHEN** `torec stop` is invoked and no daemon is running (health endpoint unreachable AND no live pidfile pid)
- **THEN** `torec stop` prints a `daemon not running` advisory and exits 0

#### Scenario: Stop waits for in-flight requests
- **WHEN** `torec stop` fires while a hook-delivery request is mid-flight
- **THEN** the daemon completes the in-flight request within the drain window before exiting; `torec stop` does not kill the daemon before the drain window elapses

---

### Requirement: torec stop never kills a recycled PID
Before signaling, `torec stop` SHALL verify the pidfile PID actually corresponds to a `torec` process (via process identity inspection on the platform). If the PID has been recycled by an unrelated process, or no longer exists, `torec stop` SHALL treat the pidfile as stale: print a stale-pid advisory, remove the pidfile, exit 0, and NOT send any signal. The stop command MUST NOT signal any process it cannot confirm as this application's daemon.

#### Scenario: PID recycled to an unrelated process
- **WHEN** the daemon died, its PID was reused by another program, and the stale pidfile still contains that PID
- **THEN** `torec stop` identifies the mismatch, prints a stale-pid advisory, removes the pidfile, exits 0, and the unrelated process remains untouched

#### Scenario: PID no longer exists
- **WHEN** the daemon died and the PID is no longer in the process table
- **THEN** `torec stop` prints a stale-pid advisory, removes the pidfile, exits 0 — no signal is sent

---

### Requirement: Stop mechanism is cross-platform
On Unix, `torec stop` SHALL deliver SIGTERM to the verified daemon PID and await shutdown. On Windows, `torec stop` SHALL use `taskkill /PID <pid>` (best-effort equivalent of a graceful stop; Windows process termination does not honor POSIX signals, so the requirement is that the process ends and the pidfile/filesystem state reconcile). Shim details (exact process-inspection command) are implementation choices; the observable contract is: signal only verified own-PIDs, then confirm liveness is gone before claiming success (short bounded wait for `/health` to stop responding or the process to disappear).

#### Scenario: Stop on Linux/macOS
- **WHEN** a healthy daemon is running on Linux (`SIGTERM` honors graceful shutdown) and `torec stop` runs
- **THEN** the daemon drains and exits 0; `torec stop` confirms with no error output

#### Scenario: Stop on Windows
- **WHEN** a healthy daemon is running on Windows (`runtime.GOOS == "windows"`) and `torec stop` runs
- **THEN** the daemon process `torec.exe` is terminated via `taskkill`, `/health` stops responding, and `torec stop` exits 0

---

### Requirement: torec status surfaces the pidfile
`torec status` SHALL, when the daemon is healthy, MAY additionally show the daemon's PID from the pidfile for transparency ("running (pid 12345)"). When `/health` is unreachable but a stale pidfile exists, `torec status` SHALL report the stale-pid condition in the existing failure advisory (keeping exit code 1). Status output remains advisory only — a stale pidfile does not change the failure semantics.

#### Scenario: Healthy status shows the PID
- **WHEN** `torec status` runs while a healthy daemon holds a pidfile
- **THEN** the healthy line includes the daemon PID (e.g. `✓ Daemon running on localhost:7331 (pid 1234)`) or an equivalent PID disclosure

#### Scenario: Status with stale pidfile and down daemon
- **WHEN** the daemon is not running and a stale pidfile exists
- **THEN** `torec status` prints its existing `daemon not running` failure (exit 1) and a short note that a stale `daemon.pid` was ignored/cleaned
