## Why

Phase 2 proved the plumbing: hooks fire, the daemon receives, and the wire format is forward-compatible. But `handleHook` still just logs and returns `{"status":"received"}` — no AI, no questions, no recall. Phase 3 wires in the intelligence layer that makes Total Recall actually useful: a staged diff arrives at the daemon, concepts are extracted via an AI provider, a recall question is synthesized from those concepts, and the result is dispatched asynchronously. After this phase, a developer can run `git commit`, trigger the hook, and — without blocking their commit — receive a recall prompt based on what they just coded.

## What Changes

- `AIConfig` gains a `BaseURL` field (`yaml:"base-url,omitempty"`) enabling users to point any OpenAI-compatible provider at a custom endpoint (Ollama, Groq, LM Studio, etc.)
- Named provider registry maps user-facing names (`anthropic`, `openai`, `ollama`, `groq`, `lm-studio`, `custom`) to their base URLs and internal adapter packages — users never need to know which package handles their provider
- `~/.tr/config.yaml` is now written from a Go template rather than `yaml.Marshal`, producing a self-documenting file with inline comments that guide manual editing
- `internal/ai/Provider` interface defined — thin primitive: `Complete(ctx, CompletionRequest) (string, error)`; domain-layer callers (pipeline, recall) build requests and parse responses
- `internal/ai/openai/` implements `Provider` via raw `net/http` — covers OpenAI, Ollama, Groq, LM Studio, and all OpenAI-compatible endpoints
- `internal/ai/anthropic/` implements `Provider` via raw `net/http` — covers Claude models
- `internal/ai/` factory `New(cfg AIConfig) (Provider, error)` — wired at the `cmd` layer and injected into the engine (dependency injection, not internal construction)
- `internal/pipeline/` — concept extraction: receives staged diff, calls `provider.Complete()`, returns `[]ConceptFingerprint`; prompt logic lives in `prompts.go`
- `internal/cache/` — SQLite concept cache at `~/.tr/cache.db` using `modernc.org/sqlite`; stores concept fingerprints keyed by repo + branch; opened at daemon start, closed on graceful shutdown
- `internal/recall/` — recall engine: reads recent concepts from cache, calls `provider.Complete()` to synthesize a question + choices; prompt logic lives in `prompts.go`
- `engine.handleHook()` writes 202 Accepted immediately (non-blocking), then spawns a goroutine to run the full pipeline: extract → cache → synthesize → dispatch
- `internal/presentation/terminal/` — v1 delivery stub: questions surface in daemon stdout; full out-of-band terminal delivery (client polling or push) is Phase 4
- `tr init` gains an AI provider selection section — Huh prompts guide users through provider choice, API key setup, and model selection before the existing hooks section; config is written via the new template writer

## Capabilities

### New Capabilities

- `ai-provider`: Provider interface, `CompletionRequest` struct, named registry, `openai` and `anthropic` adapters, `ai.New()` factory
- `concept-extraction`: Staged diff → `[]ConceptFingerprint` via AI; extraction prompt in `internal/pipeline/prompts.go`; graceful degradation on failure
- `recall-engine`: `[]ConceptFingerprint` from cache → `Question{Question, Choices}` via AI; synthesis prompt in `internal/recall/prompts.go`; graceful degradation on failure
- `concept-cache`: SQLite store at `~/.tr/cache.db`; schema, lifecycle, save and recent-query operations
- `init-ai-setup`: `tr init` AI provider selection TUI step; provider picker with named presets; provider-specific follow-up prompts; template-written config

### Modified Capabilities

- `hook-dispatch`: `HookResponse.Recall` is now populated on the async path (`status: "recall_ready"`); hook scripts receive `202 Accepted` synchronously as before — delivery is out-of-band
- `daemon-server`: `handleHook` launches async goroutine; engine holds `Provider`, `Cache`, and `RecallEngine` as injected dependencies; graceful shutdown drains in-flight goroutines

## Impact

- `internal/config/config.go` — `AIConfig` gains `BaseURL`; named provider registry added
- `internal/config/loader.go` — `writeUserConfig` replaced with template writer
- `internal/ai/` — `provider.go` (interface + factory), `openai/client.go`, `anthropic/client.go`
- `internal/pipeline/` — `extraction.go` and `prompts.go` implemented
- `internal/cache/` — `store.go` implemented; `go.mod` gains `modernc.org/sqlite`
- `internal/recall/` — `engine.go` and `prompts.go` implemented
- `internal/engine/server.go` — `New()` signature gains `Provider`, `Cache`, `RecallEngine`; `handleHook` goes async
- `internal/presentation/terminal/` — `adapter.go` v1 stub
- `cmd/total-recall/main.go` — `serveCmd` wires `ai.New()` and injects dependencies; `runInit()` gains AI provider section
- `go.mod` — adds `modernc.org/sqlite`
- `ROADMAP.md` — Phase 03 marked shipped; Phase 04 description updated

## Key Design Decisions

- **Async delivery (non-negotiable)**: Hooks must never block a Git operation. The daemon writes 202 Accepted before any AI call is made. All intelligence work runs in a background goroutine. Recall questions cannot be delivered to the committing terminal in v1; they surface in daemon stdout. Full async delivery (VS Code extension notifications API or equivalent) is Phase 4.
- **Option 3 interface pattern**: `Provider` is a primitive (`Complete()`), not domain-aware. Concept extraction, question synthesis, and prompt construction live in the calling layers (`pipeline/`, `recall/`), not in the adapter.
- **Named provider registry**: User-facing provider names decouple from internal implementation. `provider: ollama` works without any knowledge that Ollama speaks the OpenAI-compatible API.
- **Dependency injection at cmd layer**: `engine.New()` receives `Provider`, `Cache`, and `RecallEngine` as arguments. No internal construction. Testable with mocks.
- **Template-written config**: `writeUserConfig` uses a Go text/template to produce commented YAML. Power users can edit the file manually; the template ensures the file is self-documenting on first write.
