## Context

The executable `tr` is produced by `go install ./cmd/tr` (name comes from the package directory), `go build -o tr` (Makefile, name from `BINARY_NAME`), and goreleaser (`binary: tr`). A full inventory (see the prior exploration and `docs/handoff/Upstream-Bug-Fix-Handoff_ Linux-tr-Command-Name-Collision.md`) found: ~50–60 user-visible `tr ...` invocation strings in Go, hook-template consts in `internal/hooks/scripts.go` mirrored by documentation copies in `hooks/`, PATH detection in `cmd/tr/pathdetect.go` performing `exec.LookPath("tr")`, ~230 doc references, e2e tooling, and seven affected openspec capabilities. A previous rename (`total-recall` → `tr`, archived 2026-07-24) set the precedent for the mechanical shape of this change and for shipping no compatibility alias.

## Goals / Non-Goals

**Goals:**
- A clean Linux install of Total Recall cannot shadow `/usr/bin/tr` — enforcement, not just intent (CI + tests).
- PATH detection reflects the new name and flags a stale legacy `tr` install of *this* app.
- Existing users get an explicit, documented migration step, not silent breakage.

**Non-Goals:**
- Renaming the Go module path, `internal/*` import paths, daemon port, or MCP mount.
- Renaming data-layer identifiers: `~/.tr/`, `.tr.yaml`, `$TR_HOME`, `trDir`, `config.UserConfigPath` — they are data state, not the executable.
- Renaming the hook sentinel `# total-recall managed` or the `[total-recall]` advisory prefix (product labels; renaming breaks reinstall detection).
- Renaming capability directories of specs beyond what the delta format supports (openspec forbids moving capabilities in deltas). Requirement/scenario header *titles* stay historical; body text is corrected at archive time.
- Renaming archived openspec changes or handoff docs (frozen history).

## Decisions

**D1: New executable name is `torec`.**
Rationale: short (like the ergonomic `tr`), no collisions with coreutils or other common utilities, and phonetically evokes "Total Recall". Alternatives: `total-recall` (unambiguous but long — was renamed away precisely for ergonomics), `trec` (reads as "tape REC"). User decision made at proposal time.

**D2: Move `cmd/tr/` → `cmd/torec/` rather than renaming only build outputs.**
Rationale: `go install` names the binary after the package directory; keeping `cmd/tr/` would leave `go install ./cmd/tr` producing a `tr` binary — failing acceptance criterion 1 of the handoff. The `go install` URL becomes `github.com/colinwilliams91/total-recall/cmd/torec@latest`.
Alternative rejected: a separate flip-flop package — adds indirection for no gain.

**D3: PATH detection becomes two-stage.**
Stage 1: `exec.LookPath("torec")` on Unix / `Get-Command torec` on Windows — silently proceed when found, warn (GOPATH PATH-fix one-liner) when not.
Stage 2 (legacy guard): if `tr` is on PATH, run `tr --version` and classify: a Total Recall version format ("tr version dev"/`tr version <semver>` matching our format) means a stale legacy install shadowing coreutils → warn to delete it; a coreutils version string (or anything else) → silent. Timeout the child process (short context) so a pathological `tr` cannot hang `torec init`.
Alternative considered: skip stage 2 (not in the original spec) — kept, because the incident shows users won't diagnose shadows themselves, and stale installs from before the upgrade will be common.
Helper renamed `checkTrOnPath` → `checkTorecOnPath`; file `pathdetect.go` moves with the directory.

**D4: Hook bodies keep the managed sentinel; invocation strings inside them change.**
Hook bodies are compile-time consts written at `torec repo` time. Old managed hooks on disk still say `tr serve` etc. After upgrade the user re-runs `torec repo`; the sentinel (name-free) is honored so the installer overwrites in place. The post-commit hook's baked absolute path keeps working until the binary path changes — after a rebuild at a different path it fails loudly per the existing spec, fixed by re-running `torec repo`.

**D5: `Take` enforcement over `trust` for the executable name — CI guard plus unit test.**
- Unit test: `buildPostCommitHookScript`, goreleaser config parsing, and Makefile output assert `torec` and refuse `tr` (a static guard on config files).
- Linux collision-regression test (skipped on Windows): put the built binary's dir first on PATH, run `tr '[:lower:]' '[:upper:]'`, assert the system utility output — only meaningful when run where coreutils exists; guarded by `runtime.GOOS` and a coreutils presence check so Windows/macOS CI is unaffected.

**D6: Spec delta mechanics.**
MODIFIED requirement blocks keep original requirement and scenario titles verbatim (archive matching keys on them); body text is corrected. The new stale-legacy guard is an ADDED requirement under `path-detection-warning`. The new `executable-naming` capability carries the install-name contract.

## Risks / Trade-offs

- [Missed string site slips through, leaving a stale `tr` reference] → complete grep sweep (`\btr\b` in Go strings, hooks, docs, scripts) is its own task with a clean-grep acceptance check; CI guard catches the dangerous subset (binary name).
- [Users with installed hooks feel breakage after upgrading] → migration note in README + CHANGELOG post-commit advisory message updated to mention re-run; sentinel keeps overwrite path clean.
- [post-commit baked path dies when the binary is replaced at a new path] → existing loud-failure behavior; re-running `torec repo` is the documented remedy.
- [Windows `Get-Command torec` behavior] → same mechanism as today with the new name; covered by existing pathdetect tests in name-swapped form.
- [Golden files affected if cobra error help text shows "tr"] → no current golden contains the binary name (verified); golden tests run in CI to catch drift.
- [e2e PowerShell asserts on hook content containing `tr.exe ask`] → updated in the same task bucket; noted as the one place Windows-side expectations change.

## Migration Plan

1. Land the rename in one PR: directory move + config/string sweep + tests/CI.
2. Users upgrading: rebuild/install (`go install .../cmd/torec@latest`), delete their stale `tr` binary, re-run `torec repo` in existing repos. Documented in README and as a CHANGELOG entry.
3. Rollback: revert the commit; no data-format or on-disk state is changed by the rename (hooks refresh on next run).

## Open Questions

None — the executable name (`torec`), directory move, and no-compat-alias decisions were settled with the user at proposal time.
