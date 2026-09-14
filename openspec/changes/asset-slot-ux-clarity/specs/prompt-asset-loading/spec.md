## ADDED Requirements

### Requirement: Daemon startup logs unmanaged override-slot files
At daemon startup, after the drift threshold is wired, the daemon SHALL enumerate the override slot's `.md` files and, for each file whose name does not correspond to any shipped (embedded) asset, log one line: `[assets] unmanaged file %q in prompts/ — no shipped asset with this name; not active (run 'tr asset list')`. The warning is informational only — it SHALL NOT block daemon startup, SHALL NOT refuse to load any asset, SHALL NOT remove or rename files, and SHALL NOT fire for shipped-name overrides or when the slot directory is empty or absent. Files the user consciously keeps unmanaged MAY be silenced by removing, renaming, or relocating them; no configuration gate is provided.

#### Scenario: Unmanaged file is called out at startup
- **WHEN** the daemon starts and `<data-dir>/prompts/my-experiment.md` exists but no shipped asset is named `my-experiment`
- **THEN** the startup log contains the unmanaged-file line naming `my-experiment.md`

#### Scenario: Active override produces no unmanaged warning
- **WHEN** the daemon starts and the slot contains only `<data-dir>/prompts/question-generation-policy.md`
- **THEN** no unmanaged-file log line is emitted

#### Scenario: Empty or absent slot is silent
- **WHEN** the daemon starts and the slot directory is empty, absent, or unresolvable (no data dir)
- **THEN** no unmanaged-file log line is emitted
