## Requirements

### Requirement: MCP server is mounted at /mcp/ on port 7331
`mcp.NewStreamableHTTPHandler(mcpServer, nil)` SHALL be registered at `/mcp/` in `RegisterRoutes()`. The MCP server SHALL share the existing HTTP listener — no new port, no new process.

#### Scenario: MCP handler responds to tool list request
- **WHEN** an MCP client sends `tools/list` to `http://localhost:7331/mcp/`
- **THEN** the response includes `recall_next`, `recall_answer`, and `recall_status` in the tools array

---

### Requirement: recall_next tool atomically dequeues one question and returns correct_index
The `recall_next` tool SHALL call `store.NextQuestion(ctx, "mcp")`. It SHALL return the question, choices, and `correct_index` as JSON content when a question is available, or return content representing null when the queue is empty. `claimed_by` SHALL be set to `"mcp"` in the database. `correct_index` is included so AI agents can evaluate answers locally — an AI agent seeing the answer doesn't invalidate the quiz.

#### Scenario: Question available
- **WHEN** `recall_next` is called and a question exists with `delivered_at IS NULL`
- **THEN** the tool returns `{"id":N,"question":"...","choices":["...","...","..."],"correct_index":N}` and the row is marked delivered

#### Scenario: Queue empty
- **WHEN** `recall_next` is called and no unclaimed questions exist
- **THEN** the tool returns `{"question":null}` with no `correct_index`

---

### Requirement: recall_answer tool evaluates and returns correctness (no AI call)
The `recall_answer` tool SHALL accept `{"id": N, "answer_index": N}` or `{"id": N, "skip": true}`. On skip, it SHALL call `store.SkipQuestion` and return `{"ok":true}`. On answer, it SHALL call `store.GetQuestion`, evaluate `correct = (answer_index == correct_index)`, call `store.AnswerQuestion` with empty feedback (no AI call on MCP path), and return `{"ok":true,"correct":<bool>,"correct_index":N,"correct_text":"..."}`. On unknown ID, it SHALL return a tool error. On out-of-range `answer_index`, it SHALL return a tool error.

#### Scenario: Correct answer submitted
- **WHEN** `recall_answer` is called with `{"id": 1, "answer_index": 0}` and answer 0 is correct
- **THEN** the server evaluates `correct = true`, calls `AnswerQuestion(ctx, 1, 0, "Prevent storms", true, "")` — no AI feedback call
- **AND** returns `{"ok":true,"correct":true,"correct_index":0,"correct_text":"Prevent storms..."}`

#### Scenario: Incorrect answer submitted
- **WHEN** `recall_answer` is called with `{"id": 1, "answer_index": 1}` and correct_index is 0
- **THEN** `correct = false`, returns `{"ok":true,"correct":false,"correct_index":0,"correct_text":"..."}` and no AI call is made

#### Scenario: Skip submitted
- **WHEN** `recall_answer` is called with `{"id": 1, "skip": true}`
- **THEN** `SkipQuestion` is called, returns `{"ok":true}`, and `answer_index`, `correct`, `feedback` remain NULL

#### Scenario: Unknown question ID
- **WHEN** `recall_answer` is called with an ID that does not exist
- **THEN** the tool returns an error result and no DB write occurs

#### Scenario: Breaking change on input struct
- **WHEN** an existing MCP caller passes `{"id": N, "answer": "text"}` (old format)
- **THEN** the caller receives a tool error (the `Answer string` field has been replaced with `AnswerIndex *int` and `Skip bool`)

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

### Requirement: recall://recent resource returns enriched answer history
The `recall://recent` resource SHALL return the last 10 answered questions as a JSON array. Each item SHALL include `id`, `question`, `choices`, `correct_index`, `answer_index`, `correct`, and `feedback`. NULL fields SHALL be serialized as `null` (skipped rows and MCP rows have `feedback: null`; skipped rows also have `answer_index: null` and `correct: null`).

#### Scenario: Mixed answer history
- **WHEN** three questions have been answered (one correct via terminal, one incorrect via MCP, one skipped) and an MCP client reads `recall://recent`
- **THEN** the correct terminal row includes `"correct": true`, `"feedback": "<AI text>"`, `"answer_index": 0`, `"correct_index": 0`
- **AND** the incorrect MCP row includes `"correct": false`, `"feedback": null`, `"answer_index": 1`, `"correct_index": 0`
- **AND** the skipped row has `"answer_index": null`, `"correct": null`, `"feedback": null`

---

### Requirement: recall_workflow prompt instructs agent to self-explain
The `recall_workflow` prompt SHALL be registered and return a prompt message instructing the AI agent to: call `recall_next` after git commits, present questions to the user, record answers with `recall_answer`, tell the user whether they were correct, provide a brief direct explanation (especially for incorrect answers) using its own knowledge, NOT call a separate AI tool for the explanation, and continue normally if the queue is empty.

#### Scenario: Agent retrieves workflow prompt
- **WHEN** an MCP client retrieves the `recall_workflow` prompt
- **THEN** the prompt text instructs the agent to self-explain after recording an answer and explicitly states not to invoke a separate AI tool for the explanation
