# What We Actually Need In Phase 1

- excellent concept extraction
- excellent question synthesis
- low-latency hook execution
- strong MCP interoperability

## Phase 1 AI Provider Strategy

### Default:

BYOK hosted providers.

### Supported:

- Anthropic
- OpenAI
- OpenRouter

### Experimental:

- Ollama

#### We do NOT need the model to:

- understand entire repositories
- autonomously code
- plan systems

--We need it to:

- synthesize concise reasoning questions
- extract concepts
- identify tradeoffs

--Keep a relatively narrow cognition scope.

---

## E2E Architecture

- **Core Go Engine**
	- This is the actual application's Persistent Cognitive Runtime
	- The Engine owns:
		- event monitoring
		- background diff analysis
		- incremental processing
		- concept extraction
		- semantic summarization
		- architecture fingerprinting
		- background concept caching
		- recall/question synthesis
		- cognitive scoring
		- spaced repetition
		- AI provider abstraction
		- cache/state
		- event-driven orchestration
		- local daemon lifecycle
		- MCP server hosting
		- presentation dispatching
- **Event Monitor**
	- Internal subsystem of the Core Go Engine
	- Observes:
		- filesystem mutations
		- Git index changes
		- staged diff mutations
		- Git hook invocations
		- MCP requests
	- Responsible for:
		- incremental cognition signals
		- triggering background analysis
		- cache invalidation/warmup
		- concept evolution tracking
- **Incremental Analysis Pipeline**
	- Internal subsystem of the Core Go Engine
	- Responsible for:
		- diff analysis
		- concept extraction
		- semantic summarization
		- architecture fingerprinting
		- lightweight behavioral analysis
	- Designed to:
		- frontload AI processing
		- reduce commit latency
		- reuse cached cognition state
		- incrementally process repository evolution
- **Background Concept Cache**
	- Internal subsystem of the Core Go Engine
	- Stores:
		- extracted concepts
		- summaries
		- embeddings/fingerprints
		- architecture context
		- recall metadata
		- retention metadata
	- Enables:
		- low-latency recall synthesis
		- incremental cognition modeling
		- reduced token/API usage
		- contextual recall continuity
- **Recall Engine**
	- Internal subsystem of the Core Go Engine
	- Responsible for:
		- question synthesis
		- cognitive scoring
		- retention tracking
		- spaced repetition
		- recall difficulty adaptation
		- contextual reinforcement generation
	- This becomes the primary long-term product moat
- **Git Hooks**
	- Not part of the core runtime. Thin invocation adapters.
	- Hooks manage:
		- capture Git lifecycle context
		- invoke Core Go Engine
		- retrieve cached cognition state
		- optionally await lightweight response
		- exit cleanly
	- Current hooks under consideration:
		- `pre-commit`
		- `commit-msg`
		- `pre-push`
- **MCP Server**
	- Not a standalone service. Hosted inside the Core Go Engine.
	- MCP manages:
		- protocol exposure
		- tool registration
		- JSON schemas
		- request/response handling
	- Allows:
		- AI-native IDE interoperability
		- agent-triggered recall flows
		- contextual reinforcement inside agent workflows
- **Presentation Adapters**
	- Presentation Layer currently supports:
		- terminal rendering
		- MCP-mediated rendering
	- Future adapters:
		- VS Code extension (Phase 2+)
		- desktop notifications
		- JetBrains integration
		- async recall queues
- **AI Provider Abstraction**
	- Provider-agnostic inference layer
	- BYOK-first architecture
	- Planned providers:
		- Anthropic
		- OpenAI
		- OpenRouter
		- Ollama (experimental)
	- Responsibilities:
		- provider routing
		- prompt orchestration
		- inference abstraction
		- fallback/provider swapping

## E2E Lifecycle

```
                +----------------------+
                | Filesystem / Git     |
                | Event Monitoring     |
                +----------------------+
                           |
                           v
              +--------------------------+
              | Incremental Analysis     |
              | Pipeline                 |
              |--------------------------|
              | Diff Analysis            |
              | Concept Extraction       |
              | Semantic Summarization   |
              | Architecture Fingerprint |
              +--------------------------+
                           |
                           v
              +--------------------------+
              | Background Concept Cache |
              +--------------------------+
                           ^
                           |
                +------------------+
                |   Git Hooks      |
                +------------------+
                           |
                           v
              +--------------------------+
              |     Core Go Engine       |
              |--------------------------|
              | Recall Engine            |
              | Retention Tracking       |
              | Spaced Repetition        |
              | AI Provider Layer        |
              | MCP Server               |
              | Presentation Dispatch    |
              +--------------------------+
                    |              |
                    v              v
          +----------------+   +----------------+
          | Terminal UX    |   | MCP Clients    |
          +----------------+   +----------------+

```

## Event Lifecycle by "Service/Layer"

- **Conceptual UX**

```
Claude Code
	->
MCP Request
	->
Core Go Engine
	->
Return Recall Prompt
```

---

- **Conceptual Background Runtime**

```
Filesystem / Git Monitor
	->
Incremental Analysis Pipeline
	->
Background Concept Cache
```

---

- **Conceptual Commit-Time Runtime**

