
## Purpose

Inspect and manage the runtime override of the shipped prompt-asset policy from the terminal: inspect what is loaded (show), remove an override (reset), and re-baseline an override from the shipped default (sync), all without the daemon running and without touching the binary.

## Requirements




### Requirement: `tr asset show` enumerates resolved prompt assets in plain text
`torec asset show` SHALL enumerate every prompt asset known to the daemon. For each asset, it SHALL print one line with four tab-separated columns: `<name>`, `<source>` (one of `embedded`, `$TR_HOME`, `fallback`, `inactive`), `<resolved-path>` (the absolute path the asset was loaded from, or `<embedded>` when no override is in effect), and `<age>` (a human-readable mtime-relative string such as `embedded`, `6mo old`, or `2d old`). The output SHALL be plain text suitable for `awk`/`grep` parsing — no JSON, no table decoration. The command SHALL NOT require the daemon to be running (it reads the filesystem state of the data dir's `prompts/` directory directly — the Total Recall data dir is `$TR_HOME` when set, else `~/.torec` — plus the embedded bytes available from the binary).

#### Scenario: No overrides — embedded default
- **WHEN** `torec asset show` is invoked and no override files exist in the data dir's `prompts/` directory
- **THEN** the output contains a line for the canonical asset (`question-generation-policy`) with `embedded` source and `embedded` age

#### Scenario: One override present
- **WHEN** `torec asset show` is invoked and an override file exists at `<data-dir>/prompts/question-generation-policy.md`
- **THEN** the output contains a line with `$TR_HOME` source, the absolute path to the override file, and a human-readable mtime age (e.g., `2d old`, `6mo old`)

#### Scenario: Multiple overrides
- **WHEN** `torec asset show` is invoked and two distinct `.md` files exist under the data dir's `prompts/` directory
- **THEN** the output contains one line per distinct asset name; non-overridden embedded assets still appear with `embedded` source

#### Scenario: Daemon not running — show still works
- **WHEN** `torec asset show` is invoked and the daemon is not reachable
- **THEN** the command exits 0 with valid output — `show` reads disk state and the binary's embedded bytes, not daemon state


### Requirement: `tr asset show` tags slot files that shadow no shipped asset as `inactive`
`torec asset show` SHALL classify each entry in the four-column output by whether the file's name corresponds to a shipped (embedded) asset. Files under the data dir's `prompts/` directory whose name does not correspond to any shipped asset SHALL be printed with source `inactive` — with their resolved path and human age still populated — instead of the `$TR_HOME` tag used for active overrides. A file listed as `inactive` SHALL NOT be loaded by the engine and SHALL NOT affect quiz generation; the tag exists to make that fact visible. Active overrides and embedded defaults are unaffected and keep their existing tags. The command SHALL NOT require the daemon for this classification (it is derivable from `EmbeddedNames()` and directory contents alone).

#### Scenario: Shipped-name override remains active
- **WHEN** `torec asset show` is invoked and `<data-dir>/prompts/question-generation-policy.md` exists
- **THEN** the line for `question-generation-policy` carries source `$TR_HOME` as before

#### Scenario: Unmanaged file is tagged inactive
- **WHEN** `torec asset show` is invoked and `<data-dir>/prompts/my-experiment.md` exists (no shipped asset with that name)
- **THEN** the line for `my-experiment` carries source `inactive`, with the resolved path and a human age; no log or list claim implies the file is loaded

#### Scenario: Cleanup path still works on unmanaged files
- **WHEN** `torec asset reset my-experiment` is invoked after the tagging above
- **THEN** the file is removed and the restart advisory prints, per the existing reset behavior (reset is the cleanup path for unmanaged files)

### Requirement: `torec asset reset [<name>]` removes override files one per invocation
`torec asset reset` SHALL remove override files from the data dir's `prompts/` directory (the Total Recall data dir is `$TR_HOME` when set, else `~/.torec`), one file per invocation. With no `<name>` argument, it SHALL operate on the sole shipped asset: resolve the shipped asset name by enumeration (exactly one shipped asset is always present in the shipped binary) and remove that asset's override file. With an explicit `<name>` argument that matches the shipped asset, the behavior is identical; with any other format-valid `<name>`, it SHALL still remove that named file — permissiveness toward non-shipped names is deliberate, because named reset is the cleanup path for stray files in the slot. When the target file does not exist, it SHALL exit 0 with a short "no override for <name> — nothing to reset" message (no error). The command SHALL exit 1 with a `could not resolve the Total Recall data dir` message when the data dir cannot be resolved at all; it SHALL exit 1 with a message listing the shipped asset names when the sole-asset resolution is ambiguous (zero or several shipped assets — states that cannot arise in the shipped single-asset binary, but the rule is future-proof by enumeration). There is no `--all` flag, no `--force` flag, and no TTY confirmation gate. On success, the command SHALL log a restart advisory: `[assets] removed override at <path>; restart 'torec serve' to pick up the change`. The on-disk removal is the only effect — the running daemon's in-memory cache is not touched; the change takes effect at the next daemon restart.

#### Scenario: No-arg reset removes the policy override
- **WHEN** `torec asset reset` is invoked (no arguments) with an override at `<data-dir>/prompts/question-generation-policy.md`
- **THEN** the file is removed; stdout contains `[assets] removed override at <path>; restart 'torec serve' to pick up the change`; exit 0

#### Scenario: No-arg reset when no override exists
- **WHEN** `torec asset reset` is invoked with no override present in the slot
- **THEN** stdout is short ("no override for question-generation-policy — nothing to reset"); exit 0; no file is touched

#### Scenario: Named reset removes an override
- **WHEN** `torec asset reset question-generation-policy` is invoked and the override file exists
- **THEN** the file is removed; stdout contains the restart advisory; exit 0

#### Scenario: Named reset removes a single non-existent override as a no-op
- **WHEN** `torec asset reset question-generation-policy` is invoked and the override file does not exist
- **THEN** stdout is short ("no override for question-generation-policy — nothing to reset"); exit 0; no file is touched

#### Scenario: Named reset is the stray-cleanup path
- **WHEN** `torec asset reset my-experiment` is invoked and `<data-dir>/prompts/my-experiment.md` exists but `my-experiment` is not a shipped asset name
- **THEN** the file is removed (permissive cleanup); stdout contains the restart advisory; exit 0

#### Scenario: Reset when the data dir cannot be resolved
- **WHEN** `torec asset reset [<name>]` is invoked and neither `TR_HOME` nor a home directory can be resolved
- **THEN** exit 1 with `could not resolve the Total Recall data dir`; no file is touched

---

### Requirement: `torec asset sync [<name>]` re-baselines the sole shipped asset or an explicitly named one
`torec asset sync` SHALL read the embedded bytes for a prompt asset (per the `//go:embed` pattern from `synthesize-from-context`) and write them to the data dir's `prompts/<name>.md` (the Total Recall data dir is `$TR_HOME` when set, else `~/.torec`). With no `<name>` argument, it SHALL resolve the sole shipped asset by enumeration and sync it — "you never type an asset name"; if zero or several shipped assets existed (a future-hypothetical in the shipped single-asset binary), it SHALL exit 1 with a message listing the shipped asset names to pick one. With an explicit `<name>` argument, it SHALL validate the name (`^[a-z0-9-]+$`, per the name-validation requirement) and operate on exactly that asset. When the target file exists and is non-empty, the command SHALL refuse with exit 1 and a message instructing `--force` to overwrite or `torec asset reset <name>` to start from defaults; with `--force`, the command SHALL overwrite the existing file. The command SHALL exit 1 with `could not resolve the Total Recall data dir` when the data dir cannot be resolved at all. When the embedded bytes for `<name>` are unavailable (corrupt build), the command SHALL exit 1 with `embedded asset '<name>' not found` and a pointer to `torec asset show`. On success, the command SHALL log `[assets] synced <name> to <path>; restart 'torec serve' to pick up the change`. The on-disk write is the only effect — the running daemon's in-memory cache is not touched; the daemon picks up the change at its next start.

#### Scenario: No-arg sync creates the policy override
- **WHEN** `torec asset sync` is invoked (no arguments) and `<data-dir>/prompts/question-generation-policy.md` does not exist
- **THEN** the file is created with the embedded bytes; stdout contains the restart advisory; exit 0

#### Scenario: Named sync a new override
- **WHEN** `torec asset sync question-generation-policy` is invoked and the override file does not exist
- **THEN** the file is created with the embedded bytes; stdout contains the restart advisory; exit 0

#### Scenario: Sync an existing non-empty override without --force refuses
- **WHEN** `torec asset sync question-generation-policy` is invoked, the target file exists and is non-empty, and `--force` is not passed
- **THEN** exit 1 with `error: <path> exists and is non-empty — pass --force to overwrite, or 'torec asset reset question-generation-policy' to start from defaults`; no file is modified

#### Scenario: Sync with --force overwrites the existing override
- **WHEN** `torec asset sync question-generation-policy --force` is invoked
- **THEN** the file is overwritten with the embedded bytes; stdout contains the restart advisory; exit 0

#### Scenario: Sync with an unknown explicit name points at the inventory
- **WHEN** `torec asset sync ecs-policy` is invoked and no shipped asset named `ecs-policy` exists
- **THEN** exit 1 with `embedded asset 'ecs-policy' not found` and the `run 'torec asset show'` pointer; no file is touched

#### Scenario: Sync when the data dir cannot be resolved
- **WHEN** `torec asset sync [<name>]` is invoked and neither `TR_HOME` nor a home directory can be resolved
- **THEN** exit 1 with `could not resolve the Total Recall data dir`; no file is touched

#### Scenario: Sync when embedded bytes are unavailable
- **WHEN** `torec asset sync <unknown-name>` is invoked and the binary's embedded map does not contain `<unknown-name>.md`
- **THEN** exit 1 with `embedded asset <unknown-name> not found`; no file is touched

---


### Requirement: `tr asset reset|sync` validate the `<name>` argument before path construction
`torec asset reset` and `torec asset sync` SHALL validate an explicit `<name>` argument against `^[a-z0-9-]+$` (a single lowercase-hyphenated token, matching shipped asset naming) before joining it into a path. An argument that is empty after trimming, contains path separators, dots, whitespace, or characters outside the pattern SHALL be rejected with exit 1 and a message naming the offending argument and the expected form (`invalid asset name '<arg>' — expected a single lowercase-hyphenated name, e.g. 'question-generation-policy'`). No file operation SHALL be attempted for an invalid name. The no-argument forms of `reset` are not affected — they target the sole shipped asset by enumeration.

#### Scenario: Canonical name is accepted
- **WHEN** `torec asset reset question-generation-policy` is invoked (or the `sync` equivalent)
- **THEN** the name passes validation and the command behaves per its existing requirements (remove the override / write the embedded bytes)

#### Scenario: Traversal-shaped name is rejected
- **WHEN** `torec asset reset ../../sensitive` or `torec asset sync ../prompts` is invoked
- **THEN** exit 1 with `invalid asset name` and the expected form; no path outside the override directory is constructed; no file is removed or written

#### Scenario: Odd characters are rejected
- **WHEN** `torec asset sync "Question Policy!"` or `torec asset reset "policy.md"` (`.md` suffix supplied by the user) is invoked
- **THEN** exit 1 with `invalid asset name` and the expected form; no file is touched


### Requirement: `tr asset` self-documents the override loop in long help
The `asset` command SHALL carry long help text (Cobra `Long`) that explains, in plain language: what prompt-asset overrides are (a markdown file in the data dir's `prompts/` directory replacing the shipped default), the inspect → recover loop (`show` / `reset` / `sync`), and the restart caveat (the daemon picks up file mutations on next `torec serve` start). `torec asset --help` and `torec help asset` SHALL both render it. The help text is user documentation in the terminal — no new flags or subcommands are introduced by this requirement.

#### Scenario: Long help renders the override loop
- **WHEN** `torec asset --help` (or `torec help asset`) is invoked
- **THEN** the output describes what an override is, when to use `show` vs `reset` vs `sync`, and the restart caveat — not just the one-line command summaries
