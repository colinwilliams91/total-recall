## Context

Three earlier changes shipped the drop-in idea, its observability, and the slot-UX story. The discussion that shaped this change concluded: (1) no-arg `sync`/`reset` — argument-free primary forms; (2) slot = deployment target; (3) **the multi-policy concept is a bluff** — the shipped policy doc is canonical, tightly bound to the tool's purpose, and question synthesis is diff-driven, so a plural-docs narrative only creates documentation that contradicts the implementation.

## Goals / Non-Goals

**Goals:**
- `tr asset sync` / `tr asset reset` work argument-free today (sole shipped asset), refusing by enumeration only when the future-hypothetical of multiple assets materializes.
- Delete the batch-removal apparatus (`--all`, `--force` on reset, TTY confirm gates) — machinery for a question that cannot be asked.
- Docs state the singular truth once, clearly: one policy, one file, `sync` names it; slot = deployment target; machinery details live in `tr asset --help`.

**Non-Goals:**
- Renames; deleting the presentation-level generality (`LoadAll`, `EmbeddedNames`, `inactive`, unmanaged warning); prefix matching; `tr policy` alias; changes to `prompt-asset-loading`'s canonical spec.

## Decisions

### Decision: Sole-asset resolution by enumeration

`soleAssetFrom(names []string) (string, error)`: exactly one name → that name; zero → "no shipped prompt assets" refusal; several → "multiple shipped prompt assets — specify one of: <names>". The wrapper feeds `assets.EmbeddedNames()`. No hardcoded constant; the shipped single asset is discovered, and the multiple-asset branch is exercised by tests against the pure function even though the binary can never produce it.

**Alternatives considered:**
- *Hardcode `question-generation-policy` as the no-arg target.* Rejected: two sources of truth for "the asset"; enumeration degrades gracefully if an asset ever ships.
- *Shorter canonical filename (`policy.md`).* Renaming is quasi-breaking for anyone with an existing override and the descriptive name pays off the moment a second asset is ever real.

### Decision: Batch machinery is deleted, not deprecated

`--all`, reset's `--force`, the TTY confirmation prompt, and `resetAllOverrides` go — the requirement they implement ("remove several as a batch, gated") described a plural world. Named reset remains permissive for stray cleanup, which is the residual use case, and it is strictly one file per invocation. Keep sync's `--force`: it guards the one real thing sync can clobber (hours of re-tuning), which is unrelated to multiplicity.

**Alternatives considered:**
- *Keep `--all` undocumented.* Rejected: undocumented destructive flags are obedience debt — precisely the residue the single-policy pivot is removing.

### Decision: Docs carry the singular truth; the canonical spec stays name-parameterized

FEATURES.md drops the plural vocabulary ("finite set of valid names", community sharing, per-domain docs) and ships a four-step walkthrough with a two-sentence naming rule. The `prompt-asset-loading` spec keeps describing name-keyed machinery — specs capture mechanism, not product bets. Package comment and AGENTS.md reworded to singular.

### Decision: `inactive` stays, with a reframed justification

A stock stray file is now more likely (users copying shared docs around), and each one deserves exactly one honest signal — the `inactive` tag and the startup line. No docs beyond one sentence; not deleting the `asset-slot-ux-clarity` behavior just shipped.

## Risks / Trade-offs

- **[BREAKING, narrow] `tr asset reset --all` disappears.** Unreleased-in-docs surface; scripts passing `--all` on a modern build didn't exist — called out in the proposal Impact section and superseded in the delta.
- **[Trade-off] `soleAssetFrom`'s multi-asset branch is untestable against the real binary** (one asset embedded). Mitigated: pure function unit-tested with 0/1/N tables; the wrapper is a two-liner.

## Migration Plan

1. `soleAssetFrom` helper + sync/reset no-arg branches; delete batch path; rewrite `Long` help.
2. Tests: new no-arg/resolution tests; retire batch tests.
3. Docs: FEATURES.md walkthrough + singular framing; README/AGENTS/assets-comment one-liners.
4. Build/vet/test/validate + manual e2e (no-arg sync → edit → list confirm → no-arg reset; sync --force overwrite protection live).

## Open Questions

- None.
