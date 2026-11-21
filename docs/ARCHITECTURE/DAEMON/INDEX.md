The daemon IS the Core Go Engine runtime.[^1]

> **Status**: The daemon is live and fully wired. `torec serve` binds to `:7331`, registers the hook routes (`/hooks/*`), the recall REST surface (`GET /recall/next`, `POST /recall/select`, `GET /recall/stale`), the alias routes, and the MCP server at `/mcp/` (tools `recall_next`/`recall_select`/`recall_status`, resources `recall://queue`/`recall://recent`, prompt `recall_workflow`), and handles graceful shutdown on SIGTERM/SIGINT. The daemon writes its PID to `<data-dir>/daemon.pid` at bind time (`$TR_HOME` when set, else `~/.tr`) and removes it on graceful exit; `torec stop` signals that PID (Unix: SIGTERM; Windows: best-effort `taskkill /PID`) and waits for the drain; `torec status` surfaces the pid. Hook events run the async pipeline: concept extraction, cache save, recall synthesis, MCP resource notification, and (opt-in) terminal dispatch.

## Lifecycle

_Start: The developer runs `torec serve` in their terminal._

```
torec serve
    ->
Start Core Runtime
    ->
Open localhost:7331
    ->
Host:
    - hook IPC
    - MCP server
    - cognition APIs
```

_Then: Hooks trigger requests to the daemon, and the daemon processes them and sends signals to the Incremental Analysis Pipeline._

```
Git Hook
    ->
HTTP (future: gRPC/local RPC call)
    ->
localhost:7331
```

## APIs
APIs are logically distinct and separate despite running on the same server.

#### Internal Hook API

URL prefix: `/hooks`

Used by:

Git hooks
local CLI
daemon coordination

Responsibilities:

hook event submission
cache sync
recall requests
daemon control

#### MCP API

URL prefix: `/mcp`

Used by:

Claude Code (AI CLIs)
AI IDEs
MCP clients

Responsibilities:

protocol-compliant tool exposure
agent interoperability

---

### Transport

```
localhost TCP
```

### Runtime

```
single daemon
```

### Protocol

```
HTTP
```

### Shared Runtime

```
hooks + MCP share same runtime
```

### Shared Cognition State
```
one cache
one recall engine
one concept graph
```

[^1]: It hosts the MCP server and the hook IPC server, and is responsible for loading config, managing the concept cache, and orchestrating the recall process. The daemon runs locally on the developer's machine and serves as the central coordinator for all Total Recall interactions.