## Context

The daemon (`torec serve`) is a foreground Go process with SIGTERM/SIGINT graceful shutdown (5-second drain, `internal/engine/server.go:433`). No pidfile exists; there is no stop command. The data dir (`$TR_HOME` when set, else `~/.tr`) is the retained data-layer home and already hosts `config.yaml` + `memory.db` — it is the natural, `$TR_HOME`-aware location for `daemon.pid`. `torec status` shells a 1-second `/health` probe (`cmd/torec/main.go:510`). goreleaser ships windows builds, so Windows must be considered, though core flows were designed Unix-first.

## Goals / Non-Goals

**Goals:**
- Users can stop the daemon from any terminal with `torec stop`, equivalent in cleanliness to Ctrl+C on the canonically running daemon.
- Crash-safety: stale pidfiles never cause hard failures and never risk killing unrelated processes.
- Fold the "restart to pick up changes" loop (prompt-asset advisories) into actual tooling.

**Non-Goals:**
- Autostart/service-manager integration (`launchd`/`systemd`/Task Scheduler) — that's the existing ROADMAP daemon-autostart item; if/when it lands, `stop` remains valid because the manager also uses signals.
- Locking/multi-daemon arbitration: the port already binds-exclusive; a second `serve` fails at bind time with the existing port-conflict message. The pidfile records the *successful* binder.
- Changes to `/health`, hook routes, MCP, or config schema.
- Making the daemon a Windows service.

## Decisions

**D1: pidfile lives in the data dir (`<data-dir>/daemon.pid`), not `$XDG_RUNTIME_DIR` or `/run`.**
The data dir already handles `$TR_HOME` test isolation and is the established default-seeded home (created 0700). Runtime dirs (`/run`/`/var/run`) don't reflect `$TR_HOME` and complicate Windows.
Alternative rejected: rely on port-scanning to find the daemon manager (fragile, not identity-safe).

**D2: Identity check before signaling (PID-recycling defense).**
On Unix: `os/SysProcAttr`-free approach — read `/proc/<pid>/comm` (Linux) or use `ps -p <pid> -o comm=` as the identity probe; Windows: `tasklist /FI "PID eq <pid>"` and match the `torec.exe` image name. If anything is uncertain (probe fails mid-check), treat as stale.
Alternative rejected: signaling the pidfile PID unvalidated — could kill an unrelated recycled process (worst-case failure class).

**D3: Best-effort, tolerance-first CLI semantics.**
`stop` exits 0 for: healthy stop, no daemon, stale pid (with advisory). Exit 1 reserved for unexpected system errors (pidfile location unresolvable, HEALTH probe exception that isn't connection-refused, signal permission error). Matches the no-surprise advisory style already used by `torec status`.

**D4: `serve` writes pidfile after successful bind, removes on drain exit.**
Write after `net.Listen` succeeds — a failed bind must not leave a misleading pidfile. Remove after drain completes (post `wg.Wait()`); the drain goroutine handles signal-based exits so Ctrl+C path cleans up identically to `torec stop`.

**D5: `stop` avoids silly kill races by waiting for liveness to end (bounded ~5.5s) rather than fire-and-forget signaling.** The wait observes either `/health` failure or pid disappearance, then removes the pidfile. Deadline expiry prints an advisory (not exit 1; the drain is in flight) — exit 0 to match the non-fatal contract... unless the drain window elapses unexpectedly, in which case a `force kill` fallback would be a new decision; not included in this change (documented, not built).

## Risks / Trade-offs

- [PID probe commands vary between Unixes (procfs vs ps)] → any probe error yields "indeterminate", which is always treated as stale — fail-safe: stop declines to signal rather than risk an unrelated process.
- [Multiple torec daemons across `$TR_HOME` values on one machine] → the pidfile is per-data-dir; each daemon writes its own. Stop targets the pidfile of the data dir resolved in the *invoking* environment — appropriate for a per-user tool.
- [Windows `taskkill` ends the process without a drain hook] → documented best-effort; observable contract is "process ends, health goes dark, exit 0".
- [Pidfile in `~/.tr` may confuse users expecting no dotfiles] → hidden naming + docs; no new dirs.
- [`stop` run in a container vs daemon in same? no system-d context] — pidfile is the single source; containerized/other-namespaced daemon uses the same file and works as the same-lifecycle flow.

## Migration Plan

No user migration: additive pidfile state. Daemons upgraded mid-run simply lack it; `stop` in that case degrades to "no pidfile → 'daemon not running' advisory". Old and new clients coexist harmlessly since the OH health probes are the authoritative liveness test.

## Open Questions

None — Windows `taskkill` best-effort was confirmed by the user at proposal time.
