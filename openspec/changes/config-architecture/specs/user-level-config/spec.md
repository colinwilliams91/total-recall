## ADDED Requirements

### Requirement: User-level config file exists at a well-known path
The system SHALL support a user-level configuration file at `~/.tr/config.yaml` that defines personal defaults applying across all repositories.

#### Scenario: User-level config loaded at daemon startup
- **WHEN** `tr serve` is invoked
- **THEN** the Core Engine SHALL load `~/.tr/config.yaml` before loading any per-repo config

#### Scenario: User-level config loaded when hook contacts daemon
- **WHEN** a Git hook fires and POSTs to `localhost:7331`
- **THEN** the running daemon (which already holds the merged config) SHALL handle the request — hooks do NOT load config themselves

#### Scenario: User-level config absent on first run
- **WHEN** the Core Engine starts and `~/.tr/config.yaml` does not exist
- **THEN** the engine SHALL auto-create `~/.tr/config.yaml` with safe defaults, emit an advisory message to stderr (unless `--quiet` is set), and continue normally

---

### Requirement: `tr init` creates user-level config with safe defaults
The system SHALL create `~/.tr/config.yaml` with safe default values when `tr init` is run for the first time.

#### Scenario: First-time init generates user config
- **WHEN** `tr init` is run and `~/.tr/config.yaml` does not yet exist
- **THEN** the system SHALL create `~/.tr/config.yaml` with the following defaults:
  ```yaml
  privacy:
    conversation_analysis: false

  ai:
    provider: anthropic
    model: claude-sonnet
    api-key: env:ANTHROPIC_API_KEY

  recall:
    difficulty: adaptive
    max_questions: 1
  ```

#### Scenario: Init does not overwrite existing user config
- **WHEN** `tr init` is run and `~/.tr/config.yaml` already exists
- **THEN** the system SHALL NOT overwrite the existing user config
- **THEN** the system SHALL notify the user that user config already exists

#### Scenario: Init prompts user about conversation analysis opt-in
- **WHEN** `tr init` creates a new `~/.tr/config.yaml`
- **THEN** the system SHALL display a plain-language prompt asking whether to enable conversation analysis (default: N)
- **THEN** the prompt SHALL use simple, earnest language with no legal jargon
- **THEN** the system SHALL set `privacy.conversation_analysis` according to the user's response
- **THEN** the system SHALL confirm the choice and note it can be changed in `~/.tr/config.yaml`

---

### Requirement: User config is auto-created with safe defaults if init is bypassed
The system SHALL auto-create `~/.tr/config.yaml` with safe defaults if the Core Engine starts and no user config exists, rather than failing hard.

#### Scenario: Daemon starts without user config present
- **WHEN** `tr serve` is invoked and `~/.tr/config.yaml` does not exist
- **THEN** the Core Engine SHALL create `~/.tr/config.yaml` with safe defaults (including `privacy.conversation_analysis: false`)
- **THEN** the Core Engine SHALL emit a single advisory message informing the user and suggesting they run `tr init`
- **THEN** the Core Engine SHALL continue starting normally

#### Scenario: Hook fires without user config present
- **WHEN** a Git hook triggers a transient Core Engine invocation and `~/.tr/config.yaml` does not exist
- **THEN** the Core Engine SHALL create `~/.tr/config.yaml` with safe defaults
- **THEN** the Core Engine SHALL emit a single advisory message and continue without blocking the hook

#### Scenario: Auto-created config is not overwritten on subsequent runs
- **WHEN** `~/.tr/config.yaml` was auto-created on a previous run
- **THEN** any subsequent invocation SHALL load the existing file and SHALL NOT recreate or overwrite it

---

### Requirement: AI credentials live at user-level only
The system SHALL read AI provider credentials (`ai.provider`, `ai.model`, `ai.api-key`) from `~/.tr/config.yaml`. These keys SHALL NOT be required in `.tr.yaml`.

#### Scenario: API key resolved from environment variable reference
- **WHEN** `ai.api-key` is set to a value in the format `env:<VAR_NAME>` (e.g., `env:ANTHROPIC_API_KEY`)
- **THEN** the Core Engine SHALL resolve the value from the named environment variable at runtime

#### Scenario: Raw API key string detected in config
- **WHEN** `ai.api-key` contains a value that does not match the `env:<VAR_NAME>` pattern
- **THEN** the Core Engine SHALL emit a warning recommending the environment variable pattern
- **THEN** the Core Engine SHALL still proceed using the raw value
