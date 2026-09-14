## ADDED Requirements

### Requirement: `tr asset list` tags slot files that shadow no shipped asset as `inactive`
`tr asset list` SHALL classify each entry in the four-column output by whether the file's name corresponds to a shipped (embedded) asset. Files under the data dir's `prompts/` directory whose name does not correspond to any shipped asset SHALL be printed with source `inactive` — with their resolved path and human age still populated — instead of the `$TR_HOME` tag used for active overrides. A file listed as `inactive` SHALL NOT be loaded by the engine and SHALL NOT affect quiz generation; the tag exists to make that fact visible. Active overrides and embedded defaults are unaffected and keep their existing tags. The command SHALL NOT require the daemon for this classification (it is derivable from `EmbeddedNames()` and directory contents alone).

#### Scenario: Shipped-name override remains active
- **WHEN** `tr asset list` is invoked and `<data-dir>/prompts/question-generation-policy.md` exists
- **THEN** the line for `question-generation-policy` carries source `$TR_HOME` as before

#### Scenario: Unmanaged file is tagged inactive
- **WHEN** `tr asset list` is invoked and `<data-dir>/prompts/my-experiment.md` exists (no shipped asset with that name)
- **THEN** the line for `my-experiment` carries source `inactive`, with the resolved path and a human age; no log or list claim implies the file is loaded

#### Scenario: Cleanup path still works on unmanaged files
- **WHEN** `tr asset reset my-experiment` is invoked after the tagging above
- **THEN** the file is removed and the restart advisory prints, per the existing reset behavior (reset is the cleanup path for unmanaged files)

---

### Requirement: `tr asset sync <name>` unknown-asset error teaches the way out
When `tr asset sync <name>` is invoked and the running binary has no embedded asset for `<name>`, the command SHALL exit 1 with the message `embedded asset '<name>' not found — run 'tr asset list' to see the available asset names`. The refusal SHALL still prevent any file write. The error doubles as the feature's teaching surface: the finite set of valid override names is exactly what `tr asset list` prints.

#### Scenario: Unknown sync name points at the inventory
- **WHEN** `tr asset sync ecs-policy` is invoked and no shipped asset named `ecs-policy` exists
- **THEN** exit 1; the message contains `not found` and the `tr asset list` pointer; no file is written

#### Scenario: Valid name is unaffected
- **WHEN** `tr asset sync question-generation-policy` is invoked
- **THEN** behavior is unchanged per the existing sync requirements (embedded bytes written to the correctly-named file)
