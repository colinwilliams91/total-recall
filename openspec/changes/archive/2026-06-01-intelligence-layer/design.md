## Context

The daemon from Phase 2 receives hook payloads, logs them, and returns `{"status":"received"}`. The `HookResponse.Recall` field is defined but always nil. The `internal/ai`, `internal/cache`, `internal/pipeline`, `internal/recall`, and `internal/presentation/` packages are all stubs. This design wires them together into a working intelligence pipeline.

The single most important architectural decision in this phase is **async delivery**: the hook's 2-second hard timeout is non-negotiable. No AI provider can be reliably called within 2 seconds on every commit. Therefore the daemon writes 202 Accepted before any AI work begins, and all intelligence processing runs in a background goroutine. The consequence is that recall questions cannot reach the committing terminal in v1 — they surface in daemon stdout, and full out-of-band delivery (VS Code extension, client polling) is Phase 4.

## Goals / Non-Goals

**Goals:**
- `Provider` interface and `CompletionRequest` struct — thin primitive, no domain knowledge
- Named provider registry — user writes `ollama`, system resolves base URL and adapter
- `openai/` and `anthropic/` adapters via raw `net/http` (no SDK dependencies)
- `ai.New()` factory injected at cmd layer (dependency injection, not internal construction)
- Concept extraction from staged diffs via AI, with graceful degradation
- SQLite concept cache at `~/.tr/cache.db` — save and recent-query
- Recall question synthesis from cached concepts via AI, with graceful degradation
- Async goroutine pipeline in `handleHook` — 202 written before AI work begins
- Graceful shutdown drains in-flight goroutines
- `tr init` AI provider selection TUI step — named presets, provider-specific prompts
- Template-written `~/.tr/config.yaml` — self-documenting with inline comments
- v1 delivery: questions logged to daemon stdout

**Non-Goals:**
- Out-of-band terminal delivery to the committing shell (Phase 4)
- VS Code extension integration (Phase 4)
- MCP protocol implementation (future phase)
- Difficulty progression / adaptive recall (future phase)
- Spaced repetition / concept decay (future phase)
- Daemon autostart (deferred, noted in ROADMAP)

## Decisions

### 1. Async pipeline — 202 before AI

**Decision**: `handleHook` writes `202 Accepted` with `{"status":"received"}` synchronously, then spawns a goroutine. All AI work (extraction → cache → synthesis → dispatch) runs in that goroutine.

**Rationale**: The hook's 2-second hard timeout is a non-negotiable DevX requirement. Slow commit hooks are unacceptable. AI calls are not reliably sub-2-second. Even if they were today, a slow provider, cold start, or rate-limit retry would break the guarantee. Async is the only architecture that holds under all conditions.

**v1 delivery consequence**: The background goroutine cannot write to the terminal that ran `git commit` — that shell has already continued. v1 surfaces questions in daemon stdout. Phase 4 delivers via VS Code extension notifications API or a client-polling `/recall/next` endpoint.

**Goroutine lifecycle**: The daemon holds a `sync.WaitGroup`. Each hook goroutine calls `wg.Add(1)` / `wg.Done()`. Graceful shutdown calls `wg.Wait()` after `http.Server.Shutdown()` completes (subject to the existing 5-second drain timeout).

---

### 2. Provider interface — primitive, not domain-aware

**Decision**:
```go
// internal/ai/provider.go

type CompletionRequest struct {
    Model     string // "" → adapter uses config default
    System    string // system prompt
    UserTurn  string `yaml:"user-turn"` // user message
    MaxTokens int    // 0 → provider default
    JSON      bool   // hint: request JSON-mode output if supported
}

type Provider interface {
    Complete(ctx context.Context, req CompletionRequest) (string, error)
}
```

**Rationale**: If the interface were task-specific (`ExtractConcepts`, `SynthesizeQuestion`), every new task type would require changing the interface and every adapter. With a primitive interface, new tasks only touch the calling layer (pipeline/, recall/). The interface is stable. Adapters are simple and independently testable with `httptest.Server`.

**JSON bool**: OpenAI supports `response_format: {"type":"json_object"}` — the adapter sets this when `JSON: true`. Anthropic does not have a JSON mode API param; the adapter appends `\n\nRespond with valid JSON only.` to the system prompt when `JSON: true`. Both paths produce more reliable structured output than prompt-only requests.

**Return type**: `(string, error)` — caller parses. No `CompletionResponse` wrapper in Phase 3 (YAGNI). Token usage, finish reason, and truncation detection can be added if needed in a later phase.

---

### 3. Named provider registry

**Decision**: The user-facing `provider` field in `AIConfig` is a named preset, not an internal package name. A registry maps presets to base URLs and adapter selection:

```
User-facing name    Adapter package    Default base URL
────────────────    ───────────────    ─────────────────────────────────
anthropic           anthropic          https://api.anthropic.com/v1
openai              openai             https://api.openai.com/v1
ollama              openai             http://localhost:11434/v1
groq                openai             https://api.groq.com/openai/v1
lm-studio           openai             http://localhost:1234/v1
custom              openai             cfg.BaseURL (required; error if empty)
```

**BaseURL in config**: `AIConfig.BaseURL` is always present in the generated config file (via template), even when empty, with a comment explaining its purpose. Power users editing manually can set it without running `tr init` again. For all named presets except `custom`, `BaseURL` is ignored — the registry value is used.

**Rationale**: `provider: ollama` is intuitive. `provider: openai` for a local model is not. Hiding the implementation detail (openai-compat wire format) behind a friendly name removes an entire category of user confusion.

---

### 4. Dependency injection at cmd layer

**Decision**: `engine.New()` signature becomes:
```go
func New(cfg *config.Config, provider ai.Provider, cache *cache.Store, recall *recall.Engine) *Server
```

