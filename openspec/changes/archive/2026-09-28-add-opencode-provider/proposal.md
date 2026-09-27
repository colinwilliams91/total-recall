# Proposal: add-opencode-provider

## Why

OpenCode Go is a $10/month subscription that provides reliable access to a curated set of open coding models (GLM, Kimi, DeepSeek, MiMo, LongCat, Hy3/Hy4, and limited-time free models) through OpenAI-compatible endpoints. Traffic is explicitly "designed for OpenCode and other coding agents" — torec is in the target audience. today, users on OpenCode Go must configure the `custom` preset manually (base URL `https://opencode.ai/zen/go/v1`, model id, and `OPENCODE_API_KEY`), which is friction in exactly the place `torec init` is supposed to remove it. This follows the same shape of work as the OpenRouter promotion (#57) and the issue request in #58.

The engine appends `/chat/completions` to the registry base URL (`internal/ai/provider.go`, used via the `internal/ai/openai` adapter), which lines up with the OpenCode Go docs: chat-completions-routed models are served at `https://opencode.ai/zen/go/v1/chat/completions`, so `https://opencode.ai/zen/go/v1` is all we need as the base URL.

Not all Go models use chat/completions: Grok/GPT-Luna models route through the OpenAI **Responses** API (`/responses`), and Qwen/MiniMax route through the Anthropic **Messages** API (`/messages`). The default model therefore stays on the chat-completions path our single adapter already speaks. Those other routes are out of scope (a Responses adapter would be a separate change).

## What Changes

- `ProviderRegistry` (`internal/ai/provider.go`) gains `"opencode": "https://opencode.ai/zen/go/v1"` served by the existing `internal/ai/openai` adapter.
- `providerModelDefaults` (`cmd/torec/main.go`) gains `"opencode": "glm-5.3-flash"` — $0.15/$0.50 per 1M tokens, $60/mo usage limit, 0-day retention (ZDR), ~31k requests/month; one of the roomiest limits on the list and the best value pick for cost-conscious users.
- `providerAPIKeyPlaceholders` (`cmd/torec/main.go`) gains `"opencode": "env:OPENCODE_API_KEY"`.
- `runInitAI` provider picker gains an `OpenCode Go` option and includes `"opencode"` in the cloud-provider switch case (API key + model prompts).
- The unknown-provider error message in `cmd/torec/wire.go` lists `opencode`.
- `internal/config` `KnownProviders` (the comment-documented parallel registry) and the user-config template's provider comment line list `opencode` so `torec config show`/template output has parity.
- `cmd/torec/provider_test.go` `TestNewProviderRoutesOpenAIFallback` covers `opencode` routing to `*openai.Client`.
- `openspec/specs/ai-provider/spec.md`: registry preset list + an `opencode` scenario.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ai-provider`: the named-provider registry requirement's preset list and resolution scenarios add `opencode` → `https://opencode.ai/zen/go/v1` via the openai package adapter.

## Non-goals

- No Anthropic-adapter route switch for Qwen/MiniMax-via-Go (native Messages API models on the same base URL) — a follow-up if wanted.
- No OpenAI Responses API adapter for Grok/GPT-Luna models — out of scope.
- No `x-opencode-session` session-ID header or custom `User-Agent: torec/1.0` — nice-to-have client expectations from the docs; natural follow-up (a stable id derived from repo+branch would fit our cache scoping).
- No change to hook, cache, or recall behavior — this is a provider preset only.

## Impact

- **Code**: `internal/ai/provider.go` (registry entry), `internal/config/config.go` (`KnownProviders` + doc comment), `internal/config/loader.go` (template comment line), `cmd/torec/main.go` (model default, key placeholder, TUI option + switch case), `cmd/torec/wire.go` (unknown-provider error text).
- **Tests**: `cmd/torec/provider_test.go` extend the OpenAI-fallback routing test with `opencode`.
- **Specs**: `openspec/specs/ai-provider/spec.md` updated with the new preset + scenario.
- **Behavioral risk**: none beyond a new preset name; unknown-provider error text change is cosmetic.
