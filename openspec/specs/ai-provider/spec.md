## Requirements

### Requirement: Provider interface is a thin primitive
The `ai.Provider` interface SHALL expose a single method: `Complete(ctx context.Context, req CompletionRequest) (string, error)`. Domain concerns (concept extraction, question synthesis, prompt construction) are the responsibility of calling layers (`internal/pipeline/`, `internal/recall/`), not the adapter.

#### Scenario: Pipeline calls Complete for concept extraction
- **WHEN** the pipeline calls `provider.Complete()` with an extraction request
- **THEN** the adapter sends the request to the configured provider endpoint and returns the raw response string

#### Scenario: Recall engine calls Complete for question synthesis
- **WHEN** the recall engine calls `provider.Complete()` with a synthesis request
- **THEN** the adapter sends the request to the configured provider endpoint and returns the raw response string

---

### Requirement: CompletionRequest carries JSON mode hint
`CompletionRequest.JSON` SHALL be a boolean hint that the caller wants structured JSON output. Each adapter handles this in its own way: the `openai` adapter sets `response_format: {"type":"json_object"}`; the `anthropic` adapter appends `\n\nRespond with valid JSON only.` to the system prompt.

#### Scenario: OpenAI adapter with JSON mode
- **WHEN** `req.JSON == true` and the provider is openai-compatible
- **THEN** the request body includes `"response_format": {"type": "json_object"}`

#### Scenario: Anthropic adapter with JSON mode
- **WHEN** `req.JSON == true` and the provider is anthropic
- **THEN** the system prompt is extended with a JSON-only instruction

---

### Requirement: Named provider registry resolves base URLs
`ai.New(cfg AIConfig)` SHALL look up `cfg.Provider` in a named registry. If the provider is a known preset (`anthropic`, `openai`, `ollama`, `groq`, `lm-studio`, `qwen`, `minimax`, `deepseek`, `openrouter`), the registry base URL is used and `cfg.BaseURL` is ignored. If the provider is `custom`, `cfg.BaseURL` is required and `ai.New` returns `ErrNoProvider` if it is empty. Unknown provider names also return `ErrNoProvider`.

#### Scenario: Known provider resolves without BaseURL
- **WHEN** `cfg.Provider` is `"ollama"` and `cfg.BaseURL` is empty
- **THEN** `ai.New()` returns an openai-package client pointed at `http://localhost:11434/v1`

#### Scenario: Qwen provider resolves without BaseURL
- **WHEN** `cfg.Provider` is `"qwen"` and `cfg.BaseURL` is empty
- **THEN** `ai.New()` returns an openai-package client pointed at `https://dashscope.aliyuncs.com/compatible-mode/v1`

#### Scenario: MiniMax provider resolves without BaseURL
- **WHEN** `cfg.Provider` is `"minimax"` and `cfg.BaseURL` is empty
- **THEN** `ai.New()` returns an openai-package client pointed at `https://api.minimaxi.com/v1`

#### Scenario: DeepSeek provider resolves without BaseURL
- **WHEN** `cfg.Provider` is `"deepseek"` and `cfg.BaseURL` is empty
- **THEN** `ai.New()` returns an openai-package client pointed at `https://api.deepseek.com`

#### Scenario: OpenRouter provider resolves without BaseURL
- **WHEN** `cfg.Provider` is `"openrouter"` and `cfg.BaseURL` is empty
- **THEN** `ai.New()` returns an openai-package client pointed at `https://openrouter.ai/api/v1`

#### Scenario: Custom provider requires BaseURL
- **WHEN** `cfg.Provider` is `"custom"` and `cfg.BaseURL` is empty
- **THEN** `ai.New()` returns `ErrNoProvider`

#### Scenario: Unknown provider returns ErrNoProvider
- **WHEN** `cfg.Provider` is an unrecognised string
- **THEN** `ai.New()` returns `ErrNoProvider`

---

### Requirement: Nil provider results in no-AI mode, not a crash
If `ai.New()` returns `ErrNoProvider`, `serveCmd` SHALL log an advisory and pass a `nil` provider to `engine.New()`. The engine SHALL skip goroutine spawning for hook events when the provider is nil. Hooks are still acknowledged with 202 Accepted.

#### Scenario: Daemon starts without AI configured
- **WHEN** `cfg.AI.Provider` is empty and the daemon starts
- **THEN** the daemon logs `[daemon] AI provider not configured — recall questions will not be generated. Run 'total-recall init' to configure.` and proceeds without spawning AI goroutines

---

### Requirement: Adapters use raw net/http with no external SDK
Both `openai/client.go` and `anthropic/client.go` SHALL use only `net/http` and `encoding/json` from the standard library. No provider SDK packages are permitted in `go.mod`.

#### Scenario: go.mod remains SDK-free
- **WHEN** Phase 3 is complete
- **THEN** `go.mod` contains no `openai`, `anthropic`, or other provider SDK dependency
