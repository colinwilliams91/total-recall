## MODIFIED Requirements

### Requirement: tr init presents a named provider picker
`runInit()` SHALL include an AI provider selection step. The picker SHALL present named options with friendly descriptions — users never see internal details like base URLs or adapter package names. `tr init` does NOT include any hook-selection step (the hooks section moved to `tr repo` in Phase Y3).

#### Scenario: User selects Anthropic
- **WHEN** the user picks `Anthropic (Claude)` in the provider picker
- **THEN** the TUI shows a follow-up prompt for API key (pre-filled with `env:ANTHROPIC_API_KEY`) and model (pre-filled with `claude-sonnet-4-5`), with an inline explanation of the `env:VAR_NAME` pattern

#### Scenario: User selects Ollama
- **WHEN** the user picks `Ollama (local · free · runs on your machine)`
- **THEN** the TUI shows a model name input with the hint `"Run 'ollama list' to see installed models."` and no API key prompt

#### Scenario: User selects Custom
- **WHEN** the user picks `Custom (advanced)`
- **THEN** the TUI shows three inputs: base URL (with example `http://localhost:8080/v1`), model name, and optional API key

---

### Requirement: tr init pre-populates from existing config
If `~/.tr/config.yaml` already contains an `ai` block, all provider prompts SHALL be pre-populated with the existing values. The user can confirm or change each value. `tr init` does NOT load or modify any `.tr.yaml` repo-config; re-running `tr init` only re-prompts user-level questions.

#### Scenario: Re-running tr init with existing config
- **WHEN** `~/.tr/config.yaml` has `provider: anthropic`, `model: claude-sonnet-4-5`, `api-key: env:ANTHROPIC_API_KEY`
- **THEN** the provider picker defaults to Anthropic, the API key input pre-fills with `env:ANTHROPIC_API_KEY`, and the model input pre-fills with `claude-sonnet-4-5`

---

### Requirement: Config is written via template writer after AI setup
After the AI provider prompts complete, `runInit()` SHALL write `~/.tr/config.yaml` using the template writer (not `yaml.Marshal`). The resulting file SHALL include inline comments for every field, including `base-url` (blank with explanatory comment for non-custom providers).

#### Scenario: Config file after Ollama setup
- **WHEN** the user completes `tr init` with Ollama selected
- **THEN** `~/.tr/config.yaml` contains `provider: ollama`, the correct model, `api-key: ollama`, a blank `base-url:` field, and inline comments explaining each field

---

### Requirement: Cloud provider prompts explain the env:VAR_NAME pattern
For cloud providers that require an API key, the TUI prompt description SHALL explain the `env:VAR_NAME` pattern in plain language, advising users to set the variable in their shell profile rather than pasting the key directly.

#### Scenario: API key prompt description
- **WHEN** the user is at the API key input for any cloud provider
- **THEN** the prompt includes copy similar to: `"Use env:VAR_NAME so your key is never stored in plaintext. Example: env:ANTHROPIC_API_KEY. Set this variable in your ~/.zshrc or ~/.bashrc."`

---

### Requirement: tr init does not touch git or hooks
`runInit()` SHALL NOT call `hooks.FindRepoRoot`, `hooks.ResolveHooksDir`, or any hooks-installer method. It SHALL NOT mention git or hooks in any prompt or printed message. After writing `~/.tr/config.yaml`, it SHALL print exactly `Next: cd into your project and run tr repo.` (or equivalent wording clearly guiding the user to `tr repo` as the next step) and return.

#### Scenario: tr init run from outside a git repo
- **WHEN** `tr init` is run from a directory that is not inside any git repository
- **THEN** `tr init` writes `~/.tr/config.yaml`, prints the next-step guidance (`Next: cd into your project and run tr repo.`), and exits 0; no warning about "not in a git repo" is printed

#### Scenario: tr init run from inside a git repo
- **WHEN** `tr init` is run from inside a git repository
- **THEN** `tr init` behaves identically to running from outside a git repo — it writes only `~/.tr/config.yaml`, prints the next-step guidance, and exits 0; no `.tr.yaml` is written, no hooks are installed

---

### Requirement: Cloud provider prompts explain the env:VAR_NAME pattern
For cloud providers that require an API key, the TUI prompt description SHALL explain the `env:VAR_NAME` pattern in plain language, advising users to set the variable in their shell profile rather than pasting the key directly.

#### Scenario: API key prompt description
- **WHEN** the user is at the API key input for any cloud provider
- **THEN** the prompt includes copy similar to: `"Use env:VAR_NAME so your key is never stored in plaintext. Example: env:ANTHROPIC_API_KEY. Set this variable in your ~/.zshrc or ~/.bashrc."`