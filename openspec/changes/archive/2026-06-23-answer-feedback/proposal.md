## Why

Phase 4A delivered questions to developers — via MCP to AI agent harnesses and via `tr ask` to terminal users. But the interaction loop is incomplete: when a developer answers a question, they receive only `"✓ recorded"` regardless of whether they were right. The correct answer is never persisted. There is no evaluation. The dormant intelligence sitting in `recall.Question.CorrectIndex` — computed and shuffled correctly at synthesis time — is silently dropped when it crosses into the store.

Phase 4C closes the loop. After a developer answers:

1. The server evaluates correctness against the persisted `correct_index`.
2. For terminal users: the AI generates a brief, direct explanation — naming the correct answer explicitly, explaining why it is right, and noting (without apology) why the chosen answer doesn't fit. The `tr ask` subcommand blocks on this response and renders it before returning to the shell prompt.
3. For MCP users: `recall_next` returns `correct_index` upfront so the agent can evaluate and self-explain using its own intelligence. No redundant AI call on the server side.
4. All answer data — `answer_index`, `correct`, `feedback` — is persisted to `memory.db`, making `recall://recent` a genuine performance history and laying the foundation for spaced repetition.

After this phase, answering a recall question is a complete cognitive event, not just a fire-and-forget acknowledgement.

## What Changes

- `memory.db` questions table extended: `correct_index`, `answer_index`, `correct`, `feedback` columns added via idempotent `ALTER TABLE ADD COLUMN` migrations
- `SaveQuestion` signature updated: accepts `correctIndex int` — the value from `recall.Question.CorrectIndex` is now persisted at enqueue time
- `StoredQuestion` struct updated: new fields `CorrectIndex`, `AnswerIndex`, `Correct`, `Feedback`
- New store methods: `GetQuestion` (lookup by ID for answer evaluation), `AnswerQuestion` (records index + text + correctness + feedback), `SkipQuestion` (records skip, no evaluation)
- `POST /recall/answer` extended: decodes `answer_index` (not answer text); looks up question from DB; evaluates correctness; if `?feedback=true`, calls AI via `recall.Engine.GenerateFeedback`; responds with `{ok, correct, correct_text, feedback}`
- `POST /recall/answer` body extended with optional `skip: true` field — skip path calls `SkipQuestion`, returns `{ok: true}` with no feedback
- `GET /recall/next` response unchanged for terminal callers — `correct_index` is not returned (stays server-side)
- `recall.Engine` gains `GenerateFeedback` method — builds a `FeedbackRequest` from `internal/recall/prompts.go`, calls the configured AI provider, returns plain-prose explanation (≤ 150 tokens)
- `tr ask` subcommand extended: `postAnswer` sends `answer_index` and `?feedback=true`; new `stateFeedback` state displays "Evaluating..." while blocking on response; post-alt-screen output renders correct/incorrect/skip appropriately
- `recall_next` MCP tool response extended: includes `correct_index` field for MCP callers
- `recall_answer` MCP tool input extended: accepts `answer_index` (int); computes and returns `{ok, correct, correct_index, correct_text}` — no AI call on MCP path
- `recallWorkflowInstructions` updated: instructs agent to self-explain after recording an answer
- `recall://recent` resource enriched: includes `correct`, `feedback` (where non-null) in each row

## Capabilities

### New Capabilities

- `answer-schema`: extended questions table with `correct_index`, `answer_index`, `correct`, `feedback`; `GetQuestion`, `AnswerQuestion`, `SkipQuestion` store methods; `SaveQuestion` carries `correctIndex`
- `feedback-generation`: `recall.Engine.GenerateFeedback` AI call; `FeedbackRequest` prompt builder in `internal/recall/prompts.go`; `POST /recall/answer?feedback=true` server-side evaluation + AI feedback path

### Modified Capabilities

- `question-store`: `SaveQuestion` signature, `StoredQuestion` struct, `RecentAnswered` output — all updated for new schema fields
- `question-delivery`: `POST /recall/answer` — extended request/response contract; `GET /recall/next` — unchanged for terminal, `correct_index` withheld
- `tr-ask`: new `stateFeedback` state; `postAnswer` sends `answer_index`; blocking feedback render; skip acknowledgement
- `mcp-server`: `recall_next` exposes `correct_index`; `recall_answer` evaluates and returns correctness; `recallWorkflowInstructions` updated; `recall://recent` enriched

## Impact

- `internal/cache/store.go` — schema migration, new/updated store methods, `StoredQuestion` struct
- `internal/recall/prompts.go` — `FeedbackRequest` builder, `feedbackMaxTokens` constant
- `internal/recall/engine.go` — `GenerateFeedback` method
- `internal/engine/server.go` — `handleRecallAnswer` (evaluation, AI call, extended response), `runPipeline` (passes `CorrectIndex` to `SaveQuestion`)
- `internal/engine/mcp.go` — `recall_next` response, `recall_answer` input/response, `recall://recent` resource, `recallWorkflowInstructions`
- `cmd/total-recall/ask.go` — `stateFeedback`, `postAnswer`, post-alt-screen rendering
- `DOCS/ARCHITECTURE/DELIVERY.md` — Phase 4C answer feedback loop documented
- `ROADMAP.md` — Phase 4C entry added
- `DOCS/TESTING/E2E.md` — Phase 4C E2E section added

## Key Design Decisions

- **Correct answer withheld from terminal client until after answer**: `GET /recall/next` does not return `correct_index`. The terminal client cannot pre-evaluate — it submits `answer_index` and receives the verdict from the server. This preserves integrity of the quiz for terminal users.
- **MCP client receives `correct_index` upfront**: AI agent harnesses get `correct_index` in the `recall_next` response. An AI agent seeing the answer doesn't invalidate the quiz — it still drives human reflection. This avoids a redundant server-side AI call for an audience that has its own reasoning capability.
- **Evaluation is server-side arithmetic, not AI**: `correct = (answer_index == correct_index)`. The AI call (feedback generation) receives the pre-computed verdict and explains only. This keeps feedback consistent across models and allows correctness to be recorded even if the AI feedback call fails.
- **Feedback AI call is opt-in via `?feedback=true`**: The MCP path never sends this flag. The terminal path always does. Future callers (e.g. a REST API consumer) can opt in or out explicitly.
- **`GenerateFeedback` lives on `recall.Engine`**: The Engine already owns the AI provider. Adding feedback generation there keeps AI logic co-located, avoids re-wiring provider dependencies, and makes `FeedbackRequest` a natural sibling of `SynthesisRequest` in `prompts.go`.
- **Skip stores no feedback, no correctness**: `SkipQuestion` records `answer = "skip"`, `answered_at`, and leaves `answer_index`, `correct`, `feedback` NULL. Terminal shows `"→ Question saved for later."` — a gentle acknowledgement, not a dismissal.
- **Feedback persisted alongside the answer**: generated once, at the moment of highest context. Re-generating later costs another AI call and loses specificity. The `feedback TEXT` column is nullable — MCP rows and skips are NULL with no cost.
- **`answer` column kept alongside new `answer_index`**: existing rows have `answer` (text). New rows populate both. The text field retains human-readability; the index field enables programmatic evaluation without string comparison.

## Non-Goals

- VS Code extension (Phase 4B — already deferred)
- Spaced repetition / difficulty progression (future change, now has the data foundation it needs)
- Scoring dashboards or `tr review` subcommand (future change)
- Feedback for skipped questions
- Multi-model feedback routing (single configured provider handles all AI calls)
- Streaming feedback response to terminal (single-shot response is sufficient at ≤ 150 tokens)
