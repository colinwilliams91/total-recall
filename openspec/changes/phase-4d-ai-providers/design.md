## Context

Total Recall's AI provider system uses a registry pattern: `ProviderRegistry` maps provider names to base URLs, and `wire.go` routes each provider to the appropriate adapter (Anthropic native or OpenAI-compatible). Currently 6 providers are supported: `anthropic`, `openai`, `ollama`, `groq`, `lm-studio`, `custom`.

Qwen (Alibaba Cloud Model Studio), MiniMax, and DeepSeek are popular OpenAI-compatible providers. All three expose standard Chat Completions endpoints and require only a base URL and API key — no provider-specific SDK or adapter logic.

## Goals / Non-Goals

**Goals:**
- Add `qwen`, `minimax`, and `deepseek` as first-class providers in the registry
- Surface them in the `tr init` TUI with sensible defaults
- Update the `ai-provider` spec to reflect the expanded registry

**Non-Goals:**
- Adding provider-specific features (thinking mode, reasoning effort, etc.)
- Creating new adapter packages — all three use `openai.New()`
- Supporting provider-specific model lists or dynamic model discovery
- Handling provider-specific JSON mode quirks (all support standard `response_format`)

## Decisions

**1. Base URLs**

| Provider | Base URL | Notes |
|----------|----------|-------|
| `qwen` | `https://dashscope.aliyuncs.com/compatible-mode/v1` | Alibaba Cloud Model Studio / DashScope OpenAI-compatible endpoint |
| `minimax` | `https://api.minimaxi.com/v1` | MiniMax OpenAI-compatible Chat Completions endpoint |
| `deepseek` | `https://api.deepseek.com` | DeepSeek OpenAI-compatible endpoint (no `/v1` suffix) |

All verified against official provider documentation.

**2. Default models**

| Provider | Default Model | Rationale |
|----------|--------------|-----------|
| `qwen` | `qwen-max` | Flagship model; users can override to `qwen-plus`, `qwen-turbo`, `qwen3.7-max`, etc. |
| `minimax` | `MiniMax-M3` | Latest flagship; users can override to `MiniMax-M2.7`, etc. |
| `deepseek` | `deepseek-v4-pro` | Latest flagship; users can override to `deepseek-v4-flash` |

**3. Environment variable names**

| Provider | Env Var | Rationale |
|----------|---------|-----------|
| `qwen` | `QWEN_API_KEY` | User preference; DashScope docs use `DASHSCOPE_API_KEY` but `QWEN_API_KEY` is more intuitive |
| `minimax` | `MINIMAX_API_KEY` | Matches MiniMax documentation |
| `deepseek` | `DEEPSEEK_API_KEY` | Matches DeepSeek documentation |

**4. TUI display labels**

| Provider | TUI Label | Rationale |
|----------|-----------|-----------|
| `qwen` | `Qwen (Alibaba Cloud Model Studio)` | Matches OpenCode Go docs naming for discoverability |
| `minimax` | `MiniMax (e.g. MiniMax-M3)` | Clear model example |
| `deepseek` | `DeepSeek (e.g. deepseek-v4-pro)` | Clear model example |

**5. Adapter routing**

All three providers route through `openai.New()` in `wire.go` — same as `groq`, `ollama`, `lm-studio`, and `custom`. No changes to the `switch` statement needed; the existing `default` case handles them.

**6. TUI flow**

All three are cloud providers requiring an API key and model name — they follow the same TUI flow as `anthropic`, `openai`, and `groq` (the `"anthropic", "openai", "groq"` case in `main.go`).

## Risks / Trade-offs

**[Risk] Base URL changes** — Providers may change their API endpoints. → Mitigation: URLs are in a single map entry; easy to update. Users can also override via `custom` provider.

**[Risk] Model deprecation** — `deepseek-chat` and `deepseek-reasoner` are deprecated 2026/07/24. → Mitigation: default to `deepseek-v4-pro` which is the current model. Users specify their own model name.

**[Risk] Qwen regional endpoints** — DashScope has different endpoints for Beijing, Virginia, Singapore, Japan. → Mitigation: default to Beijing (primary). Users in other regions can use `custom` provider with their regional URL.

**[Trade-off] No provider-specific features** — DeepSeek supports thinking mode, MiniMax supports adaptive thinking. → Accepted: these are advanced features. The thin provider interface intentionally excludes them. Users needing these features can use `custom` provider with a wrapper.
