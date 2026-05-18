The daemon IS the Core Go Engine runtime.[^1]

## Lifecycle

_Start: The developer runs `tr serve` in their terminal._

```
tr serve
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