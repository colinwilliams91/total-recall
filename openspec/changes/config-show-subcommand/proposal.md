## Why

The CLI's verb grammar has one hybrid: `tr config --show` carries its verb in a flag while every verb-shaped surface is a subcommand (`tr asset show`, top-level verbs `tr ask/init/repo/serve/status`). The split was sequenced into existence, not decided — `config` was conceived as a read/write shell, only `--show` was ever implemented, and the `asset` noun arrived later with proper verb subcommands. The diagnostic tell: `tr config` with no flags prints help, because a command whose verb is a flag has no meaningful bare form.

Unifying direction (agreed in exploration): **nouns get verb subcommands; flags are modifiers** — never carriers of verbs. `tr config show` is the fix; `--show` survives as a hidden deprecated alias so existing scripts and muscle memory (real users on v0.4.x) keep working while docs move over. Also expands compatibility with the plurality scrub (FEATURES.md is freshly singular; this change keeps the inspect surface consistent with `tr asset show`).

## What Changes

- **New: `tr config show`** — a subcommand of `config` running exactly what `--show` runs today (`runConfigShow`); no args; prints the resolved config with source annotations plus the `prompt assets:` section.
- **Deprecated: `tr config --show`** — the flag keeps working bit-for-bit but is hidden from help/usage and prints a one-line deprecation notice: `note: 'tr config --show' is deprecated — use 'tr config show'` (suppressed by `--quiet`). Removal is not scheduled.
- **Bare `tr config` unchanged** — still prints the command help (now listing the `show` subcommand).
- **Docs sweep** — the four `tr config --show` mentions (README Configuration section, FEATURES.md observability section, AGENTS.md, single-policy change prose if any) switch to `tr config show`.

## Capabilities

### New Capabilities

- (none)

### Modified Capabilities

- `config-override-chain`: ADDED requirement covering the `tr config show` subcommand form and the `--show` deprecation semantics. The canonical "Resolved config is inspectable" requirement (whose scenario references `tr config --show`) remains true — the flag form is deprecated, not removed, so no REMOVED/MODIFIED churn is needed; ADDED ops compose in either archive order against the three unarchived changes.

## Impact

- **Code:** `cmd/tr/main.go` (`configCmd` gains a `show` subcommand; the `--show` flag stays working, marked hidden, emitting the deprecation notice when used).
- **Tests:** `cmd/tr/config_test.go` or new — `tr config show` renders the resolved config (asserts the `prompt assets:` section and `[user]` tags); `--show` still works and emits the deprecation notice (suppressed under `--quiet`); hidden flag does not appear in `tr config --help` output.
- **Docs:** `README.md` (1), `FEATURES.md` (1), `AGENTS.md` (1) mentions.
- **Specs:** `openspec/changes/config-show-subcommand/specs/config-override-chain/spec.md` (ADDED requirement only).
- **Dependencies:** none. **BREAKING:** none — the flag form still functions; only its discoverability is retired.

## Key Design Decisions

- **Verb-carrying via subcommand, flags as modifiers.** `--show` as a flag cannot take positional arguments and caps `config` at one verb; `tr config set` becomes trivially addable later. This matches `git remote add` / `docker container ls` grammar and the `tr asset show|reset|sync` shape this codebase just adopted.
- **Deprecated alias, not removal.** The Canonical `config-override-chain` scenario (`tr config --show`) stays satisfied; a removal decision is deferred and would ride a future change with its own REMOVED block.
- **Deprecation notice respects `--quiet`.** The notice is an advisory; the `--quiet` persistent flag exists precisely to suppress advisories.

## Non-Goals

- A `config set` / `config get` subcommand — Headroom is deliberately left in the namespace shape; verbs are added when real.
- Any output-format change to the resolved-config rendering (the `prompt assets:` section and ANSI styling are untouched).
- Removing `--show` outright.

## Developer Workflow Impact

Zero commit-time or daemon-loop impact. Purely a CLI surface nicety plus doc consistency; no transactional behavior changes anywhere.

## Open Questions

- None.
