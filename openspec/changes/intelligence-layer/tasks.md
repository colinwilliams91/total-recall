## 1. Config Schema + Template Writer

- [ ] 1.1 Add `BaseURL string` field to `AIConfig` in `internal/config/config.go` with tag `yaml:"base-url,omitempty"`
- [ ] 1.2 Add named provider registry to `internal/config/config.go` — `var KnownProviders = map[string]string{...}` mapping provider names to default base URLs; include `anthropic`, `openai`, `ollama`, `groq`, `lm-studio`, `custom` (empty — requires BaseURL)
- [ ] 1.3 Replace `writeUserConfig` in `internal/config/loader.go` with a template writer — use `text/template` to produce commented YAML; template covers all fields in `UserConfig` including `base-url` (blank by default, with explanatory comment)
- [ ] 1.4 Update `DefaultUserConfig()` — default provider remains `anthropic`; `BaseURL` defaults to `""`
- [ ] 1.5 Update `tr config --show` output in `internal/config/show.go` to display `base-url` when non-empty, with source annotation

## 2. AI Provider Interface + Adapters

- [ ] 2.1 Create `internal/ai/provider.go` — define `CompletionRequest` struct (`Model`, `System`, `UserTurn` with `yaml:"user-turn"`, `MaxTokens int`, `JSON bool`) and `Provider` interface (`Complete(ctx context.Context, req CompletionRequest) (string, error)`)
- [ ] 2.2 Define `ErrNoProvider` sentinel error in `internal/ai/provider.go` — returned by `New()` when provider is unconfigured
- [ ] 2.3 Create `internal/ai/openai/client.go` — implement `Provider` via raw `net/http` POST to `{BaseURL}/chat/completions`; set `response_format: {"type":"json_object"}` when `req.JSON == true`; resolve BaseURL from registry if not overridden; 10-second default timeout
- [ ] 2.4 Create `internal/ai/anthropic/client.go` — implement `Provider` via raw `net/http` POST to `{BaseURL}/v1/messages`; append `\n\nRespond with valid JSON only.` to system prompt when `req.JSON == true`; 10-second default timeout
- [ ] 2.5 Implement `ai.New(cfg config.AIConfig) (Provider, error)` factory in `internal/ai/provider.go` — look up provider in `config.KnownProviders`; resolve base URL (registry default → `cfg.BaseURL` override for `custom`); return `ErrNoProvider` for unknown provider or `custom` with empty `BaseURL`

## 3. Provider Wiring (cmd layer)

- [ ] 3.1 Update `engine.New()` signature in `internal/engine/server.go` to accept `provider ai.Provider`, `store *cache.Store`, `recall *recall.Engine` as parameters; store as fields on `Server`
- [ ] 3.2 Wire dependencies in `cmd/total-recall/main.go` `serveCmd`: call `ai.New(cfg.AI)`, `cache.Open()`, `recall.New(provider, store)`, then `engine.New(cfg, provider, store, recallEngine)`
- [ ] 3.3 Handle `ErrNoProvider` from `ai.New()` gracefully — log advisory message and pass `nil` provider to engine; engine skips goroutine spawn when provider is nil

## 4. Concept Extraction Pipeline

- [ ] 4.1 Create `internal/pipeline/prompts.go` — implement `ExtractionRequest(diff, model string) ai.CompletionRequest`; system prompt instructs AI to return JSON array `[{"concept":"...","source":"code","weight":0.9}]`; `JSON: true`; `MaxTokens: 512`
- [ ] 4.2 Add diff truncation guard in `ExtractionRequest` — if `len(diff) > 8000`, truncate to 8000 chars and append `\n[... diff truncated for context limit ...]`; log advisory
- [ ] 4.3 Implement `ExtractConcepts(ctx context.Context, provider ai.Provider, diff, model string) ([]ConceptFingerprint, error)` in `internal/pipeline/extraction.go` — call `provider.Complete()`, `json.Unmarshal` response into `[]ConceptFingerprint`; return empty slice (not error) on AI or parse failure; log failure
- [ ] 4.4 Verify `ConceptFingerprint` struct fields (`Concept`, `Source`, `Weight`) match JSON produced by extraction prompt — update struct tags if needed

## 5. Concept Cache (SQLite)

- [ ] 5.1 Add `modernc.org/sqlite` to `go.mod` via `go get modernc.org/sqlite`
- [ ] 5.2 Create `internal/cache/store.go` — define `Store` struct; implement `Open() (*Store, error)` that creates `~/.tr/cache.db` (with `os.MkdirAll` for `~/.tr/`) and runs schema migration
- [ ] 5.3 Define SQLite schema in `Open()` — `CREATE TABLE IF NOT EXISTS concepts (id INTEGER PRIMARY KEY AUTOINCREMENT, repo TEXT NOT NULL, branch TEXT NOT NULL, concept TEXT NOT NULL, source TEXT NOT NULL, weight REAL NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`
- [ ] 5.4 Implement `(*Store).Save(ctx context.Context, fingerprints []ConceptFingerprint, repo, branch string) error` — batch INSERT in a single transaction
- [ ] 5.5 Implement `(*Store).Recent(ctx context.Context, repo string, limit int) ([]ConceptFingerprint, error)` — SELECT ordered by `created_at DESC`, filtered by repo
- [ ] 5.6 Implement `(*Store).Close() error` — close the underlying `*sql.DB`; called in daemon graceful shutdown

