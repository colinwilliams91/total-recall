## MODIFIED Requirements

### Requirement: recall_next tool atomically dequeues one question, scoped to repo and branch
The `recall_next` tool SHALL accept `repo` and `branch` input fields (the absolute repository path and the branch name). It SHALL call `store.NextQuestion(ctx, repo, branch, "mcp")` where `repo` and `branch` are the supplied values or `""` when omitted. It SHALL return the question, choices, and `correct_index` as JSON content when a question is available, or `{"question":null}` when the queue is empty for that repo and branch. `claimed_by` SHALL be set to `"mcp"`. `correct_index` is included so AI agents can evaluate answers locally.

When either `repo` or `branch` is `""` (omitted by the agent or the user is outside a git repo), `NextQuestion` shall return `nil, nil` (no question) and the tool shall return `{"question":null}` — no global pool exists.

#### Scenario: Question available for the supplied repo and branch
- **WHEN** `recall_next` is called with `{"repo": "/path/X", "branch": "feature-X"}` and a question with `repo=/path/X` AND `branch=feature-X` exists with `delivered_at IS NULL`
- **THEN** the tool returns `{"id":N,"question":"...","choices":[...],"correct_index":N}` and the row is marked delivered

#### Scenario: Repo or branch omitted - no question returned (no global pool)
- **WHEN** `recall_next` is called with `{}` (no `repo` or `branch` field)
- **THEN** the tool returns `{"question":null}` — no global pool dequeue occurs; the AI agent should pass both `repo` and `branch` derived from `git rev-parse --show-toplevel` and `git rev-parse --abbrev-ref HEAD`

#### Scenario: Queue empty for the repo and branch
- **WHEN** `recall_next` is called with `{"repo": "/path/Y", "branch": "main"}` and no unclaimed questions exist for `/path/Y` AND `main`
- **THEN** the tool returns `{"question":null}`

#### Scenario: Branch-isolation enforced - main's caller doesn't see feature-X's questions
- **WHEN** `recall_next` is called with `{"repo": "/path/X", "branch": "main"}` and only questions with `branch = "feature-X"` exist for `/path/X`
- **THEN** the tool returns `{"question":null}` — strict branch isolation

---

### Requirement: recall_answer tool evaluates and returns correctness (ID-keyed, no AI call)
The `recall_answer` tool SHALL accept `{"id": N, "answer_index": N, "skip": true, "repo": "/path", "branch": "feature-X"}` where `repo` and `branch` are optional and accepted for symmetry. On skip, it SHALL call `store.SkipQuestion` and return `{"ok":true}`. On answer, it SHALL call `store.GetQuestion`, evaluate `correct = (answer_index == correct_index)`, call `store.AnswerQuestion` with empty feedback (no AI call on MCP path), and return `{"ok":true,"correct":<bool>,"correct_index":N,"correct_text":"..."}`. `AnswerQuestion`, `SkipQuestion`, and `GetQuestion` are ID-keyed, so `repo` and `branch` are accepted for symmetry but do not affect the operation. On unknown ID, it SHALL return a tool error. On out-of-range `answer_index`, it SHALL return a tool error.

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

### Requirement: recall_status tool returns daemon health, scoped to repo and branch
The `recall_status` tool SHALL accept `repo` and `branch` input fields. It SHALL return `{"daemon":"ok","ai_configured":bool,"queue_depth":N}` where `ai_configured` is true when `cfg.AI.Provider` is non-empty and `queue_depth` is the count of unclaimed questions for the supplied `repo` AND `branch`. When either `repo` or `branch` is omitted (or empty), `queue_depth` SHALL be `0` — no global pool exists.

#### Scenario: Status for a specific repo and branch
- **WHEN** `recall_status` is called with `{"repo": "/path/X", "branch": "feature-X"}`
- **THEN** `queue_depth` reflects only unclaimed questions with `repo=/path/X` AND `branch=feature-X`

#### Scenario: Status without repo or branch
- **WHEN** `recall_status` is called with `{}`
- **THEN** `queue_depth` is `0` — no global pool

#### Scenario: Branch-isolation in status - main's queue_depth excludes feature-X's
- **WHEN** `recall_status` is called with `{"repo": "/path/X", "branch": "main"}` and `/path/X` has 3 pending on `feature-X`, 0 on `main`
- **THEN** `queue_depth` is `0`

---

### Requirement: recall://queue resource is subscribable and repo-and-branch-scoped
The `recall://queue` resource SHALL be registered with `server.AddResource`. Its handler SHALL return current queue depth and the next pending question (peeked without dequeuing) for the requested repo AND branch as JSON. The `repo` and `branch` SHALL be derived from the resource URI query string (`recall://queue?repo=/path/X&branch=feature-X`) or default to empty when absent. When either is empty (no query string supplied), the handler SHALL return `{"depth":0,"next":null}` without querying the store — no global pool exists. Connected clients that subscribe SHALL receive `notifications/resources/updated` via SSE when `runPipeline` calls `NotifyResourceUpdated("recall://queue")`.

Resource template URI: `recall://queue{?repo}{&branch}` (RFC 6570 compliant — `repo` and `branch` are both optional query parameters).

#### Scenario: Question synthesized for a repo and branch
- **WHEN** `runPipeline` saves a new question with `repo=/path/X` AND `branch=feature-X` to `memory.db`
- **THEN** `NotifyResourceUpdated("recall://queue")` is called for each connected MCP session

---

### Requirement: recall://recent resource returns repo-and-branch-scoped answer history
The `recall://recent` resource SHALL return the last 10 answered questions for the requested repo AND branch as a JSON array. The `repo` and `branch` SHALL be derived from the resource URI query string (`recall://recent?repo=/path/X&branch=feature-X`) or default to empty when absent. When either is empty, the handler SHALL return an empty array `[]` without querying the store — no global answered history exists. Each item SHALL include `id`, `question`, `choices`, `correct_index`, `answer_index`, `correct`, and `feedback`. NULL fields SHALL be serialized as `null`.

Resource template URI: `recall://recent{?repo}{&branch}`.

#### Scenario: Recent answered questions returned for repo and branch
- **WHEN** `recall://recent?repo=/path/X&branch=feature-X` is requested and 3 answered questions exist for that repo AND branch
- **THEN** the resource returns a JSON array of 3 items, each with `id`, `question`, `choices`, `correct_index`, `answer_index`, `correct`, and `feedback` fields

#### Scenario: Recent empty when no repo or branch
- **WHEN** `recall://recent` is requested without a `repo` or `branch` query string
- **THEN** the resource returns `[]` without querying the store

#### Scenario: Branch-isolation in recent history
- **WHEN** `recall://recent?repo=/path/X&branch=main` is requested and only `branch=feature-X` questions exist for `/path/X`
- **THEN** the resource returns `[]` — strict branch isolation

---

### Requirement: recall_workflow prompt instructs agent to self-explain and pass repo + branch
The `recall_workflow` prompt SHALL be registered and return a prompt message instructing the AI agent to: call `recall_next` after git commits (passing both the absolute repo path from `git rev-parse --show-toplevel` and the current branch from `git rev-parse --abbrev-ref HEAD`), present questions to the user, record answers with `recall_answer`, tell the user whether they were correct, provide a brief direct explanation using its own knowledge, NOT call a separate AI tool for the explanation, and continue normally if the queue is empty.

#### Scenario: Agent retrieves workflow prompt
- **WHEN** an MCP client retrieves the `recall_workflow` prompt
- **THEN** the prompt text instructs the agent to self-explain after recording an answer and to pass both `repo` and `branch` to `recall_next` derived from git commands