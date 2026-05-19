# EVENT MONITOR LAYER

This layer will parse data from the EVENT_MONITOR sub-modules and handle building the actual signal to be sent to the Incremental Analysis Pipeline -- considering its one of many origins (e.g. Git Hooks/Index changes, Filesystem changes, MCP requests)

## System Map

```
Incremental Analysis Pipeline
	→
Cache
	→
Recall Engine
	→
Presentation Layer
```

```
Core Go Engine
├── Event Monitor <<-
│	├── Observers
│   │	├── Filesystem Watcher
│   │	├── Git Index Watcher
│   │	├── Git Hook Events
│   │	└── MCP Requests
│	└── Actors
│		├── Incremental Cognition Signals
│		├── Triggering Background Analysis
│		├── Cache Invalidation/Warmup
│		└── Concept Evolution Tracking
├── Incremental Analysis Pipeline
``
```

### Generate Recall Event

Example:

```json
{
	"service": "git-hook",
	"event": "pre-commit",
	"repo": "api-server",
	"branch": "feature/retries",
	"concepts": [
		"retry logic",
		"backoff",
		"async concurrency"
	]
}
```