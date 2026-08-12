## ADDED Requirements

### Requirement: `tr asset list` enumerates resolved prompt assets in plain text
`tr asset list` SHALL enumerate every prompt asset known to the daemon. For each asset, it SHALL print one line with four tab-separated columns: `<name>`, `<source>` (one of `embedded`, `$TR_HOME`, `fallback`), `<resolved-path>` (the absolute path the asset was loaded from, or `<embedded>` when no override is in effect), and `<age>` (a human-readable mtime-relative string such as `embedded`, `6mo old`, or `2d old`). The output SHALL be plain text suitable for `awk`/`grep` parsing — no JSON, no table decoration. The command SHALL NOT require the daemon to be running (it reads the filesystem state of `$TR_HOME/prompts/` directly, plus the embedded bytes available from the binary).

#### Scenario: No overrides — embedded default
- **WHEN** `tr asset list` is invoked and `$TR_HOME` is unset
- **THEN** the output contains a line for the canonical asset (`question-generation-policy`) with `embedded` source and `embedded` age

#### Scenario: One override present
- **WHEN** `tr asset list` is invoked with `$TR_HOME` set and `$TR_HOME/prompts/question-generation-policy.md` exists
- **THEN** the output contains a line with `$TR_HOME` source, the absolute path to the override file, and a human-readable mtime age (e.g., `2d old`, `6mo old`)

#### Scenario: Multiple overrides
- **WHEN** `tr asset list` is invoked with `$TR_HOME` set and two distinct `.md` files exist under `$TR_HOME/prompts/`
- **THEN** the output contains one line per distinct asset name; non-overridden embedded assets still appear with `embedded` source

#### Scenario: Daemon not running — list still works
- **WHEN** `tr asset list` is invoked and the daemon is not reachable
- **THEN** the command exits 0 with valid output — `list` reads disk state and the binary's embedded bytes, not daemon state

---

### Requirement: `tr asset reset [<name>]` removes an override file
`tr asset reset` SHALL remove the override file at `$TR_HOME/prompts/<name>.md`. With an explicit `<name>` argument, it SHALL remove only that file; when the file does not exist, it SHALL exit 0 with a short "no override for <name> — nothing to reset" message (no error). With no `<name>` argument, it SHALL enumerate every `.md` under `$TR_HOME/prompts/` and remove them as a batch — gated by a `--all` flag and a TTY confirmation prompt. The command SHALL refuse to run when `$TR_HOME` is unset (no overrides could exist) with exit code 1 and a `TR_HOME is not set; nothing to reset` message. On success, the command SHALL log a restart advisory: `[assets] removed override at <path>; restart 'tr serve' to pick up the change`. The on-disk removal is the only effect — the running daemon's in-memory cache is not touched; the change takes effect at the next daemon restart.

#### Scenario: Reset a single existing override
- **WHEN** `tr asset reset question-generation-policy` is invoked and `$TR_HOME/prompts/question-generation-policy.md` exists
- **THEN** the file is removed; stdout contains the restart advisory; exit 0

#### Scenario: Reset a single non-existent override is a no-op
- **WHEN** `tr asset reset question-generation-policy` is invoked and the override file does not exist
- **THEN** stdout is short ("no override for question-generation-policy — nothing to reset"); exit 0; no file is touched

#### Scenario: Reset with no args requires --all
- **WHEN** `tr asset reset` is invoked with no arguments and two override files exist under `$TR_HOME/prompts/`
- **THEN** the command refuses and exits non-zero with a message instructing `--all --force` for batch removal

#### Scenario: Reset --all --force in TTY removes all overrides
- **WHEN** `tr asset reset --all --force` is invoked in an interactive TTY and two overrides exist
- **THEN** both files are removed; stdout contains the restart advisory; exit 0

#### Scenario: Reset when TR_HOME is unset
- **WHEN** `tr asset reset <name>` is invoked and `$TR_HOME` is unset
- **THEN** exit 1 with `TR_HOME is not set; nothing to reset`; no file is touched

---

### Requirement: `tr asset sync [<name>]` writes the canonical embedded content to an override slot
`tr asset sync <name>` SHALL read the embedded bytes for `<name>.md` (per the `//go:embed` pattern from `synthesize-from-context`) and write them to `$TR_HOME/prompts/<name>.md`. When the target file exists and is non-empty, the command SHALL refuse with exit 1 and a message instructing `--force` to overwrite or `tr asset reset <name>` to start from defaults. With `--force`, the command SHALL overwrite the existing file. The command SHALL refuse to run with no `<name>` argument (sync-all is too easily mistaken for destructive default-restoration; an explicit name forces intent). When `$TR_HOME` is unset, the command SHALL exit 1 with `TR_HOME is not set; nothing to sync to`. When the embedded bytes for `<name>` are unavailable (corrupt build), the command SHALL exit 1 with `embedded asset <name> not found`. On success, the command SHALL log `[assets] synced <name> to <path>; restart 'tr serve' to pick up the change`. The on-disk write is the only effect — the running daemon's in-memory cache is not touched.

#### Scenario: Sync a new override
- **WHEN** `tr asset sync question-generation-policy` is invoked and `$TR_HOME/prompts/question-generation-policy.md` does not exist
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

#### Scenario: Sync when TR_HOME is unset
- **WHEN** `tr asset sync <name>` is invoked and `$TR_HOME` is unset
- **THEN** exit 1 with `TR_HOME is not set; nothing to sync to`; no file is touched

#### Scenario: Sync when embedded bytes are unavailable
- **WHEN** `tr asset sync <unknown-name>` is invoked and the binary's embedded map does not contain `<unknown-name>.md`
- **THEN** exit 1 with `embedded asset <unknown-name> not found`; no file is touched