`cmd/total-recall/main.go` `serveCmd` constructs and injects all dependencies:
```go
provider, err := ai.New(cfg.AI)
store, err := cache.Open()
recallEngine := recall.New(provider, store)
server := engine.New(cfg, provider, store, recallEngine)
```

**Rationale**: Dependency injection makes `engine_test.go` straightforward — pass a mock `Provider`, mock `Store`. No internal construction means no test-breaking side effects (network calls, file I/O) inside `engine.New()`.

**Missing provider handling**: If `cfg.AI.Provider` is empty or unconfigured, `ai.New()` returns an `ErrNoProvider` sentinel. `serveCmd` logs an advisory (`[daemon] AI provider not configured — recall questions will not be generated. Run 'total-recall init' to configure.`) and passes a `nil` provider. The engine checks for nil provider before spawning AI goroutines — hooks still fire and are acknowledged, but no recall questions are produced.

---

### 5. Template-written config file

**Decision**: `writeUserConfig` is replaced by a template writer. The template produces:

```yaml
# Total Recall user configuration
# Run 'total-recall init' to reconfigure interactively.
# This file is never committed — it lives at ~/.tr/config.yaml.

privacy:
  # Set to true to allow MCP conversation content to feed the concept cache.
  # This is an opt-in feature. See: https://github.com/colinwilliams91/total-recall
  conversation_analysis: false

ai:
  # Provider: anthropic | openai | ollama | groq | lm-studio | custom
  # Run 'total-recall init' to change provider interactively.
  provider: anthropic

  # Model to use for concept extraction and question synthesis.
  model: claude-sonnet-4-5

  # API key. Use env:VAR_NAME to avoid storing keys in plaintext.
  # Example: env:ANTHROPIC_API_KEY
  api-key: env:ANTHROPIC_API_KEY

  # Base URL override for OpenAI-compatible endpoints (ollama, groq, lm-studio, custom).
  # Leave blank for standard cloud providers (anthropic, openai).
  # Example: http://localhost:11434/v1
  base-url:

recall:
  # Difficulty level: easy | medium | hard | adaptive
  difficulty: adaptive

  # Maximum number of recall questions per commit (default: 1).
  max_questions: 1
```

**Rationale**: `yaml.Marshal` produces a bare struct dump with no comments, no ordering guarantees, and no guidance for manual editing. Power users who open the file later need to understand what each field does without reading documentation. The template gives full control over order, comments, and formatting. The struct (`UserConfig`) remains the source of truth for reading; the template is only used for writing.

---

### 6. tr init AI provider selection

**Decision**: `runInit()` gains a new section that runs before the existing hooks section. Flow:

1. Provider picker (Huh select): `Anthropic (Claude)`, `OpenAI (GPT)`, `Ollama (local)`, `Groq`, `LM Studio`, `Custom`
2. Provider-specific follow-up:
   - **Cloud** (Anthropic, OpenAI, Groq): API key input, pre-filled with `env:PROVIDER_API_KEY`. Inline tip explains `env:VAR_NAME` pattern.
   - **Local** (Ollama, LM Studio): Model name input with `ollama list` hint. No API key prompt.
   - **Custom**: Base URL input, model name, API key (optional).
3. Selected values written to `~/.tr/config.yaml` via template writer.
4. If config already contains an `ai` block, prompts pre-populate with existing values.

**Rationale**: Users with low AI institutional knowledge should never encounter `base-url` or `provider: openai` for a local model. The TUI surfaces friendly names and fills in the technical details. The config file is the output, not the input.

---

### 7. Concept extraction and synthesis prompts

**Extraction prompt** (`internal/pipeline/prompts.go`):
- System: instructs the model to analyze a staged Git diff and return a JSON array of technical concepts demonstrated, each with a name, source (`"code"`), and confidence weight
- User turn: the raw `git diff --cached` output
- `JSON: true`, `MaxTokens: 512`
- Diff truncation guard: if diff exceeds ~8000 characters, truncate with a `[... truncated ...]` marker and log advisory

**Synthesis prompt** (`internal/recall/prompts.go`):
- System: instructs the model to generate a single recall question from a list of concepts, with 3 multiple-choice answers, returned as JSON `{question, choices}`; difficulty setting injected into system prompt
- User turn: JSON-serialized concept list
- `JSON: true`, `MaxTokens: 256`

---

## Risks / Trade-offs

**[Risk] v1 delivery is daemon stdout only** — users running `total-recall serve` in the background won't see questions at commit time unless they're watching the daemon terminal. Mitigation: document clearly in README; Phase 4 resolves with VS Code extension. This deliberately shrinks the v1 user pool to developers who are already running the daemon in a visible terminal.

**[Risk] AI call latency variation** — a slow provider may cause goroutines to pile up if the user commits rapidly. Mitigation: each goroutine has a context with a 10-second AI timeout (separate from the hook's 2-second transport timeout). Graceful shutdown drains up to the existing 5-second window; any remaining goroutines are logged and abandoned.

**[Risk] Large diffs** — a diff containing thousands of lines may hit model context limits. Mitigation: diff truncation guard in extraction prompt builder (8000-char ceiling in Phase 3; configurable in future phase).

**[Risk] `modernc.org/sqlite` adds a CGO-free SQLite dep** — this is the only new external dependency in Phase 3. It is CGO-free (pure Go), so cross-compilation remains clean. Verified against go.mod conventions from Phase 2.

**[Risk] Nil provider in production** — if user skips AI setup, every hook POST spawns a goroutine that immediately exits (nil check). This is correct but should be verified under load. Mitigation: early nil check before goroutine spawn; no goroutine created if provider is nil.
