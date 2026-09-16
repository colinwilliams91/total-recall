## MODIFIED Requirements

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

---

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

---

### Requirement: `tr asset reset [<name>]` removes an override file
`torec asset reset` SHALL remove the override file at `<data-dir>/prompts/<name>.md` (the Total Recall data dir is `$TR_HOME` when set, else `~/.torec`). With an explicit `<name>` argument, it SHALL remove only that file; when the file does not exist, it SHALL exit 0 with a short "no override for <name> — nothing to reset" message (no error). With no `<name>` argument, it SHALL enumerate every `.md` under the data dir's `prompts/` directory and remove them as a batch — gated by a `--all` flag and a TTY confirmation prompt. The command SHALL exit 1 with a `could not resolve the Total Recall data dir` message when the data dir cannot be resolved at all (no `TR_HOME` and no home directory available). On success, the command SHALL log a restart advisory: `[assets] removed override at <path>; restart 'torec serve' to pick up the change`. The on-disk removal is the only effect — the running daemon's in-memory cache is not touched; the change takes effect at the next daemon restart.

#### Scenario: Reset a single existing override
- **WHEN** `torec asset reset question-generation-policy` is invoked and `<data-dir>/prompts/question-generation-policy.md` exists
- **THEN** the file is removed; stdout contains the restart advisory; exit 0

#### Scenario: Reset a single non-existent override is a no-op
- **WHEN** `torec asset reset question-generation-policy` is invoked and the override file does not exist
- **THEN** stdout is short ("no override for question-generation-policy — nothing to reset"); exit 0; no file is touched

#### Scenario: Reset with no args requires --all
- **WHEN** `torec asset reset` is invoked with no arguments and two override files exist under the data dir's `prompts/` directory
- **THEN** the command refuses and exits non-zero with a message instructing `--all --force` for batch removal

#### Scenario: Reset --all --force in TTY removes all overrides
- **WHEN** `torec asset reset --all --force` is invoked in an interactive TTY and two overrides exist
- **THEN** both files are removed; stdout contains the restart advisory; exit 0

#### Scenario: Reset when the data dir cannot be resolved
- **WHEN** `torec asset reset <name>` is invoked and neither `TR_HOME` nor a home directory can be resolved
- **THEN** exit 1 with `could not resolve the Total Recall data dir`; no file is touched

---

### Requirement: `tr asset sync [<name>]` writes the canonical embedded content to an override slot
`torec asset sync <name>` SHALL read the embedded bytes for `<name>.md` (per the `//go:embed` pattern from `synthesize-from-context`) and write them to `<data-dir>/prompts/<name>.md` (the Total Recall data dir is `$TR_HOME` when set, else `~/.torec`). When the target file exists and is non-empty, the command SHALL refuse with exit 1 and a message instructing `--force` to overwrite or `torec asset reset <name>` to start from defaults. With `--force`, the command SHALL overwrite the existing file. The command SHALL refuse to run with no `<name>` argument (sync-all is too easily mistaken for destructive default-restoration; an explicit name forces intent). The command SHALL exit 1 with `could not resolve the Total Recall data dir` when the data dir cannot be resolved at all. When the embedded bytes for `<name>` are unavailable (corrupt build) or no shipped asset with that name exists, the command SHALL exit 1 with the message `embedded asset '<name>' not found — run 'torec asset show' to see the available asset names`; the refusal SHALL still prevent any file write, and the error doubles as the feature's teaching surface: the finite set of valid override names is exactly what `torec asset show` prints. On success, the command SHALL log `[assets] synced <name> to <path>; restart 'torec serve' to pick up the change`. The on-disk write is the only effect — the running daemon's in-memory cache is not touched.

#### Scenario: Sync a new override
- **WHEN** `torec asset sync question-generation-policy` is invoked and `<data-dir>/prompts/question-generation-policy.md` does not exist
- **THEN** the file is created with the embedded bytes; stdout contains the restart advisory; exit 0

