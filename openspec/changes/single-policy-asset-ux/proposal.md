## Why

The drop-in feature's UX was confusing for three named reasons, and the fix agreed in exploration — "slot = deployment target, no-arg commands, docs as walkthrough" — surfaced one deeper discovery: **the multi-policy concept was the confusion's root cause, and it is a bluff.** There will never be multiple question-generation policies. The shipped policy doc is canonical and tightly bound to the tool's purpose; question synthesis is driven overwhelmingly by the working diffs and cached concepts, the policy doc only shapes pedagogy *framing*, and its tunable surface is small. The community sharing of policy docs is a pipe dream. Documentation currently explains machinery ("finite set of valid names", `inactive` eligibility, per-domain docs) sized for a plural feature that does not exist and never will — and the machine-level inventory concepts were written down as if plural was the point.

Also carried from the prior discussion: `tr asset sync` and `tr asset reset` should work with **no argument** when there is exactly one shipped asset (there is), because forcing the long asset name is pure ceremony; and `reset`'s batch-removal machinery (`--all`, `--force`, TTY gates) is an orphan of the multi-doc era — its confirmation apparatus answers "which of several overrides?" for a world where there is only ever one.

## What Changes

- **`tr asset sync` (no args) targets the sole shipped asset.** No argument: resolves the single shipped asset name and syncs it ("you never type an asset name"). Multiple shipped assets (hypothetical future): error listing the names. Explicit `<name>` args keep working unchanged (validation, `--force` overwrite protection, teaching error).
- **`tr asset reset` (no args) removes the sole shipped asset's override.** Explicit `<name>` args keep working (permissive — they flush stray or unmanaged files). The batch-removal path is deleted: `--all` and `--force` flags, the TTY confirmation prompt, and the "requires --all" gate all go. Reset is exactly one file per invocation, with the existing single-file advisory semantics.
- **Documentation tells the single-policy truth.** FEATURES.md: the customization section becomes a four-step walkthrough (`sync` → edit → restart → `list` to confirm), with one two-sentence naming rule ("one policy. one file. `sync` names it; anything else in the slot is listed `inactive` and ignored"). The Community policy-sharing section is deleted (a one-sentence trust note stays: a shared doc deploys by copying over the slot file — review what you import). "The filename is the address" machinery talk exits FEATURES.md and lands in `tr asset --help`. README teaser drops the long asset name. `assets` package doc comment and AGENTS.md reworded from "future prompt assets" to the singular truth.
- **`tr asset --help` re-taught**: the addition of no-arg primary forms and the slot-deployment framing.

## Capabilities

### New Capabilities

- (none)

### Modified Capabilities

- `cli-asset-commands`: the reset and sync requirements are replaced via REMOVED + ADDED (requirements level — the clean mechanism for layer-level semantics that retire scenarios): reset loses its batch apparatus (`--all`, `--force`, TTY gates) and gains a no-arg form targeting the sole shipped asset; sync's no-arg form resolves the sole shipped asset instead of refusing. The canonical spec was just created by the `prompt-asset-observability` archive, so the REMOVED blocks header-match it directly.

## Impact

- **Code:** `cmd/tr/asset.go` (sole-asset resolution helper; sync/reset no-arg branches; delete `resetAllOverrides`, the `--all`/`--force` reset flags, and the TTY `confirm` helper; `Long` help rewrite); assets package doc comment wording.
- **Tests:** `cmd/tr/asset_test.go` — no-arg sync creates the override; no-arg reset removes it / no-ops when absent; sole-asset resolution table (0/1/N); batch-related tests retired.
- **Docs:** `FEATURES.md`, `README.md` (one clause), `AGENTS.md` (one clause), `assets/assets.go` package comment.
- **Specs:** `openspec/changes/single-policy-asset-ux/specs/cli-asset-commands/spec.md` (2 MODIFIED requirements, headers matching the canonical created at the `prompt-asset-observability` archive).
- **Dependencies:** none. **BREAKING:** the batch-reset invocation (`tr asset reset --all [--force]`) is removed; the multi-asset "sync with no name refuses" contract is replaced. Neither form is user-facing documentation anywhere — the breaking change is a CLI surface called out explicitly in this change.

## Key Design Decisions

- **No-arg = sole asset, decided by enumeration not hardcoding.** `EmbeddedNames()` answers "which asset?" from the binary itself — zero shipped assets or several both refuse with the inventory pointer, so the rule survives contact with reality. No constant is invented.
- **`--force` survives on `sync`, dies on `reset`.** Sync's `--force` protects *your tuning* from a silent overwrite (the only file it can clobber, and a real one); reset's was an artifact of multi-file batch removal, which no longer exists.
- **The plural machinery in code stays dormant, not deleted.** `LoadAll`, `EmbeddedNames`, `UnmanagedNames`, the `inactive` tag, and the unmanaged startup log keep serving the single-asset world: slot hygiene (a stray file is by definition a mis-drop worth one log line), and list's status role. Only its *narrative* is stripped — docs stop implying names/variants/inventories matter to the user.
- **Canonical spec mechanism language stays name-parameterized.** The spec describes how the loader and CLI work, not a product bet that no second asset is conceivable; the singular truth lives in docs, package comments, and command help. (Explicitly considered and declined during this change's exploration.)

## Non-Goals

- Renaming the asset or its file (`question-generation-policy` stays).
- Deleting `LoadAll`/`EmbeddedNames`/unmanaged machinery — presentation-level generality is invisible in docs and needs no churn.
- A `tr policy` alias subcommand or prefix matching — more name-resolution is how the confusion got here.
- Any change to loader precedence, drift warning, front-matter parsing, or the `prompt-asset-loading` canonical spec.

## Developer Workflow Impact

Zero commit-time or daemon-loop changes. `tr asset sync` / `tr asset reset` lose a mandatory argument; `list` and the drift warning are unchanged; a hypothetical script that used `reset --all` (unreleased batching) or passed names explicitly keeps working for the single shipped asset.

## Open Questions

- None carried in — exploration concluded both calls (delete batch machinery; docs singular / spec name-parameterized).
