## Why

The `synthesize-from-context` change shipped a runtime override path: a file at `$TR_HOME/prompts/<name>.md` takes precedence over the `//go:embed`-ed default. This is a deliberate product decision — the dev loop and the user loop are the same loop; the policy doc is research strategy, not vendor lore; users with domain-specific learning goals (a game dev who wants more ECS questions, a DBA who wants more query-plan questions) should be able to override the shipped default without recompiling.

Without observability, three failure modes emerge in production:

1. **Drift / silent rot** — a user overrides once in week one, then nine months later ships a new research-backed canonical doc and adaptive resolver that their stale override knows nothing about. Questions stay quality-frozen at the old policy; the user perceives regression instead of upgrade. There is no signal the override is stale.
2. **Customization-induced regressions** — a user pastes `Always ask about React hooks.` into `~/.tr/prompts/...`, every question becomes one-note, and they open an issue blaming Total Recall. The startup log line is buried among hundreds of daemon logs; invisible to the user.
3. **Liability framing** — there is no UI surface to show "the user overrode the shipped default." When a user's custom override produces bad questions, the user's mental model is "Total Recall is bad," not "my override is bad." Compare to VS Code's layered settings: bad `settings.json` colors are visibly the user's, because the UI surfaces what's overridden.

The override mechanism is right; the override mechanism *alone* is what's risky. The fix is observability, not gating — keep the mechanism open and symmetric; ship duty-of-care alongside it.

## What Changes

**Modified capability `prompt-asset-loading`** — startup logging surface:
- When `assets.Load` resolves an override, the daemon log SHALL emit the resolved override path, the embedded-default's compile-time source, and the mtime delta between the two. When the override's mtime is older than the embedded default's build by more than a tunable threshold (default 90 days), the log line SHALL escalate to `OVERRIDE WARNING: <path> is <N>d older than the embedded default — re-sync with 'tr asset sync'`. Drift becomes loud, not silent.

**New capability `cli-asset-commands`** — three `tr asset` subcommands:
- `tr asset list` — enumerates every prompt asset known to the daemon, showing its resolved source (`embedded` | `$TR_HOME`/path | `fallback`), the resolved path, and a human-readable age (e.g., `6mo old`, `2d old`). Output is plain text suitable for `tput` / pipable to `grep`. Defaulting to plain prose per the terminal-first UX convention.
- `tr asset reset [<name>]` — removes the override file at `$TR_HOME/prompts/<name>.md` so the next daemon start falls back to the embedded default. With no `<name>` argument, removes every override. Prompts for confirmation when more than one override is in scope (interactive TTY only). Single-shot recovery path: "run `tr asset reset` and see if your questions improve" — equivalent to "kill your browser extensions" debugging.
- `tr asset sync [<name>]` — copies the current embedded default to `$TR_HOME/prompts/<name>.md` (overwriting an existing override). The reverse of `reset`: instead of discarding the override slot, refresh it with the canonical current content so the user has a starting point for re-tuning. After `sync`, the override mtime is current, the drift warning falls silent, and the user can iterate on a known-good baseline.

**Modified capability (TBD per implementation)** — `tr config show` extension:
- The `tr config show` output already lists config-source per key (`difficulty: adaptive  # [default]`). It SHALL additionally print one line per resolved prompt asset: `policy: <resolved-path>  # [<source>], <age>`. The line lives under a new "prompt assets" section of the show output. Surfaces the override in the obvious place — a user who runs `tr config show` while debugging sees the override immediately, not buried in daemon logs.

## Capabilities

