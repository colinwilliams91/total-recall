## MODIFIED Requirements

### Requirement: tr init performs PATH detection before any prompts
`runInit()` SHALL, as its first action before any user-facing prompts are shown, perform PATH detection per the `path-detection-warning` capability specification. Specifically: invoke the `checkTrOnPath()` helper (defined in the same package or a small `cmd/tr/pathdetect.go` file). On Unix, the helper uses `exec.LookPath("tr")` (stdlib, no shell dependency). On Windows, the helper invokes `Get-Command tr` via `exec.Command("powershell.exe", "-NoProfile", "-Command", "(Get-Command tr -ErrorAction SilentlyContinue).Name")`. If `tr` is found, return silently. If not, print a shell-specific warning to stderr per the `path-detection-warning` spec.

After the detection call, `tr init` SHALL proceed with the existing conversation-analysis opt-in prompt and AI provider form, regardless of detection result. The detection warning is informational only and does not change the rest of the `tr init` flow.

#### Scenario: tr on PATH — silent
- **WHEN** `tr init` is run and `exec.LookPath("tr")` returns no error (Unix) or `Get-Command tr` finds the binary (Windows)
- **THEN** no PATH-related output is printed; `tr init` proceeds with the conversation-analysis opt-in prompt

#### Scenario: tr not on PATH — warning printed
- **WHEN** `tr init` is run and `exec.LookPath("tr")` returns a "not in PATH" error (Unix) or `Get-Command tr` returns empty (Windows)
- **THEN** a shell-specific one-line warning is printed to stderr (see `path-detection-warning` spec for exact format); `tr init` then proceeds with the conversation-analysis opt-in prompt

---

### Requirement: tr init presents a named provider picker
`runInit()` SHALL include an AI provider selection step that runs AFTER the PATH-detection check. The picker SHALL present named options with friendly descriptions — users never see internal details like base URLs or adapter package names. `tr init` does NOT include any hook-selection step (the hooks section moved to `tr repo` in Phase Y3).

#### Scenario: User selects Anthropic
- **WHEN** the user picks `Anthropic (Claude)` in the provider picker
- **THEN** the TUI shows a follow-up prompt for API key (pre-filled with `env:ANTHROPIC_API_KEY`) and model (pre-filled with `claude-sonnet-4-5`), with an inline explanation of the `env:VAR_NAME` pattern

#### Scenario: User selects Ollama
- **WHEN** the user picks `Ollama (local · free · runs on your machine)`
- **THEN** the TUI shows a model name input with the hint `"Run 'ollama list' to see installed models."` and no API key prompt

#### Scenario: User selects Custom
- **WHEN** the user picks `Custom (advanced)`
- **THEN** the TUI shows three inputs: base URL (with example `http://localhost:8080/v1`), model name, and optional API key