## 6. Recall Engine

- [ ] 6.1 Create `internal/recall/prompts.go` — implement `SynthesisRequest(concepts []ConceptFingerprint, difficulty, model string) ai.CompletionRequest`; system prompt instructs AI to return JSON `{"question":"...","choices":["...","...","..."]}` for a single multiple-choice recall question; difficulty level injected into system prompt; `JSON: true`; `MaxTokens: 256`
- [ ] 6.2 Define `Question` struct in `internal/recall/engine.go` — `Question string`, `Choices []string`
- [ ] 6.3 Implement `Engine` struct and `New(provider ai.Provider, store *cache.Store) *Engine` constructor in `internal/recall/engine.go`
- [ ] 6.4 Implement `(*Engine).Synthesize(ctx context.Context, repo, branch, difficulty, model string) (*Question, error)` — load recent concepts from store, call `provider.Complete()` with synthesis request, `json.Unmarshal` response into `Question`; return `nil, nil` (not error) if concepts are empty or AI/parse fails; log failure

## 7. Async Pipeline in handleHook

- [ ] 7.1 Add `wg sync.WaitGroup` field to `Server` in `internal/engine/server.go` — tracks in-flight hook goroutines
- [ ] 7.2 Update `handleHook` — write 202 Accepted with `{"status":"received"}` synchronously; then if provider is non-nil, call `s.wg.Add(1)` and spawn goroutine: `go s.runPipeline(env)`
- [ ] 7.3 Implement `(*Server).runPipeline(env HookEnvelope)` — calls `pipeline.ExtractConcepts`, `store.Save`, `recall.Synthesize`, then `s.dispatcher.Dispatch(question)` if question non-nil; defers `s.wg.Done()`; uses context with 10-second timeout
- [ ] 7.4 Update graceful shutdown in `Start()` — after `http.Server.Shutdown(ctx)` returns, call `s.wg.Wait()` to drain in-flight goroutines before returning; log count of drained goroutines

## 8. Presentation Layer (v1 Delivery Stub)

- [ ] 8.1 Define `Dispatcher` interface in `internal/presentation/` — `Dispatch(q recall.Question) error`
- [ ] 8.2 Implement `internal/presentation/terminal/adapter.go` — `terminalAdapter` struct implementing `Dispatcher`; `Dispatch` logs the question and choices to stdout in the styled format (🧠 Recall Check header, question text, numbered choices); note in code comment that terminal delivery to the committing shell requires a client-polling or push mechanism (Phase 4)
- [ ] 8.3 Wire `terminalAdapter` as the default `Dispatcher` in `engine.New()` when `cfg.Presentation.Terminal == true`; no dispatcher (no-op) otherwise
- [ ] 8.4 Add `DOCS/ARCHITECTURE/DELIVERY.md` — document v1 delivery limitation and Phase 4 plan (VS Code extension notifications API, `/recall/next` polling endpoint)

## 9. tr init TUI Extension (AI Provider Setup)

- [ ] 9.1 Add AI provider selection section to `runInit()` in `cmd/total-recall/main.go` — insert before existing hooks section
- [ ] 9.2 Implement Huh select for provider — options: `Anthropic (Claude)` · `OpenAI (GPT)` · `Ollama (local · free · runs on your machine)` · `Groq (cloud · fast · free tier)` · `LM Studio (local)` · `Custom (advanced)`; each option includes a description line
- [ ] 9.3 Implement cloud provider follow-up (Anthropic, OpenAI, Groq) — Huh input for API key, pre-filled with `env:PROVIDER_API_KEY`; inline description: `"Use env:VAR_NAME so your key is never stored in plaintext. Set the variable in your shell profile."`; Huh input for model name pre-filled with provider default
- [ ] 9.4 Implement local provider follow-up (Ollama, LM Studio) — Huh input for model name; description: `"Run 'ollama list' to see installed models."` (Ollama) or `"Enter the model name as shown in LM Studio."` (LM Studio); no API key prompt
- [ ] 9.5 Implement Custom follow-up — Huh input for base URL (description: `"Full base URL of your OpenAI-compatible endpoint, e.g. http://localhost:8080/v1"`), model name, and optional API key
- [ ] 9.6 Pre-populate all prompts from existing `~/.tr/config.yaml` AI block if present — user can confirm or change
- [ ] 9.7 Write completed AI config to `~/.tr/config.yaml` via template writer (Group 1.3) after all prompts complete

## 10. Docs + Cleanup

- [ ] 10.1 Update `DOCS/ARCHITECTURE/` — add Intelligence Layer section covering async pipeline, named provider registry, cache schema, and v1 delivery limitation
- [ ] 10.2 Update `README.md` — add "AI Provider Setup" section; show `tr init` flow excerpt; note async delivery and v1 behavior; add `modernc.org/sqlite` to dependency notes
- [ ] 10.3 Update `ROADMAP.md` — mark Phase 03 (Intelligence Layer) as shipped; describe Phase 04 (out-of-band delivery, VS Code extension)
- [ ] 10.4 Run `go build ./...` and `go vet ./...` — confirm clean build with no new warnings
