## ADDED Requirements

### Requirement: OpenCode Go preset resolves without BaseURL
The provider registry SHALL include `opencode` as a known preset whose base URL is `https://opencode.ai/zen/go/v1`, served by the `openai` package adapter (Chat Completions API). The engine appends `/chat/completions`, matching the OpenCode Go chat-completions endpoint shape. The default starting model for the preset is `glm-5.3-flash` (chat-completions-routed; OpenCode Go routes Grok/GPT-Luna models via the OpenAI Responses API and Qwen/MiniMax via the Anthropic Messages API — those routes are out of scope for this preset). The API key is expected from the `OPENCODE_API_KEY` environment variable via the `env:` pattern.

#### Scenario: OpenCode provider resolves without BaseURL
- **WHEN** `cfg.Provider` is `"opencode"` and `cfg.BaseURL` is empty
- **THEN** the factory returns an openai-package client pointed at `https://opencode.ai/zen/go/v1`

#### Scenario: OpenCode default model
- **WHEN** `torec init` is run and the user selects `OpenCode Go` with no prior model configured
- **THEN** the model field is pre-filled with `glm-5.3-flash` and the API key input with `env:OPENCODE_API_KEY`

#### Scenario: OpenCode Go provider picker exposes the preset
- **WHEN** `torec init` presents the AI provider picker
- **THEN** an `OpenCode Go` option is present alongside the other cloud providers
