## Requirements

### Requirement: MCP server is mounted at /mcp/ on port 7331
`mcp.NewStreamableHTTPHandler(mcpServer, nil)` SHALL be registered at `/mcp/` in `RegisterRoutes()`. The MCP server SHALL share the existing HTTP listener — no new port, no new process.

#### Scenario: MCP handler responds to tool list request
- **WHEN** an MCP client sends `tools/list` to `http://localhost:7331/mcp/`
- **THEN** the response includes `recall_next`, `recall_answer`, and `recall_status` in the tools array

---

### Requirement: recall_next tool atomically dequeues one question
The `recall_next` tool SHALL call `store.NextQuestion(ctx, "mcp")`. It SHALL return the question and choices as JSON content when a question is available, or return content representing null when the queue is empty. `claimed_by` SHALL be set to `"mcp"` in the database.

#### Scenario: Question available
- **WHEN** `recall_next` is called and a question exists with `delivered_at IS NULL`
- **THEN** the tool returns `{"id":N,"question":"...","choices":["...","...","..."]}` and the row is marked delivered

#### Scenario: Queue empty
- **WHEN** `recall_next` is called and no unclaimed questions exist
- **THEN** the tool returns content indicating no question (`null` / empty)

---

### Requirement: recall_answer tool records a response
The `recall_answer` tool SHALL accept `{"id": N, "answer": "..."}` and call `store.AnswerQuestion(ctx, id, answer)`. It SHALL return `{"ok": true}` on success. The string `"skip"` is a valid answer.

---

### Requirement: recall_status tool returns daemon health
The `recall_status` tool SHALL return `{"daemon":"ok","ai_configured":bool,"queue_depth":N}` where `ai_configured` is true when `cfg.AI.Provider` is non-empty and `queue_depth` is the count of unclaimed questions.

---

### Requirement: recall://queue resource is subscribable
The `recall://queue` resource SHALL be registered with `server.AddResource`. Its handler SHALL return current queue depth and the next pending question (peeked without dequeuing) as JSON. Connected clients that subscribe SHALL receive `notifications/resources/updated` via SSE when `runPipeline` calls `sess.NotifyResourceUpdated("recall://queue")`.

#### Scenario: Question synthesized
- **WHEN** `runPipeline` saves a new question to `memory.db`
- **THEN** `NotifyResourceUpdated("recall://queue")` is called for each connected MCP session

---

### Requirement: recall_workflow prompt provides agent instructions
The `recall_workflow` prompt SHALL be registered and return a prompt message instructing the AI agent to call `recall_next` after git commits and present the question to the user.
