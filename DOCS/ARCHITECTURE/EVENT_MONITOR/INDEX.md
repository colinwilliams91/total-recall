# EVENT MONITOR LAYER

This layer will parse data from the EVENT_MONITOR sub-modules and handle building the actual signal to be sent to the PRESENTATION_LAYER -- considering its one of many origins (e.g. Git Hooks/Index changes, Filesystem changes, MCP requests)

## System Map

```
Core Go Engine
├── Event Monitor <<-
│   ├── Filesystem Watcher
│   ├── Git Index Watcher
│   ├── Git Hook Events
│   └── MCP Requests
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