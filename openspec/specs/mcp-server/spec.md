## Requirements

### Requirement: MCP server is mounted at /mcp/ on port 7331
`mcp.NewStreamableHTTPHandler(mcpServer, nil)` SHALL be registered at `/mcp/` in `RegisterRoutes()`. The MCP server SHALL share the existing HTTP listener — no new port, no new process.

#### Scenario: MCP handler responds to tool list request
- **WHEN** an MCP client sends `tools/list` to `http://localhost:7331/mcp/`
- **THEN** the response includes `recall_next`, `recall_answer`, and `recall_status` in the tools array

---

### Requirement: recall_next tool atomically dequeues one question, scoped to optional repo
The `recall_next` tool SHALL accept an optional `repo` input field (the absolute repository path). It SHALL call `store.NextQuestion(ctx, repo, "mcp")` where `repo` is the supplied value or `""` when omitted. It SHALL return the question, choices, and `correct_index` as JSON content when a question is available, or `{"question":null}` when the queue is empty for that repo. `claimed_by` SHALL be set to `"mcp"`. `correct_index` is included so AI agents can evaluate answers locally.

#### Scenario: Question available for the supplied repo
- **WHEN** `recall_next` is called with `{"repo": "/path/X"}` and a question with `repo=/path/X` exists with `delivered_at IS NULL`
- **THEN** the tool returns `{"id":N,"question":"...","choices":[...],"correct_index":N}` and the row is marked delivered

#### Scenario: Repo omitted — global dequeue (backward compatible)
- **WHEN** `recall_next` is called with `{}` (no `repo` field)
- **THEN** the tool dequeues from the global pool (`repo=""`), preserving behavior for existing MCP clients

#### Scenario: Queue empty for the repo
- **WHEN** `recall_next` is called with `{"repo": "/path/Y"}` and no unclaimed questions exist for `/path/Y`
- **THEN** the tool returns `{"question":null}`

---

### Requirement: recall_answer tool evaluates and returns correctness (no AI call)
The `recall_answer` tool SHALL accept `{"id": N, "answer_index": N, "skip": true, "repo": "/path"}` where `repo` is optional. On skip, it SHALL call `store.SkipQuestion` and return `{"ok":true}`. On answer, it SHALL call `store.GetQuestion`, evaluate `correct = (answer_index == correct_index)`, call `store.AnswerQuestion` with empty feedback (no AI call on MCP path), and return `{"ok":true,"correct":<bool>,"correct_index":N,"correct_text":"..."}`. `AnswerQuestion` and `SkipQuestion` are ID-keyed, so `repo` is accepted for symmetry but does not affect the operation. On unknown ID, it SHALL return a tool error. On out-of-range `answer_index`, it SHALL return a tool error.

#### Scenario: Correct answer submitted
- **WHEN** `recall_answer` is called with `{"id": 1, "answer_index": 0}` and answer 0 is correct
- **THEN** the server evaluates `correct = true`, calls `AnswerQuestion(ctx, 1, 0, "Prevent storms", true, "")` — no AI feedback call
- **AND** returns `{"ok":true,"correct":true,"correct_index":0,"correct_text":"Prevent storms..."}`

#### Scenario: Skip submitted
- **WHEN** `recall_answer` is called with `{"id": 1, "skip": true}`
- **THEN** `SkipQuestion` is called, returns `{"ok":true}`

#### Scenario: Unknown question ID
- **WHEN** `recall_answer` is called with an ID that does not exist
- **THEN** the tool returns an error result and no DB write occurs

---

### Requirement: recall_status tool returns daemon health, scoped to optional repo
The `recall_status` tool SHALL accept an optional `repo` input field. It SHALL return `{"daemon":"ok","ai_configured":bool,"queue_depth":N}` where `ai_configured` is true when `cfg.AI.Provider` is non-empty and `queue_depth` is the count of unclaimed questions for the supplied `repo` (or the global pool when `repo` is omitted).

#### Scenario: Status for a specific repo
- **WHEN** `recall_status` is called with `{"repo": "/path/X"}`
- **THEN** `queue_depth` reflects only unclaimed questions with `repo=/path/X`

#### Scenario: Status without repo
- **WHEN** `recall_status` is called with `{}`
- **THEN** `queue_depth` reflects the global pool

---

### Requirement: recall://queue resource is subscribable and repo-scoped
The `recall://queue` resource SHALL be registered with `server.AddResource`. Its handler SHALL return current queue depth and the next pending question (peeked without dequeuing) for the requested repo as JSON. The repo SHALL be derived from the resource URI query string (`recall://queue?repo=/path/X`) or default to the global pool when no query string is supplied. Connected clients that subscribe SHALL receive `notifications/resources/updated` via SSE when `runPipeline` calls `NotifyResourceUpdated("recall://queue")`.

#### Scenario: Question synthesized for a repo
- **WHEN** `runPipeline` saves a new question with `repo=/path/X` to `memory.db`
- **THEN** `NotifyResourceUpdated("recall://queue")` is called for each connected MCP session

---

### Requirement: recall://recent resource returns repo-scoped answer history
The `recall://recent` resource SHALL return the last 10 answered questions for the requested repo as a JSON array. The repo SHALL be derived from the resource URI query string or default to the global pool. Each item SHALL include `id`, `question`, `choices`, `correct_index`, `answer_index`, `correct`, and `feedback`. NULL fields SHALL be serialized as `null`.

---

### Requirement: recall_workflow prompt instructs agent to self-explain
The `recall_workflow` prompt SHALL be registered and return a prompt message instructing the AI agent to: call `recall_next` after git commits (passing the current repo when known), present questions to the user, record answers with `recall_answer`, tell the user whether they were correct, provide a brief direct explanation using its own knowledge, NOT call a separate AI tool for the explanation, and continue normally if the queue is empty.

#### Scenario: Agent retrieves workflow prompt
- **WHEN** an MCP client retrieves the `recall_workflow` prompt
- **THEN** the prompt text instructs the agent to self-explain after recording an answer and to pass the current repo to `recall_next` when known
