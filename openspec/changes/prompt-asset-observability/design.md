## Context

The `synthesize-from-context` change shipped a runtime override path for prompt assets: a file at `$TR_HOME/prompts/<name>.md` takes precedence over the `//go:embed`-ed default in `assets.Load`. This is a deliberate product decision — the policy doc is research strategy, not vendor lore; users with domain-specific learning goals should be able to override without recompiling.

The override is currently invisible in production. `assets.Load` emits one startup log line (`[assets] prompt asset %q loaded from override at %s`) buried among hundreds of daemon logs. There is no surface in `tr config show`. There is no recovery command — a user who pastes a bad override and gets bad questions for a week has no path to "undo my customization" other than remembering where they put the file and deleting it manually.

## Goals / Non-Goals

**Goals:**
- Surface the resolved prompt-asset path in `tr config show` so a debugging user sees the override in the obvious place, not buried in daemon logs.
- Enrich the daemon startup log with the override's mtime relative to the embedded default's build, and escalate to a visible `OVERRIDE WARNING` when the override is stale beyond a threshold (default 90 days, configurable).
- Ship `tr asset list`, `tr asset reset [<name>]`, and `tr asset sync [<name>]` as the recovery / re-baseline path. `reset` discards an override; `sync` refreshes an override slot with the canonical current content.
- Make drift loud, not silent — a user nine months past a one-off override should be *told* their override is stale, not asked to remember.

**Non-Goals:**
- Live reload / file-watching cache invalidation — the `synthesize-from-context` model is "restart to invalidate"; preserved.
- `tr asset diff` showing textual diff between an override and the embedded default — useful follow-up, not blocking.
- `tr asset show-embedded <name>` subcommand emitting embedded bytes — useful for piping; deferred.
- `--json` flag on `tr asset list` — plain text ships first; structured output is a follow-up if MCP or scripting needs it.
- TUI picker across multiple overrides — overkill at current scale (likely 1-2 per user).
- Validation of override content (warning if the file is empty or contains only front-matter) — the existing `parseAsset` already handles empty bodies via the `synthesize-from-context` fallback template; no new validation needed.

## Decisions

### Decision: `reset` and `sync` are file-ops, not daemon-ops

Both commands manipulate `$TR_HOME/prompts/` directly via filesystem operations. Neither calls the daemon, neither requires the daemon to be running, neither invalidates the daemon's in-memory cache. The user must `tr serve` restart to pick up the change — exactly as with manual editing.

**Alternatives considered:**
- *Signal the running daemon to reload.* Rejected: the `synthesize-from-context` cache model is "restart to invalidate." Introducing a signal/reload path here would invent a live-reload subsystem, scope creep beyond observability.
- *Touch a sentinel file the daemon stats per call.* Rejected: re-introduces per-call disk IO that the `//go:embed`+cache design deliberately eliminated.

### Decision: `sync` is the complement of `reset`, not a duplicate

`reset` removes the override file (slot becomes empty, defaults load next start). `sync` populates the slot with the canonical current content (mtime becomes current, drift warning silenced, user has a re-tuning baseline). Both are one-shot file ops. Two commands exist because they support different mental models: "I want to go back to defaults" (`reset`) vs "I want to start tuning again from a known-good baseline" (`sync`).

**Alternatives considered:**
- *Ship only `reset`; users can copy from the source repo to re-baseline.* Rejected: requires the user to know where the source repo is and find the embedded file. `sync` closes that loop in one command — the binary already has the embedded bytes, just writes them to disk.

### Decision: Drift threshold is configurable, default 90 days

`prompt-asset.drift-warning-days` in `~/.tr/config.yaml` controls the threshold. Default 90 (matches the "180d/90d" example in the design narrative). Research-driven users iterating monthly might lower to 30; conservative users might raise to 365.

**Alternatives considered:**
- *Hardcoded 90.* Rejected: diverges from the existing two-tier config pattern; users who deliberately keep a tuned old override would have no way to silence a warning they understand.

### Decision: Warning is logged, not blocking

The startup warning never refuses to start the daemon and never overrides the user's file. Purely informational. A user who deliberately keeps an old override (because they tuned it for their domain and the canonical updates don't apply) can ignore the warning or raise the threshold.

**Alternatives considered:**
- *Block daemon startup until the user reconciles.* Rejected: hostile to the dev loop. The override is *intentional* product behavior, not an error condition.

### Decision: `list` is plain text, not JSON

The terminal-first UX convention applies. A future `--json` flag can be added if MCP or scripting needs it; not built speculatively.

**Alternatives considered:**
- *JSON-default.* Rejected: violates the terminal-first convention; speculative until a scripting consumer exists.

### Decision: `tr config show` extends with a "prompt assets" section

The existing `tr config show` output lists config-source per key. A new section prints one line per resolved prompt asset: `policy: <resolved-path>  # [<source>], <age>`. The section lives at the end of the show output to keep the existing config scan visually compact.

**Alternatives considered:**
- *A separate `tr asset status` command.* Rejected: `tr config show` is already where users debug "what is the daemon configured to do"; surfacing the override in a separate command forces a second command for a debugging session that should be one.

## Risks / Trade-offs

- **[Trade-off] `reset` and `sync` require a daemon restart to take effect.** Documented in command help text. Mitigation: the commands log a clear `[assets] restart 'tr serve' to pick up the change` advisory on success when a daemon is reachable.
- **[Trade-off] Mtime is a weak staleness signal.** A user who touches the file with `touch` resets the warning without actually re-syncing content. Acceptable: the warning is informational, not a hard guard; a user gaming `touch` is a user who already knows what they're doing.
- **[Risk] `reset` with no args could surprise a user who didn't realize they had multiple overrides.** Mitigated by confirmation prompt in interactive TTY when >1 override is in scope; in non-TTY use, an explicit `--all` flag is required to remove more than one.
- **[Risk] `sync` could overwrite an override a user spent hours tuning.** Mitigated by `--force` required when the target exists and is non-empty; default behavior is to refuse with a message pointing at `reset` first if they really want a clean baseline.

## Migration Plan

1. Additive change — no schema migration, no wire change, no daemon-restart required to *receive* the upgrade.
2. Implement `assets.go` mtime computation + warning log; add `LoadAll()` for `list`.
3. Implement `cmd/tr/asset.go` with `list`, `reset`, `sync`; register in `main.go`.
4. Extend `internal/config/show.go` with the "prompt assets" section.
5. Tests for each command's happy path and edge cases.
6. No `openspec/specs` canonical sync until this change archives.

## Open Questions

- Should `sync` prompt the user for confirmation when the target file is non-empty, or require an explicit `--force`? Spec currently mandates `--force` when the target exists and is non-empty. Implementation may surface this as friction for the most common case (user runs `sync` for the first time, no file exists yet — happy path, no `--force`); the friction lands only on the reload-after-tuning case, where `--force` is a deliberate "I am about to discard my tuning" affordance. Captured as a tasks.md note; if the implementation reveals it's the wrong default, flip during implementation.
- Should `list` show the embedded default's content size alongside the override's? Useful for `diff` mental math; not blocking. Defer until `diff` lands.