```
Git Hook
	->
Core Go Engine
	->
Retrieve Cached Cognition State
	->
Recall Engine
	->
Presentation Layer
```

## Config Loading Lifecycle

Config is resolved once at daemon startup and held in memory. The resolution order is:

1. **`EnsureUserConfig`** — load `~/.tr/config.yaml`, or auto-create from `DefaultUserConfig()` if absent. Emits an advisory to stderr (suppressible with `--quiet`).
2. **`LoadRepoConfig`** — read `.tr.yaml` from the process working directory. Returns nil if absent (not an error).
3. **`Merge(user, repo)`** — deep-merge into a single resolved `Config`. Per-repo values win. `privacy.*` and `ai.*` in `.tr.yaml` are discarded with a warning.

Hooks do not load config themselves — they POST to the running daemon which already holds the merged config.

See [CONFIG.md](CONFIG.md) for full schema, merge rules, and `total-recall config --show` output format.

---

## Intelligence Layer (Phase 3)

### AI Provider Abstraction (`internal/ai/`)

- `Provider` interface: `Complete(ctx, CompletionRequest) (string, error)`
- `CompletionRequest` carries model, system prompt, user turn, max tokens, and a `JSON bool` that enables structured output mode per-provider
- Named provider registry (`config.KnownProviders`): `anthropic`, `openai`, `groq`, `ollama`, `lm-studio`, `custom`
- Adapters in `internal/ai/openai/` (covers all OpenAI-compatible providers) and `internal/ai/anthropic/`
- Factory (`cmd/total-recall/wire.go`) lives in the cmd layer to avoid Go import cycles
- `ErrNoProvider` sentinel — graceful no-op when AI is unconfigured

### Concept Extraction Pipeline (`internal/pipeline/`)

- `ExtractConcepts(ctx, provider, diff, model)` — sends staged diff to AI; parses JSON array of `{concept, source, weight}` fingerprints
- Diff truncated at 8 000 characters; AI failures are non-fatal (logged, empty slice returned)
- `ExtractionRequest` in `prompts.go` builds the `CompletionRequest` with system prompt embedded

### Concept Cache (`internal/cache/`)

- SQLite via `modernc.org/sqlite` (pure Go, no CGo)
- Database at `~/.tr/memory.db` (or `$TR_HOME/memory.db`); both tables tagged with `repo TEXT` column for repo-scoped recall; schema: `concepts(id, concept, source, weight, repo, seen_at)`, `questions(id, question, choices, correct_index, repo, queued_at, delivered_at, claimed_by, answer, answer_index, correct, feedback, answered_at)`
- `Save(ctx, repo, []Fingerprint)` — batch INSERT in a single transaction, tagged with repo
- `Recent(ctx, repo, n)` — SELECT for repo ordered by `seen_at DESC`

### Recall Engine (`internal/recall/`)

- `Synthesize(ctx, repo, difficulty, model)` — loads recent concepts for repo, calls AI for a multiple-choice question
- `Question{Question string, Choices []string}` — first choice is always correct
- AI/parse failures are non-fatal (nil, nil)

### Async Pipeline (`internal/engine/server.go`)

- `handleHook` responds 202 Accepted immediately; AI work is offloaded to a background goroutine via `s.wg.Add(1); go s.runPipeline(env)`
- `runPipeline` extracts concepts → saves to cache → synthesizes question → dispatches
- `sync.WaitGroup` drained in `Start()` after HTTP server shuts down — ensures no goroutines are abandoned

### Presentation Layer (`internal/presentation/`)

- `Dispatcher` interface (`internal/engine/dispatcher.go`): `Dispatch(recall.Question) error`
- `terminal.Adapter` (v1): writes question to daemon stdout (see [DELIVERY.md](DELIVERY.md) for v1 limitation and Phase 4 plan)
- Wired via `cfg.Presentation.Terminal` in `engine.New()`

---

## Important Clarification About "Daemon"
The daemon is NOT:

- a separate architecture layer
- a separate service
- a separate component

It is simply:

> the execution mode of the Core Go Engine.

Meaning:

```
tr serve
```

runs the Core Engine persistently.

Git hooks are thin HTTP clients — they POST to the running daemon:

```
Hook fires → POST localhost:7331/hooks/<event> → daemon responds
```

If the daemon is not running, hooks emit a single advisory and exit 0 (the Git operation proceeds normally). Transient engine invocation — running the Core Engine per-hook without a daemon — is **deferred indefinitely**.

---

Bind to Git to remain portable.

Do NOT make Git hooks the runtime host.

Hooks should contact the daemon.

That is much cleaner architecturally.

---
## Then Thin Clients

### MCP Adapter

Expose retention APIs.

- Spin up local server.
- Must interop with Agents CLI, VS Code MCP Starter, etc.

---
### CLI Adapter

Simple TUI prompt.

---
### VS Code Adapter

WebSocket/local IPC to daemon.

---
### Notification Adapter

Native desktop notifications.

---

## Extremely Important Architectural Decision

### The Daemon Should Own State

NOT:

- hooks
- VS Code
- MCP clients

This is critical.

The daemon becomes:

> the cognition operating layer.

That enables:

- multi-editor support
- async recall
- shared cache
- spaced repetition
- long-term memory modeling

without duplication.

That’s the architecture that scales.