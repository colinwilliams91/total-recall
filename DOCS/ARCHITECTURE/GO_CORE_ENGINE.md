# Persistent Cognitive Runtime
Written in Go

# Internal Architecture

```
Core Go Engine
├── Event Monitor
│   ├── Filesystem Watcher
│   ├── Git Index Watcher
│   ├── Git Hook Events
│   └── MCP Requests
│
├── Incremental Analysis Pipeline
│   ├── Diff Analysis
│   ├── Concept Extraction
│   ├── Architecture Fingerprinting
│   └── Semantic Summarization
│
├── Background Concept Cache
│
├── Recall Engine
│   ├── Question Synthesis
│   ├── Cognitive Scoring
│   ├── Retention Tracking
│   └── Spaced Repetition
│
├── AI Provider Abstraction
│
├── MCP Server
│
└── Presentation Dispatch
    ├── Terminal Renderer
    └── MCP Renderer
```

## Two Operating Modes
### Mode 1 — Background Runtime

The daemonized cognition layer.

Responsibilities:

- monitor filesystem
- monitor Git index
- incremental concept extraction
- maintain concept graph
- warm caches
- preprocess summaries
- observe coding evolution

This is where:

```
Filesystem/Git Monitor
	->
Background Concept Cache
```

lives.

### Mode 2 — Interactive Recall Runtime

Triggered by:

- Git hooks
- MCP requests
- future IDE adapters

Responsibilities:

- retrieve cached cognition state
- synthesize final recall question
- render/publish prompt
- record retention event

This is where:

```
Git Hook
	->
Retrieve Cached Context
	->
Question Synthesis
```

lives.

---

# Go Core Engine Responsibilities


---

# 1. Event Monitoring

Observes:

- filesystem mutations
- Git index changes
- staged diff changes
- hook invocations
- MCP requests

This becomes:

> the cognition signal layer.

---

# 2. Incremental Analysis Pipeline

Instead of:

```
Analyze Entire Diff Every Commit
```

you evolve toward:

```
Track Incremental Concept Evolution
```

This is both:

- faster
- cognitively richer

---

# 3. Background Concept Cache

Stores:

- summaries
- embeddings
- detected concepts
- architecture fingerprints
- framework detection
- behavioral patterns

Potentially:

```json
{  
	"repo": "api-service",
	"branch": "feature/retries",
	"concepts": [
		"retry logic",
		"exponential backoff",
		"optimistic concurrency"
	],  
	"confidence": 0.92,
	"updated_at": "..."
}
```

---

# 4. Recall Engine

Still internal.

Still your moat.

Now benefits from:

- richer context
- development trajectory
- reduced latency

Responsibilities:

- question synthesis
- cognitive scoring
- retention tracking
- spaced repetition

---

# 5. AI Provider Abstraction

```
Core Go Engine  
	->  
AI Provider Abstraction  
	->  
User-Configured Provider (BYOK)
```

### Supported:

- Anthropic
- OpenAI
- OpenRouter
- Ollama (experimental)

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

# 6. MCP Server

Still:

- internal
- hosted inside Core Engine

Now:

```
development activity
	->
continuous cognition modelingcommit
	->
lightweight recall synthesis
```

# This Also Makes MCP Much Better

Because now MCP clients can ask:

```
What concepts are currently active?
```

WITHOUT requiring:

- a commit
- a push
- a hook trigger

---

# 7. Presentation Dispatch

Determines:

- terminal
- MCP
- future adapters

This becomes cleaner now because rendering is decoupled from analysis timing.

---

# One Important Warning

DO NOT let this become:

- a full IDE indexer
- a full LSP
- a full semantic compiler platform

You want:

- lightweight cognition signals
- not whole-program analysis

Stay disciplined there.

---

## Think of the Go Core Engine as:

“The Cognitive Runtime”

And:

- Git hooks
- MCP
- future IDE adapters

become:

> event emitters and presentation surfaces.
