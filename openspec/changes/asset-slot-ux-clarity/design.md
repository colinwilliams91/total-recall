## Context

The drop-in slot's UX has three ambiguities: inert files look like active overrides in `tr asset list`; the naming rule (filename = shipped asset name) is nowhere visible; and the slot invites a workspace mental model where a workspace is the wrong frame. Exploration agreed on the corrective model — **slot = deployment target** — with no variant machinery.

## Goals / Non-Goals

**Goals:**
- Make inert-ness visible: `inactive` tag in `list`, startup log for unmanaged files, teaching error on `sync` with an unknown name.
- Make the naming rule explicit and opinionated in docs: filename is the address, `sync` names files correctly, don't hand-name.
- Document the deployment-target model (ideas + versioning in git/folders; the slot activates exactly one; swap = copy over; `sync`/`reset` are the operators).

**Non-Goals:**
- Multiple active variants or config-driven selection (`adaptive-difficulty`'s Resolver pattern is the natural vehicle if this ever becomes real).
- `tr asset import`, a sharing platform, or changes to loader precedence/drift/front-matter parsing.

## Decisions

### Decision: Slot is a deployment target, and the docs say so

One file per shipped asset; the filename is the address; `tr asset sync <name>` is the doorway that creates the correctly-named file. Ideas, drafts, forks, and version history live in git or a policies folder outside the data dir. Swapping = copying a candidate over the slot file (or `sync` + edit); `tr asset list` confirms what is active; `reset` empties the slot. The user's "can't fork into two files" concern is resolved by reframing, not mechanism: the slot was never where ideas live.

**Alternatives considered:**
- *Config-driven variant selection (`prompt-asset.override: <name>`).* Deferred: with one shipped asset it machinery-for-nobody; the `adaptive-difficulty` Resolver pattern makes the later bolt-on natural.
- *Subdirectory convention for variants (`prompts/variants/`).* Rejected for now: adds a second rule to teach while selection machinery doesn't exist to honor it.

### Decision: `inactive` is a source-tag value, dispatched in `cmd/tr`, not the assets package

The list command already owns presentation (tab-separated, parse-friendly); it classifies each name against `assets.EmbeddedNames()`. The assets package stays presentation-neutral — `LoadAll` keeps resolving whatever is on disk (the daemon's startup warning needs the same classification, so the *classification primitive* lives in assets: `UnmanagedNames()` = slot names − embedded names; the list borrows the same primitive so both surfaces can never drift).

**Alternatives considered:**
- *A fifth `state` column (`active|inactive`).* Rejected: five columns complicate `awk` parsing and duplicate information — no override file is ever "inactive" once it shadows a shipped asset, so source alone can carry it: `embedded` / `$TR_HOME` (active) / `inactive` / `fallback`.
- *Classify inside `LoadAll` by returning a different `Source`.* Rejected: `Source` semantically means where an asset was loaded *from*; an unmanaged file is not loaded at all — conflating the two breaks the `"$TR_HOME"` tag's meaning for every consumer.

### Decision: The startup warning lives in the daemon wiring, backed by an assets helper

`assets.WarnUnmanagedOverrides()` enumerates slot `.md` files, computes the unmanaged delta against `EmbeddedNames()`, and logs one line per file. `serveCmd` calls it after `SetDriftThreshold` — same wiring point as the drift threshold, visible in the same startup log surface. Silent when the slot is empty/absent/unresolvable; never blocking.

**Alternatives considered:**
- *Warn inside `Load` at first resolution.* Rejected: `Load` sees one name at a time and never sees the slot's population — it structurally cannot detect "no shipped asset will ever request this name."
- *Warn in `LoadAll`.* Rejected: `LoadAll` is called by `list` and `config show`; a warning firing from every inspection command is noise, and `list`'s job is the tag, not logs.

### Decision: `sync`'s error message carries the pointer, not a name-list dump

`embedded asset '<name>' not found — run 'tr asset list' to see the available asset names`. Dumping the asset list into the error would desync the moment an asset is added or removed; pointing at the inventory keeps one source of truth.

### Decision: Show-section scope unchanged (Open Question resolved as open)

`tr config show`'s `prompt assets:` section iterates shipped names only — by construction it cannot display slot orphans, and adding them would bloat a config-debugging surface with non-config files. The proposal's open question is resolved: no change. `tr asset list` is the inventory surface.

## Risks / Trade-offs

- **[Trade-off] Startups with consciously-kept unmanaged files get one log line each.** Acceptable: matches the drift-warning posture (informational, unblockable noise for a state the user should see at least once). A persistent noise complaint could add a dedicated silence mechanism later — deliberately not built.
- **[Risk] `inactive` tag semantics could confuse consumers that parse `list` by source value.** Mitigated: the tag is additive to an enum the tool just introduced (no released consumers of the exact tag set); theFEATURES.md workflow text updated in the same change.

## Migration Plan

1. assets: `UnmanagedNames()` + `WarnUnmanagedOverrides()`; wire the call in `serveCmd` after `SetDriftThreshold`.
2. `cmd/tr/asset.go`: list dispatches `inactive` for unmanaged names; extend sync's unknown-asset error message.
3. Tests: assets enumeration/log; list tag; sync hint; reset-on-unmanaged still works.
4. FEATURES.md naming/deployment-target rewrite; README touch-up if needed.
5. Build/vet/test/validate + manual e2e.

## Open Questions

- None (proposal's open question resolved above: show-section scope unchanged).
