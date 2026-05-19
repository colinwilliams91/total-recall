## ADDED Requirements

### Requirement: Per-repo config overrides user-level defaults
The system SHALL implement a two-tier config precedence model where per-repo `.tr.yaml` values override `~/.tr/config.yaml` defaults on any conflicting key.

#### Scenario: Per-repo key overrides user default
- **WHEN** both `~/.tr/config.yaml` and `.tr.yaml` define the same key (e.g., `recall.max_questions`)
- **THEN** the resolved config SHALL use the value from `.tr.yaml`

#### Scenario: User default used when per-repo key absent
- **WHEN** a key is defined in `~/.tr/config.yaml` but not in `.tr.yaml`
- **THEN** the resolved config SHALL use the value from `~/.tr/config.yaml`

#### Scenario: Per-repo config absent entirely
- **WHEN** no `.tr.yaml` exists in the current repository
- **THEN** the Core Engine SHALL use `~/.tr/config.yaml` values exclusively with no error

---

### Requirement: Merge is a deep merge, not a shallow replacement
The system SHALL deep-merge the two config files such that nested keys are merged independently rather than replacing entire top-level blocks.

#### Scenario: Partial block override preserves sibling keys
- **WHEN** `.tr.yaml` defines `recall.max_questions: 2` but does not define `recall.difficulty`
- **THEN** the resolved config SHALL have `recall.max_questions: 2` (from repo) AND `recall.difficulty: adaptive` (from user default)
- **THEN** the user-level `recall.difficulty` SHALL NOT be discarded because `.tr.yaml` touched the `recall` block

---

### Requirement: Privacy keys are not overridable per-repo
The system SHALL treat `privacy.*` keys as user-level only. Per-repo `.tr.yaml` SHALL NOT override privacy settings.

#### Scenario: Privacy key in per-repo config is ignored
- **WHEN** `.tr.yaml` contains a `privacy` block
- **THEN** the Core Engine SHALL ignore the per-repo `privacy` block
- **THEN** the Core Engine SHALL emit a warning that privacy settings are user-level only

---

### Requirement: Resolved config is inspectable
The system SHALL provide a CLI command to display the fully resolved config for the current context.

#### Scenario: Developer inspects resolved config
- **WHEN** the user runs `tr config --show`
- **THEN** the system SHALL print the resolved (merged) config, annotating each key with its source (`user` or `repo`)
