## MODIFIED Requirements

### Requirement: tr init performs PATH detection before any prompts
`runInit()` SHALL, as its first action before any user-facing prompts are shown, perform PATH detection per the `path-detection-warning` capability specification. Specifically: invoke the `checkTrOnPath()` helper (defined in the same package or a small `cmd/torec/pathdetect.go` file). On Unix, the helper uses `exec.LookPath("torec")` (stdlib, no shell dependency). On Windows, the helper invokes `Get-Command torec` via `exec.Command("powershell.exe", "-NoProfile", "-Command", "(Get-Command torec -ErrorAction SilentlyContinue).Name")`. If `torec` is found, return silently. If not, print a shell-specific warning to stderr per the `path-detection-warning` spec.

After the detection call, `torec init` SHALL proceed with the existing conversation-analysis opt-in prompt and AI provider form, regardless of detection result. The detection warning is informational only and does not change the rest of the `torec init` flow.

#### Scenario: tr on PATH — silent
- **WHEN** `torec init` is run and `exec.LookPath("torec")` returns no error (Unix) or `Get-Command torec` finds the binary (Windows)
- **THEN** no PATH-related output is printed; `torec init` proceeds with the conversation-analysis opt-in prompt

#### Scenario: tr not on PATH — warning printed
- **WHEN** `torec init` is run and `exec.LookPath("torec")` returns a "not in PATH" error (Unix) or `Get-Command torec` returns empty (Windows)
- **THEN** a shell-specific one-line warning is printed to stderr (see `path-detection-warning` spec for exact format); `torec init` then proceeds with the conversation-analysis opt-in prompt

---

### Requirement: tr init presents a named provider picker
`runInit()` SHALL include an AI provider selection step that runs AFTER the PATH-detection check. The picker SHALL present named options with friendly descriptions — users never see internal details like base URLs or adapter package names. `torec init` does NOT include any hook-selection step (the hooks section moved to `torec repo` in Phase Y3).

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
If `~/.torec/config.yaml` already contains an `ai` block, all provider prompts SHALL be pre-populated with the existing values. The user can confirm or change each value. `torec init` does NOT load or modify any `.torec.yaml` repo-config; re-running `torec init` only re-prompts user-level questions.

#### Scenario: Re-running tr init with existing config
- **WHEN** `~/.torec/config.yaml` has `provider: anthropic`, `model: claude-sonnet-4-5`, `api-key: env:ANTHROPIC_API_KEY`
- **THEN** the provider picker defaults to Anthropic, the API key input pre-fills with `env:ANTHROPIC_API_KEY`, and the model input pre-fills with `claude-sonnet-4-5`

---

### Requirement: Config is written via template writer after AI setup
After the AI provider prompts complete, `runInit()` SHALL write `~/.torec/config.yaml` using the template writer (not `yaml.Marshal`). The resulting file SHALL include inline comments for every field, including `base-url` (blank with explanatory comment for non-custom providers).

#### Scenario: Config file after Ollama setup
- **WHEN** the user completes `torec init` with Ollama selected
- **THEN** `~/.torec/config.yaml` contains `provider: ollama`, the correct model, `api-key: ollama`, a blank `base-url:` field, and inline comments explaining each field

---

### Requirement: tr init does not touch git or hooks
`runInit()` SHALL NOT call `hooks.FindRepoRoot`, `hooks.ResolveHooksDir`, or any hooks-installer method. It SHALL NOT mention git or hooks in any prompt or printed message. After writing `~/.torec/config.yaml`, it SHALL print exactly `Next: cd into your project and run torec repo.` (or equivalent wording clearly guiding the user to `torec repo` as the next step) and return.

#### Scenario: tr init run from outside a git repo
- **WHEN** `torec init` is run from a directory that is not inside any git repository
- **THEN** `torec init` writes `~/.torec/config.yaml`, prints the next-step guidance (`Next: cd into your project and run torec repo.`), and exits 0; no warning about "not in a git repo" is printed

#### Scenario: tr init run from inside a git repo
- **WHEN** `torec init` is run from inside a git repository
- **THEN** `torec init` behaves identically to running from outside a git repo — it writes only `~/.torec/config.yaml`, prints the next-step guidance, and exits 0; no `.torec.yaml` is written, no hooks are installed