### New Capabilities
- `cli-asset-commands`: `tr asset list`, `tr asset reset [<name>]`, and `tr asset sync [<name>]` subcommands. Operate on the `$TR_HOME/prompts/` directory; touch no embedded bytes; no daemon restart needed for `list` (it reads the cache state on disk) but required after `reset`/`sync` (the daemon's in-memory cache is not invalidated by file mutations — restart picks up the change, per the existing `synthesize-from-context` cache model).

### Modified Capabilities
- `prompt-asset-loading`: `assets.Load` emits a structured startup log line including the override path, embedded source path, and mtime delta. When delta exceeds a threshold (default 90 days, configurable as `prompt-asset.drift-warning-days` user config), the line escalates to `OVERRIDE WARNING`.
- (TBD — depends on the canonical `tr config show` capability name in the existing spec tree): `tr config show` prints a "prompt assets" section listing resolved sources and ages.

## Impact

- **Code:** `assets/assets.go` (starter log line enriched with mtime computation; export a `LoadAll() []PromptAsset` for `tr asset list`); `cmd/tr/asset.go` (new file — Cobra `assetCmd` + `listCmd`/`resetCmd`/`syncCmd`); `cmd/tr/main.go` (register `assetCmd`); `internal/config/show.go` (new "prompt assets" section); `internal/config/config.go` (new optional `PromptAsset.DriftWarningDays` field, default 90).
- **Tests:** `cmd/tr/asset_test.go` — table tests for `list` output shape, `reset` removes file, `reset` is a no-op (and exits 0) when no override exists, `sync` copies embedded to `$TR_HOME/prompts/`, `sync` overwrites an existing override; `assets/assets_test.go` — `TestLoadWarnsOnStaleOverride` asserts the warning log line fires when mtime delta > threshold; `TestLoadNoWarningWhenRecent` asserts the warning is silent within threshold; `cmd/tr/config_test.go` — `TestConfigShowListsPromptAssets` asserts the new section appears with expected keys.
- **APIs:** New `tr asset list|reset|sync` subcommands. No external API changes (CLI additions only). Daemon HTTP surface unchanged.
- **Dependencies:** None new.
- **Specs:** This change adds `openspec/specs/cli-asset-commands/spec.md` (new capability) and updates `openspec/specs/prompt-asset-loading/spec.md` (MODIFIED requirements for the startup log and warning threshold).

## Key Design Decisions

- **`reset` and `sync` are file-ops, not daemon-ops.** Both commands manipulate `$TR_HOME/prompts/` directly. Neither calls the daemon, neither requires the daemon to be running, neither invalidates the daemon's in-memory cache. The user must `tr serve` restart after to pick up the change — exactly as with manual editing. This preserves the existing `synthesize-from-context` cache-invalidation model (restart is the invalidation mechanism) and avoids inventing a live-reload subsystem.

- **`sync` is the complement of `reset`, not a duplicate.** `reset` removes the override file, leaving the slot empty; `sync` populates the slot with the canonical current content. Both are one-shot operations, both silence the drift warning (one by emptying the slot, one by refreshing the mtime). The two exist because they support different mental models: "I want to go back to defaults" (`reset`) vs "I want to start tuning again from a known-good baseline" (`sync`).

- **Drift threshold is configurable, not hardcoded user-constant.** `prompt-asset.drift-warning-days` defaults to 90 (matches the threshold used in the design doc's "180d/90d" example). Research-driven users iterating monthly might lower it to 30; conservative users might raise it to 365. Captured in `~/.tr/config.yaml` per the existing two-tier config pattern.

- **Warning is logged, not blocking.** The startup warning never refuses to start the daemon and never overrides the user's file. It is purely informational. A user who deliberately keeps an old override (because they tuned it for their domain and the canonical updates don't apply) can ignore the warning or raise the threshold.

- **`list` is plain text, not JSON.** The terminal-first convention applies. A future JSON output flag (`--json`) can be added if MCP or scripting needs it; not built speculatively.

- **No file-watcher, no live reload.** The `synthesize-from-context` change deliberately chose a process-lifetime cache. This change keeps that decision — `list` reads disk state (one stat per known asset, no cache), but `reset`/`sync` do not signal the daemon. Live-reload is a follow-up if the restart-tax becomes a real friction point.

## Non-Goals

- Live reload / file-watching eviction of the in-memory cache — the `synthesize-from-context` model is "restart to invalidate." Live reload is a follow-up.
- `tr asset diff` showing the textual diff between an override and the embedded default — a useful follow-up, but not blocking; users who want this can run `diff <(tr asset show-embedded <name>) <name>.md` or similar.
- `tr asset show-embedded` subcommand emitting the embedded bytes to stdout — useful for piping; deferred. Users can extract the canonical content from the source repository for now.
- `--json` flag on `tr asset list` — plain text ships first; structured output is a follow-up if MCP or scripting needs it.
- A TUI picker for `reset`/`sync` across multiple overrides — overkill at current override-count scales (likely 1-2 per user). Plain confirmation prompt is enough.
- Backward migration — the override mechanism is additive from `synthesize-from-context`; there is no pre-existing override to clean up.
- Validation of override content (e.g., warning if the override file is empty or only contains front-matter) — the existing `parseAsset` already handles empty bodies by returning `Body: ""`, which triggers the `synthesize-from-context` fallback template path. No new validation needed.