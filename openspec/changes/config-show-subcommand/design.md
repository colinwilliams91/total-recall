## Context

The verb-grammar unification decided in exploration: nouns carry verb subcommands, flags modify. `tr config --show` is the sole verb-as-flag in the CLI, a sequencing artifact of `config`'s imagined get/set future. The discussion weighed and rejected the "3rd-position gets `--`" rule (it puts flags on the verb layer; flags cannot take positional arguments and cap nouns at one verb) in favor of subcommand verbs.

## Goals / Non-Goals

**Goals:**
- `tr config show` exists and produces byte-identical output to the `--show` path.
- `--show` keeps working for existing scripts/muscle memory, hidden from help, with a `--quiet`-aware deprecation notice.
- Doc surface says `tr config show` everywhere.

**Non-Goals:**
- `config set`/`get`; output-format changes; flag removal.

## Decisions

### Decision: Additive unification — subcommand now, flag as hidden alias

Both forms route to the same `runConfigShow()`. The alias path is not silently broken: hidden + notice teaches the new form at the moment of use, which is friendlier than a hard break and consistent with how the CLI already advises (`/notes` pattern: restart advisories, OVERRIDE WARNING). Flag removal is deliberately left unscheduled — when it ever happens it gets its own REMOVED delta with Reason/Migration, not a ride-along.

**Alternatives considered:**
- *Hard-remove `--show`.* Rejected: zero live-user cost today, but zero benefit either — and real users on v0.4.x have it in muscle memory.
- *The "3rd-position `--`" rule (user's original lean, explicitly floated then retracted into this decision).* Rejected: flags-for-verbs cannot express `reset <name>` / `sync --force` arg shapes and would make `asset`'s trio inexpressible; subcommand-verbs is the only direction that scales.

### Decision: `show` is `Args: cobra.NoArgs`

Matches the other verb subcommands (`asset show`) and prevents `tr config show extra` from printing config plus silently swallowing a typo.

## Risks / Trade-offs

- **[Trade-off] Two render paths exist for one output** (subcommand + deprecated flag). Mitigated: both call `runConfigShow()` — zero duplication; the flag path exists only for compat until someone schedules its removal.

## Migration Plan

1. `configCmd`: add `show` subcommand; keep `--show` flag (hidden) + notice.
2. Tests: subcommand output parity, notice behavior, `--quiet` suppression, hidden in help, NoArgs rejection.
3. Docs sweep (4 mentions).
4. Build/vet/test/validate + manual (`tr config show` shows; `tr config --help` hides `--show` but lists `show`; `--show` still renders + notice).

## Open Questions

- None.
