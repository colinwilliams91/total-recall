## ADDED Requirements

### Requirement: `tr init` prompts the user about conversation analysis opt-in
The system SHALL present a single plain-language prompt during `tr init` asking whether to enable conversation analysis. The prompt SHALL be simple and earnest, with no legal jargon.

#### Scenario: User opts in during init
- **WHEN** the user answers `y` to the conversation analysis prompt during `tr init`
- **THEN** `~/.tr/config.yaml` SHALL be written with `privacy.conversation_analysis: true`
- **THEN** the system SHALL confirm the choice and inform the user it can be changed in `~/.tr/config.yaml`

#### Scenario: User opts out or skips during init
- **WHEN** the user answers `n` or presses Enter (accepting the default) at the conversation analysis prompt
- **THEN** `~/.tr/config.yaml` SHALL be written with `privacy.conversation_analysis: false`
- **THEN** the system SHALL confirm and inform the user conversation analysis can be enabled later in `~/.tr/config.yaml`

#### Scenario: Auto-created config defaults conversation analysis to off
- **WHEN** `~/.tr/config.yaml` is auto-created because init was bypassed
- **THEN** `privacy.conversation_analysis` SHALL default to `false`
- **THEN** no prompt is shown — the advisory message directs the user to `tr init` to configure preferences

---
The system SHALL require explicit opt-in before processing MCP conversation content (Signal 1: user questions; Signal 2: agent explanations) as input signals to the Incremental Analysis Pipeline. The default SHALL be off.

#### Scenario: Conversation analysis disabled by default
- **WHEN** `privacy.conversation_analysis` is absent or set to `false`
- **THEN** the MCP Event Monitor SHALL NOT forward conversation content to the Incremental Analysis Pipeline
- **THEN** MCP SHALL still function as a presentation output surface (delivering recall prompts)

#### Scenario: Conversation analysis enabled by user opt-in
- **WHEN** `privacy.conversation_analysis: true` is set in `~/.tr/config.yaml`
- **THEN** the MCP Event Monitor SHALL forward conversation signals (Signal 1 + Signal 2) to the Incremental Analysis Pipeline for concept extraction

---

### Requirement: Code context correlation is unaffected by the conversation analysis flag
The system SHALL always correlate MCP agent code context (Signal 3) with the Background Concept Cache regardless of the `conversation_analysis` flag, because this code is already captured by the Filesystem and Git Index watchers.

#### Scenario: Code context correlated when flag is off
- **WHEN** `privacy.conversation_analysis: false`
- **AND** an MCP agent is viewing code already present in the Background Concept Cache
- **THEN** the Core Engine SHALL correlate the active MCP session with the cached concept state for that code
- **THEN** this correlation SHALL enrich recall synthesis without processing any conversation text

---

### Requirement: Raw conversation text is never persisted
The system SHALL discard raw conversation text after concept extraction. Only structured concept fingerprints SHALL be written to the Background Concept Cache.

#### Scenario: Concept extracted, source text discarded
- **WHEN** `conversation_analysis: true` and the Incremental Analysis Pipeline processes a conversation signal
- **THEN** the pipeline SHALL extract concept fingerprints (concept tags, confidence scores, code grounding references)
- **THEN** the pipeline SHALL NOT write the raw conversation text to any persistent store
- **THEN** only the structured extraction result SHALL be written to the Background Concept Cache

