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

Whereas:

```
tr hook pre-commit
```

runs it transiently.

Same engine.

Different lifecycle mode.

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