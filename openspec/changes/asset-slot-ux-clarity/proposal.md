## Why

The drop-in slot works, but its UX leaves three ambiguities that surfaced during the FEATURES.md work:

1. **Inert files look like success.** A `.md` file dropped into the slot that doesn't match a shipped asset name is listed by `tr asset list` with source `$TR_HOME`, exactly like a real override — but nothing ever loads it, so quizzes don't change. The worst ambiguity in the feature: silent no-op presented as active state.
2. **The naming rule is invisible.** The file must exactly match a shipped asset's name (today: `question-generation-policy`), but nothing *says* that — neither at the CLI (`sync` refuses unknown names, but the error doesn't teach the way out) nor in the docs.
3. **Wrong mental model invites confusion.** Users reasonably treat the slot as a workspace — put ideas in it — which is what makes extra files feel like they should work, forks feel like a limitation, and versioning feel "confined to git."

The corrective reframe (agreed in exploration): **the slot is a deployment target, not a workspace.** One file per shipped asset; the filename is the address; ideas live in git or a policies folder; the slot holds the one active doc; swap = copy over the slot; `tr asset list` confirms what's active.

## What Changes

- **`tr asset list` distinguishes active from unmanaged.** A file in the slot whose name shadows no shipped asset is listed with source `inactive` (new tag alongside `embedded` | `$TR_HOME` | `fallback`) — the path and age still print, so the file is explainable and cleanable, but nothing implies it's loaded.
- **Daemon startup surfaces unmanaged files.** One log line per unmanaged file at daemon startup: `[assets] unmanaged file %q in prompts/ — no shipped asset with this name; not active (run 'tr asset list')`. Reload... restart-persisted noise for a file the user should either rename, remove, or consciously keep — consistent with the drift-warning observability style: informational, never blocking.
- **`tr asset sync <unknown-name>` teaches the doorway.** Error message extended: `embedded asset '<name>' not found — run 'tr asset list' to see the available asset names`.
- **Documentation rewrite (FEATURES.md + README tweak):** the explicit naming rule ("the filename is the address; it must match a name the binary ships; don't name files by hand — `sync` names them"), the deployment-target model (ideas and versioning live in git or a policies folder; the slot activates exactly one; swap = copy over; `sync`/`reset` are the operators), and the front-matter clarification (front-matter `name:` is descriptive metadata; the filename is the address).

## Capabilities

### New Capabilities

- (none)

### Modified Capabilities

- `cli-asset-commands`: `tr asset list` SHALL list unmanaged slot files with an `inactive` source tag instead of implying they are overrides; `sync`'s unknown-name error SHALL teach the way out. Captured as ADDED requirements (order-robust against the still-unarchived `prompt-asset-observability` change, which also carries `cli-asset-commands` deltas).
- `prompt-asset-loading`: the daemon startup surface SHALL log unmanaged override-slot files. ADDED requirement for the same order-robustness reason; the archived `synthesize-from-context` delta is historical and untouched.

## Impact

- **Code:** `assets` package (`UnmanagedNames()` or equivalent — slot names minus embedded names, plus the startup warn helper); `cmd/tr/asset.go` (list source-tag classification against `assets.EmbeddedNames()`; sync error message extension); `cmd/tr/main.go` (`serveCmd` wires the unmanaged-file warning after `SetDriftThreshold`).
- **Tests:** `assets/assets_test.go` (unmanaged enumeration/log); `cmd/tr/asset_test.go` (`inactive` tag output, sync hint message, unmanaged log via serve wiring or the assets helper directly).
- **Docs:** `FEATURES.md` (naming rule + deployment-target section replacing part of the customization text); `README.md` (one-line tweak if the Make-it-yours wording implies anything contrary — expected to be a no-op or one clause).
- **Specs:** delta files under `openspec/changes/asset-slot-ux-clarity/specs/` only. Archives are untouched.
- **Dependencies:** none new. **BREAKING:** none — `reset <any-format-valid-name>` stays permissive (it is the orphan-cleanup path); list gains a tag; a log line is additive.

## Key Design Decisions

- **Slot = deployment target.** One active file per shipped asset; the filename is the address; `sync` is the doorway that names files correctly. Versioning/forking explicitly lives outside the slot (git, folders) — document this rather than building selection machinery for one asset.
- **`inactive` as a source tag, not a new column.** The four-column format (`name/source/path/age`) stays `awk`-friendly; unmanaged files report source `inactive` so one field answers "will this affect quizzes?" — no.
- **Startup warning, not blocking, not list-only.** The daemon is where confusion gets diagnosed last (bad quizzes, why?); a startup log line reaches the user in the same surface as the drift warning.
- **`reset` remains permissive toward unknown names.** Refusing reset on non-embedded names would strand the very orphan files this change teaches users to clean up.

## Non-Goals

- Multiple *active* variants / variant selection via config — explicitly declined; the `adaptive-difficulty` Resolver pattern makes this a natural follow-up if demand is ever real.
- `tr asset import <file>` with diff preview — still a future affordance if community sharing grows; the trust note in FEATURES.md covers today.
- Any policy-sharing platform — concept only.
- Changes to loader precedence, drift warning, or front-matter parsing — `prompt-asset-observability` and `features-doc-asset-hardening` own those surfaces.

## Developer Workflow Impact

No commit-time or daemon-loop changes. Additive observability (one log line, one list tag, one clearer error) and documentation read at leisure. Swapping policy docs via copy/sync/reset behaves exactly as before.

## Open Questions

- Whether the `inactive` tag should also appear in the `tr config show` "prompt assets" section — leaning no: show iterates shipped asset names only, so slot orphans are naturally out of its scope; the list is the inventory surface. Confirm at implementation.