#### Scenario: Sync an existing non-empty override without --force refuses
- **WHEN** `torec asset sync question-generation-policy` is invoked, the target file exists and is non-empty, and `--force` is not passed
- **THEN** exit 1 with `error: <path> exists and is non-empty — pass --force to overwrite, or 'torec asset reset <name>' to start from defaults`; no file is modified

#### Scenario: Sync with --force overwrites the existing override
- **WHEN** `torec asset sync question-generation-policy --force` is invoked
- **THEN** the file is overwritten with the embedded bytes; stdout contains the restart advisory; exit 0

#### Scenario: Sync with no name refuses
- **WHEN** `torec asset sync` is invoked with no arguments
- **THEN** exit 1 with `error: sync requires an explicit asset name`; no file is modified

#### Scenario: Sync when the data dir cannot be resolved
- **WHEN** `torec asset sync <name>` is invoked and neither `TR_HOME` nor a home directory can be resolved
- **THEN** exit 1 with `could not resolve the Total Recall data dir`; no file is touched

#### Scenario: Sync when embedded bytes are unavailable / unknown name points at the inventory
- **WHEN** `torec asset sync ecs-policy` is invoked and the binary's embedded map contains no shipped asset named `ecs-policy`
- **THEN** exit 1; the message contains `not found` and the `torec asset show` pointer; no file is touched

#### Scenario: Valid name is unaffected
- **WHEN** `torec asset sync question-generation-policy` is invoked
- **THEN** behavior is unchanged per the existing sync requirements (embedded bytes written to the correctly-named file)

---

### Requirement: `tr asset reset|sync` validate the `<name>` argument before path construction
`torec asset reset` and `torec asset sync` SHALL validate an explicit `<name>` argument against `^[a-z0-9-]+$` (a single lowercase-hyphenated token, matching shipped asset naming) before joining it into a path. An argument that is empty after trimming, contains path separators, dots, whitespace, or characters outside the pattern SHALL be rejected with exit 1 and a message naming the offending argument and the expected form (`invalid asset name '<arg>' — expected a single lowercase-hyphenated name, e.g. 'question-generation-policy'`). No file operation SHALL be attempted for an invalid name. The batch (no-argument) forms of `reset` are not affected — they enumerate the override directory directly.

#### Scenario: Canonical name is accepted
- **WHEN** `torec asset reset question-generation-policy` is invoked (or the `sync` equivalent)
- **THEN** the name passes validation and the command behaves per its existing requirements (remove the override / write the embedded bytes)

#### Scenario: Traversal-shaped name is rejected
- **WHEN** `torec asset reset ../../sensitive` or `torec asset sync ../prompts` is invoked
- **THEN** exit 1 with `invalid asset name` and the expected form; no path outside the override directory is constructed; no file is removed or written

#### Scenario: Odd characters are rejected
- **WHEN** `torec asset sync "Question Policy!"` or `torec asset reset "policy.md"` (`.md` suffix supplied by the user) is invoked
- **THEN** exit 1 with `invalid asset name` and the expected form; no file is touched

---

### Requirement: `tr asset` self-documents the override loop in long help
The `asset` command SHALL carry long help text (Cobra `Long`) that explains, in plain language: what prompt-asset overrides are (a markdown file in the data dir's `prompts/` directory replacing the shipped default), the inspect → recover loop (`show` / `reset` / `sync`), and the restart caveat (the daemon picks up file mutations on next `torec serve` start). `torec asset --help` and `torec help asset` SHALL both render it. The help text is user documentation in the terminal — no new flags or subcommands are introduced by this requirement.

#### Scenario: Long help renders the override loop
- **WHEN** `torec asset --help` (or `torec help asset`) is invoked
- **THEN** the output describes what an override is, when to use `show` vs `reset` vs `sync`, and the restart caveat — not just the one-line command summaries
