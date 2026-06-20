## MODIFIED Requirements

### Requirement: Named provider registry resolves base URLs
`ai.New(cfg AIConfig)` SHALL look up `cfg.Provider` in a named registry. If the provider is a known preset (`anthropic`, `openai`, `ollama`, `groq`, `lm-studio`, `qwen`, `minimax`, `deepseek`), the registry base URL is used and `cfg.BaseURL` is ignored. If the provider is `custom`, `cfg.BaseURL` is required and `ai.New` returns `ErrNoProvider` if it is empty. Unknown provider names also return `ErrNoProvider`.

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

#### Scenario: Custom provider requires BaseURL
- **WHEN** `cfg.Provider` is `"custom"` and `cfg.BaseURL` is empty
- **THEN** `ai.New()` returns `ErrNoProvider`

#### Scenario: Unknown provider returns ErrNoProvider
- **WHEN** `cfg.Provider` is an unrecognised string
- **THEN** `ai.New()` returns `ErrNoProvider`
