## Why

Total Recall currently supports 6 AI providers (Anthropic, OpenAI, Groq, Ollama, LM Studio, Custom). Users want to use Qwen (Alibaba Cloud Model Studio), MiniMax, and DeepSeek models for recall question generation. These are popular, high-quality OpenAI-compatible providers that should be first-class options in the TUI. Phase 4D extends provider support with minimal lift — all three route through the existing `openai.New()` adapter.

## What Changes

- Add `qwen` provider to registry with base URL `https://dashscope.aliyuncs.com/compatible-mode/v1`
- Add `minimax` provider to registry with base URL `https://api.minimaxi.com/v1`
- Add `deepseek` provider to registry with base URL `https://api.deepseek.com`
- Add TUI select options for each provider in `tr init`
- Add default model names: `qwen-max`, `MiniMax-M3`, `deepseek-v4-pro`
- Add API key env var placeholders: `QWEN_API_KEY`, `MINIMAX_API_KEY`, `DEEPSEEK_API_KEY`
- Update `ai-provider` spec to reflect new registry entries
- Add Phase 4D entry to `ROADMAP.md`

## Capabilities

### New Capabilities

None — this change extends existing provider infrastructure without introducing new capabilities.

### Modified Capabilities

- `ai-provider`: Provider registry gains 3 new entries (qwen, minimax, deepseek); all use OpenAI-compatible adapter

## Impact

- `internal/ai/provider.go` — `ProviderRegistry` map gains 3 entries
- `cmd/total-recall/main.go` — TUI select options, `providerModelDefaults`, `providerAPIKeyPlaceholders`
- `cmd/total-recall/wire.go` — error message lists new providers
- `openspec/specs/ai-provider/spec.md` — registry requirement updated
- `ROADMAP.md` — Phase 4D section added
- No new dependencies, no breaking changes, no API changes
