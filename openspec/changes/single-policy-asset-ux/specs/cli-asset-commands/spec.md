## REMOVED Requirements

### Requirement: `tr asset reset [<name>]` removes an override file
**Reason**: The requirement's semantics presuppose multiple overrides in the slot ("remove every `.md` as a batch", `--all` flag, `--force`, TTY confirmation gates). The single-policy pivot settles that there is one shipped asset and the slot holds exactly one active file; batch machinery answers a question nobody can ask. Superseded by the ADDED no-arg form below.
**Migration**: `tr asset reset` (no args) now removes the sole shipped asset's override directly — no `--all`, no `--force`, no confirmation gate. A named argument still removes that one file, and remains the permissive cleanup path for stray files in the slot.

---

### Requirement: `tr asset sync [<name>]` writes the canonical embedded content to an override slot
**Reason**: The "no `<name>` argument SHALL be refused" contract (sync-all-as-destructive-restoration guard) is inverted: with exactly one shipped asset the argument was pure ceremony. Superseded by the ADDED sole-asset resolution semantics.
**Migration**: `tr asset sync` with no argument resolves the sole shipped asset; explicit `<name>` arguments keep working unchanged, including the `--force` overwrite protection and the `tr asset list` teaching error.

## ADDED Requirements

### Requirement: `tr asset reset [<name>]` removes override files one per invocation
`tr asset reset` SHALL remove override files from the data dir's `prompts/` directory (the Total Recall data dir is `$TR_HOME` when set, else `~/.tr`), one file per invocation. With no `<name>` argument, it SHALL operate on the sole shipped asset: resolve the shipped asset name by enumeration (exactly one shipped asset is always present in the shipped binary) and remove that asset's override file. With an explicit `<name>` argument that matches the shipped asset, the behavior is identical; with any other format-valid `<name>`, it SHALL still remove that named file — permissiveness toward non-shipped names is deliberate, because named reset is the cleanup path for stray files in the slot. When the target file does not exist, it SHALL exit 0 with a short "no override for <name> — nothing to reset" message (no error). The command SHALL exit 1 with a `could not resolve the Total Recall data dir` message when the data dir cannot be resolved at all; it SHALL exit 1 with a message listing the shipped asset names when the sole-asset resolution is ambiguous (zero or several shipped assets — states that cannot arise in the shipped single-asset binary, but the rule is future-proof by enumeration). There is no `--all` flag, no `--force` flag, and no TTY confirmation gate. On success, the command SHALL log a restart advisory: `[assets] removed override at <path>; restart 'tr serve' to pick up the change`. The on-disk removal is the only effect — the running daemon's in-memory cache is not touched; the change takes effect at the next daemon restart.

#### Scenario: No-arg reset removes the policy override
- **WHEN** `tr asset reset` is invoked (no arguments) with an override at `<data-dir>/prompts/question-generation-policy.md`
- **THEN** the file is removed; stdout contains `[assets] removed override at <path>; restart 'tr serve' to pick up the change`; exit 0

#### Scenario: No-arg reset when no override exists
- **WHEN** `tr asset reset` is invoked with no override present in the slot
- **THEN** stdout is short ("no override for question-generation-policy — nothing to reset"); exit 0; no file is touched

#### Scenario: Named reset removes an override
- **WHEN** `tr asset reset question-generation-policy` is invoked and the override file exists
- **THEN** the file is removed; stdout contains the restart advisory; exit 0

#### Scenario: Named reset removes a single non-existent override as a no-op
- **WHEN** `tr asset reset question-generation-policy` is invoked and the override file does not exist
- **THEN** stdout is short ("no override for question-generation-policy — nothing to reset"); exit 0; no file is touched

#### Scenario: Named reset is the stray-cleanup path
- **WHEN** `tr asset reset my-experiment` is invoked and `<data-dir>/prompts/my-experiment.md` exists but `my-experiment` is not a shipped asset name
- **THEN** the file is removed (permissive cleanup); stdout contains the restart advisory; exit 0

#### Scenario: Reset when the data dir cannot be resolved
- **WHEN** `tr asset reset [<name>]` is invoked and neither `TR_HOME` nor a home directory can be resolved
- **THEN** exit 1 with `could not resolve the Total Recall data dir`; no file is touched

---

### Requirement: `tr asset sync [<name>]` re-baselines the sole shipped asset or an explicitly named one
`tr asset sync` SHALL read the embedded bytes for a prompt asset (per the `//go:embed` pattern from `synthesize-from-context`) and write them to the data dir's `prompts/<name>.md` (the Total Recall data dir is `$TR_HOME` when set, else `~/.tr`). With no `<name>` argument, it SHALL resolve the sole shipped asset by enumeration and sync it — "you never type an asset name"; if zero or several shipped assets existed (a future-hypothetical in the shipped single-asset binary), it SHALL exit 1 with a message listing the shipped asset names to pick one. With an explicit `<name>` argument, it SHALL validate the name (`^[a-z0-9-]+$`, per the name-validation requirement) and operate on exactly that asset. When the target file exists and is non-empty, the command SHALL refuse with exit 1 and a message instructing `--force` to overwrite or `tr asset reset <name>` to start from defaults; with `--force`, the command SHALL overwrite the existing file. The command SHALL exit 1 with `could not resolve the Total Recall data dir` when the data dir cannot be resolved at all. When the embedded bytes for `<name>` are unavailable (corrupt build), the command SHALL exit 1 with `embedded asset '<name>' not found` and a pointer to `tr asset list`. On success, the command SHALL log `[assets] synced <name> to <path>; restart 'tr serve' to pick up the change`. The on-disk write is the only effect — the running daemon's in-memory cache is not touched; the daemon picks up the change at its next start.

#### Scenario: No-arg sync creates the policy override
- **WHEN** `tr asset sync` is invoked (no arguments) and `<data-dir>/prompts/question-generation-policy.md` does not exist
- **THEN** the file is created with the embedded bytes; stdout contains the restart advisory; exit 0

#### Scenario: Named sync a new override
- **WHEN** `tr asset sync question-generation-policy` is invoked and the override file does not exist
- **THEN** the file is created with the embedded bytes; stdout contains the restart advisory; exit 0

#### Scenario: Sync an existing non-empty override without --force refuses
- **WHEN** `tr asset sync question-generation-policy` is invoked, the target file exists and is non-empty, and `--force` is not passed
- **THEN** exit 1 with `error: <path> exists and is non-empty — pass --force to overwrite, or 'tr asset reset question-generation-policy' to start from defaults`; no file is modified

#### Scenario: Sync with --force overwrites the existing override
- **WHEN** `tr asset sync question-generation-policy --force` is invoked
- **THEN** the file is overwritten with the embedded bytes; stdout contains the restart advisory; exit 0

#### Scenario: Sync with an unknown explicit name points at the inventory
- **WHEN** `tr asset sync ecs-policy` is invoked and no shipped asset named `ecs-policy` exists
- **THEN** exit 1 with `embedded asset 'ecs-policy' not found` and the `run 'tr asset list'` pointer; no file is touched

#### Scenario: Sync when the data dir cannot be resolved
- **WHEN** `tr asset sync [<name>]` is invoked and neither `TR_HOME` nor a home directory can be resolved
- **THEN** exit 1 with `could not resolve the Total Recall data dir`; no file is touched

#### Scenario: Sync when embedded bytes are unavailable
- **WHEN** `tr asset sync <unknown-name>` is invoked and the binary's embedded map does not contain `<unknown-name>.md`
- **THEN** exit 1 with `embedded asset <unknown-name> not found`; no file is touched
