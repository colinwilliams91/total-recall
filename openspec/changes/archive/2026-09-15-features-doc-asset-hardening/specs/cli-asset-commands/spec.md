## ADDED Requirements

### Requirement: `tr asset reset|sync` validate the `<name>` argument before path construction
`tr asset reset` and `tr asset sync` SHALL validate an explicit `<name>` argument against `^[a-z0-9-]+$` (a single lowercase-hyphenated token, matching shipped asset naming) before joining it into a path. An argument that is empty after trimming, contains path separators, dots, whitespace, or characters outside the pattern SHALL be rejected with exit 1 and a message naming the offending argument and the expected form (`invalid asset name '<arg>' — expected a single lowercase-hyphenated name, e.g. 'question-generation-policy'`). No file operation SHALL be attempted for an invalid name. The batch (no-argument) forms of `reset` are not affected — they enumerate the override directory directly.

#### Scenario: Canonical name is accepted
- **WHEN** `tr asset reset question-generation-policy` is invoked (or the `sync` equivalent)
- **THEN** the name passes validation and the command behaves per its existing requirements (remove the override / write the embedded bytes)

#### Scenario: Traversal-shaped name is rejected
- **WHEN** `tr asset reset ../../sensitive` or `tr asset sync ../prompts` is invoked
- **THEN** exit 1 with `invalid asset name` and the expected form; no path outside the override directory is constructed; no file is removed or written

#### Scenario: Odd characters are rejected
- **WHEN** `tr asset sync "Question Policy!"` or `tr asset reset "policy.md"` (`.md` suffix supplied by the user) is invoked
- **THEN** exit 1 with `invalid asset name` and the expected form; no file is touched

---

### Requirement: `tr asset` self-documents the override loop in long help
The `asset` command SHALL carry long help text (Cobra `Long`) that explains, in plain language: what prompt-asset overrides are (a markdown file in the data dir's `prompts/` directory replacing the shipped default), the inspect → recover loop (`list` / `reset` / `sync`), and the restart caveat (the daemon picks up file mutations on next `tr serve` start). `tr asset --help` and `tr help asset` SHALL both render it. The help text is user documentation in the terminal — no new flags or subcommands are introduced by this requirement.

#### Scenario: Long help renders the override loop
- **WHEN** `tr asset --help` (or `tr help asset`) is invoked
- **THEN** the output describes what an override is, when to use `list` vs `reset` vs `sync`, and the restart caveat — not just the one-line command summaries
