
## Purpose

Inspect and manage the runtime override of the shipped prompt-asset policy from the terminal: inspect what is loaded (show), remove an override (reset), and re-baseline an override from the shipped default (sync), all without the daemon running and without touching the binary.

## Requirements


### Requirement: `tr asset show` enumerates resolved prompt assets in plain text
`tr asset show` SHALL enumerate every prompt asset known to the daemon. For each asset, it SHALL print one line with four tab-separated columns: `<name>`, `<source>` (one of `embedded`, `$TR_HOME`, `fallback`, `inactive`), `<resolved-path>` (the absolute path the asset was loaded from, or `<embedded>` when no override is in effect), and `<age>` (a human-readable mtime-relative string such as `embedded`, `6mo old`, or `2d old`). The output SHALL be plain text suitable for `awk`/`grep` parsing — no JSON, no table decoration. The command SHALL NOT require the daemon to be running (it reads the filesystem state of the data dir's `prompts/` directory directly — the Total Recall data dir is `$TR_HOME` when set, else `~/.tr` — plus the embedded bytes available from the binary).

#### Scenario: No overrides — embedded default
- **WHEN** `tr asset show` is invoked and no override files exist in the data dir's `prompts/` directory
- **THEN** the output contains a line for the canonical asset (`question-generation-policy`) with `embedded` source and `embedded` age

#### Scenario: One override present
- **WHEN** `tr asset show` is invoked and an override file exists at `<data-dir>/prompts/question-generation-policy.md`
- **THEN** the output contains a line with `$TR_HOME` source, the absolute path to the override file, and a human-readable mtime age (e.g., `2d old`, `6mo old`)

#### Scenario: Multiple overrides
- **WHEN** `tr asset show` is invoked and two distinct `.md` files exist under the data dir's `prompts/` directory
- **THEN** the output contains one line per distinct asset name; non-overridden embedded assets still appear with `embedded` source

#### Scenario: Daemon not running — show still works
- **WHEN** `tr asset show` is invoked and the daemon is not reachable
- **THEN** the command exits 0 with valid output — `show` reads disk state and the binary's embedded bytes, not daemon state

---

### Requirement: `tr asset show` tags slot files that shadow no shipped asset as `inactive`
`tr asset show` SHALL classify each entry in the four-column output by whether the file's name corresponds to a shipped (embedded) asset. Files under the data dir's `prompts/` directory whose name does not correspond to any shipped asset SHALL be printed with source `inactive` — with their resolved path and human age still populated — instead of the `$TR_HOME` tag used for active overrides. A file listed as `inactive` SHALL NOT be loaded by the engine and SHALL NOT affect quiz generation; the tag exists to make that fact visible. Active overrides and embedded defaults are unaffected and keep their existing tags. The command SHALL NOT require the daemon for this classification (it is derivable from `EmbeddedNames()` and directory contents alone).

#### Scenario: Shipped-name override remains active
- **WHEN** `tr asset show` is invoked and `<data-dir>/prompts/question-generation-policy.md` exists
- **THEN** the line for `question-generation-policy` carries source `$TR_HOME` as before

#### Scenario: Unmanaged file is tagged inactive
- **WHEN** `tr asset show` is invoked and `<data-dir>/prompts/my-experiment.md` exists (no shipped asset with that name)
- **THEN** the line for `my-experiment` carries source `inactive`, with the resolved path and a human age; no log or list claim implies the file is loaded

#### Scenario: Cleanup path still works on unmanaged files
- **WHEN** `tr asset reset my-experiment` is invoked after the tagging above
- **THEN** the file is removed and the restart advisory prints, per the existing reset behavior (reset is the cleanup path for unmanaged files)

---

### Requirement: `tr asset reset [<name>]` removes an override file
`tr asset reset` SHALL remove the override file at `<data-dir>/prompts/<name>.md` (the Total Recall data dir is `$TR_HOME` when set, else `~/.tr`). With an explicit `<name>` argument, it SHALL remove only that file; when the file does not exist, it SHALL exit 0 with a short "no override for <name> — nothing to reset" message (no error). With no `<name>` argument, it SHALL enumerate every `.md` under the data dir's `prompts/` directory and remove them as a batch — gated by a `--all` flag and a TTY confirmation prompt. The command SHALL exit 1 with a `could not resolve the Total Recall data dir` message when the data dir cannot be resolved at all (no `TR_HOME` and no home directory available). On success, the command SHALL log a restart advisory: `[assets] removed override at <path>; restart 'tr serve' to pick up the change`. The on-disk removal is the only effect — the running daemon's in-memory cache is not touched; the change takes effect at the next daemon restart.

#### Scenario: Reset a single existing override
- **WHEN** `tr asset reset question-generation-policy` is invoked and `<data-dir>/prompts/question-generation-policy.md` exists
- **THEN** the file is removed; stdout contains the restart advisory; exit 0

#### Scenario: Reset a single non-existent override is a no-op
- **WHEN** `tr asset reset question-generation-policy` is invoked and the override file does not exist
- **THEN** stdout is short ("no override for question-generation-policy — nothing to reset"); exit 0; no file is touched

#### Scenario: Reset with no args requires --all
- **WHEN** `tr asset reset` is invoked with no arguments and two override files exist under the data dir's `prompts/` directory
- **THEN** the command refuses and exits non-zero with a message instructing `--all --force` for batch removal

#### Scenario: Reset --all --force in TTY removes all overrides
- **WHEN** `tr asset reset --all --force` is invoked in an interactive TTY and two overrides exist
- **THEN** both files are removed; stdout contains the restart advisory; exit 0

#### Scenario: Reset when the data dir cannot be resolved
- **WHEN** `tr asset reset <name>` is invoked and neither `TR_HOME` nor a home directory can be resolved
- **THEN** exit 1 with `could not resolve the Total Recall data dir`; no file is touched

---

### Requirement: `tr asset sync [<name>]` writes the canonical embedded content to an override slot
`tr asset sync <name>` SHALL read the embedded bytes for `<name>.md` (per the `//go:embed` pattern from `synthesize-from-context`) and write them to `<data-dir>/prompts/<name>.md` (the Total Recall data dir is `$TR_HOME` when set, else `~/.tr`). When the target file exists and is non-empty, the command SHALL refuse with exit 1 and a message instructing `--force` to overwrite or `tr asset reset <name>` to start from defaults. With `--force`, the command SHALL overwrite the existing file. The command SHALL refuse to run with no `<name>` argument (sync-all is too easily mistaken for destructive default-restoration; an explicit name forces intent). The command SHALL exit 1 with `could not resolve the Total Recall data dir` when the data dir cannot be resolved at all. When the embedded bytes for `<name>` are unavailable (corrupt build) or no shipped asset with that name exists, the command SHALL exit 1 with the message `embedded asset '<name>' not found — run 'tr asset show' to see the available asset names`; the refusal SHALL still prevent any file write, and the error doubles as the feature's teaching surface: the finite set of valid override names is exactly what `tr asset show` prints. On success, the command SHALL log `[assets] synced <name> to <path>; restart 'tr serve' to pick up the change`. The on-disk write is the only effect — the running daemon's in-memory cache is not touched.

#### Scenario: Sync a new override
- **WHEN** `tr asset sync question-generation-policy` is invoked and `<data-dir>/prompts/question-generation-policy.md` does not exist
- **THEN** the file is created with the embedded bytes; stdout contains the restart advisory; exit 0

#### Scenario: Sync an existing non-empty override without --force refuses
- **WHEN** `tr asset sync question-generation-policy` is invoked, the target file exists and is non-empty, and `--force` is not passed
- **THEN** exit 1 with `error: <path> exists and is non-empty — pass --force to overwrite, or 'tr asset reset <name>' to start from defaults`; no file is modified

#### Scenario: Sync with --force overwrites the existing override
- **WHEN** `tr asset sync question-generation-policy --force` is invoked
- **THEN** the file is overwritten with the embedded bytes; stdout contains the restart advisory; exit 0

#### Scenario: Sync with no name refuses
- **WHEN** `tr asset sync` is invoked with no arguments
- **THEN** exit 1 with `error: sync requires an explicit asset name`; no file is modified

#### Scenario: Sync when the data dir cannot be resolved
- **WHEN** `tr asset sync <name>` is invoked and neither `TR_HOME` nor a home directory can be resolved
- **THEN** exit 1 with `could not resolve the Total Recall data dir`; no file is touched

#### Scenario: Sync when embedded bytes are unavailable / unknown name points at the inventory
- **WHEN** `tr asset sync ecs-policy` is invoked and the binary's embedded map contains no shipped asset named `ecs-policy`
- **THEN** exit 1; the message contains `not found` and the `tr asset show` pointer; no file is touched

#### Scenario: Valid name is unaffected
- **WHEN** `tr asset sync question-generation-policy` is invoked
- **THEN** behavior is unchanged per the existing sync requirements (embedded bytes written to the correctly-named file)