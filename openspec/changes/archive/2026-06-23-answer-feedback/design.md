## Context

Phase 4A left the answer interaction in an incomplete state: `AnswerQuestion` records a choice text string and a timestamp, nothing more. `recall.Question.CorrectIndex` — correctly computed and shuffled during synthesis — is never passed to `SaveQuestion` and has no column to land in. The feedback shown to the developer is unconditional `"✓ recorded"`.

This design wires the evaluation loop. The central changes are: (1) extend the schema to carry `correct_index` through the full lifecycle, (2) add server-side evaluation at answer time, (3) add a feedback AI call on the terminal path only, (4) update the terminal state machine to block on and render the result, and (5) update MCP tools to return correctness data and trust the agent to self-explain.

## Goals / Non-Goals

**Goals:**
- Persist `correct_index` at enqueue time; persist `answer_index`, `correct`, `feedback` at answer time
- Server evaluates correctness as arithmetic (`answer_index == correct_index`), not AI
- Terminal path (`POST /recall/answer?feedback=true`) calls AI for explanation, blocks on response
- MCP path (`recall_answer` tool) evaluates locally, returns correctness data, no AI call
- `tr ask` renders correct / incorrect / skip feedback after alt-screen closes
- `recall://recent` enriched with correctness and feedback for answered rows

**Non-Goals:**
- Spaced repetition, scoring, streaks (future change)
- VS Code extension (Phase 4B)
- Streaming feedback
- Skip feedback

## Decisions

### 1. Schema extension — idempotent column migrations

**Decision**: New columns are added to the existing `questions` table using `ALTER TABLE ADD COLUMN` statements, guarded by a helper that ignores "duplicate column name" errors. `CREATE TABLE IF NOT EXISTS` is updated to include the new columns for fresh installs.

```sql
-- Added to existing tables via addColumnIfMissing():
ALTER TABLE questions ADD COLUMN correct_index INTEGER NOT NULL DEFAULT 0;
ALTER TABLE questions ADD COLUMN answer_index   INTEGER;
ALTER TABLE questions ADD COLUMN correct        INTEGER;   -- 1=correct, 0=wrong, NULL=skip/pending
ALTER TABLE questions ADD COLUMN feedback       TEXT;      -- NULL for MCP rows and skips
```

Full schema for fresh installs:
```sql
CREATE TABLE IF NOT EXISTS questions (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    question       TEXT     NOT NULL,
    choices        TEXT     NOT NULL,
    correct_index  INTEGER  NOT NULL DEFAULT 0,
    queued_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    delivered_at   DATETIME,
    claimed_by     TEXT,
    answer         TEXT,           -- choice text or "skip"; kept for human readability
    answer_index   INTEGER,        -- 0-based; NULL if skipped
    correct        INTEGER,        -- 1=correct, 0=wrong, NULL=skip/pending
    feedback       TEXT,           -- AI explanation; NULL for MCP rows and skips
    answered_at    DATETIME
);
```

`DEFAULT 0` on `correct_index` is safe for existing rows — no existing row has a meaningful correct index anyway (it was never stored). Future rows will always have the real value.

**Rationale**: SQLite does not support transactional DDL renaming or dropping columns without full table recreation. `ALTER TABLE ADD COLUMN` is the lowest-risk migration path. Keeping the existing `answer` (text) column alongside the new `answer_index` avoids destructive migration and retains human-readable history.

---

### 2. `correct_index` information boundary

**Decision**:

```
GET /recall/next (terminal)          recall_next tool (MCP)
─────────────────────────────        ──────────────────────────────
{                                    {
  "id": 1,                             "id": 1,
  "question": "...",                   "question": "...",
  "choices": [...]                     "choices": [...],
}                                      "correct_index": 0    ← included
← correct_index withheld              }
```

The terminal client cannot pre-evaluate. It submits `answer_index`; the server returns the verdict. The MCP client receives `correct_index` upfront — an AI agent seeing the answer doesn't invalidate the quiz; it still prompts human reflection. Withholding it from the MCP path would require a redundant server-side AI call for an audience with its own reasoning capability.

---

### 3. Two distinct answer paths — `POST /recall/answer`

**Terminal path** (`?feedback=true`):

```
POST /recall/answer?feedback=true
Body: {"id": 1, "answer_index": 0}

Server:
  1. GetQuestion(id) → StoredQuestion (with correct_index, choices)
  2. correct = (answer_index == question.correct_index)
  3. answer_text = question.choices[answer_index]
  4. correct_text = question.choices[question.correct_index]
  5. recallEngine.GenerateFeedback(ctx, question, answer_index, model) → feedback string
  6. AnswerQuestion(ctx, id, answer_index, answer_text, correct, feedback)
  7. Respond: {ok, correct, correct_text, feedback}

Response: {"ok":true,"correct":false,"correct_text":"Prevent storms...","feedback":"Jitter randomizes..."}
```

**MCP path** (no `?feedback=true`):

```
recall_answer tool
Input: {"id": 1, "answer_index": 0}

Server:
  1. GetQuestion(id) → StoredQuestion
  2. correct = (answer_index == question.correct_index)
  3. answer_text = question.choices[answer_index]
  4. AnswerQuestion(ctx, id, answer_index, answer_text, correct, "")
     (feedback stored as NULL)
  5. Return: {ok, correct, correct_index, correct_text}
  ← no AI call

Agent is instructed (via recall_workflow prompt) to self-explain.
```

**Skip path** (either caller):

```
POST /recall/answer
Body: {"id": 1, "skip": true}

  OR

recall_answer tool
Input: {"id": 1, "skip": true}

Server:
  1. SkipQuestion(ctx, id) → sets answer="skip", answered_at=now()
     leaves answer_index, correct, feedback NULL
  2. Respond: {"ok": true}
```

---

### 4. Feedback AI call — `recall.Engine.GenerateFeedback`

**Decision**: A new method on the existing `recall.Engine`:

```go
func (e *Engine) GenerateFeedback(
    ctx context.Context,
    question string,
    choices []string,
    correctIndex int,
    answerIndex int,
    model string,
) (string, error)
```

It calls `recall.FeedbackRequest(...)` (new function in `prompts.go`) to build the `ai.CompletionRequest`, then calls `e.provider.Complete`. On error, it logs and returns `""` — the handler continues with empty feedback rather than failing the answer record. Feedback failure must never prevent the answer from being stored.

**System prompt (fixed)**:
```
You are a technical recall assistant giving immediate feedback after a developer
answers a quiz question. Be direct, concise, and informative. Do not use
markdown, asterisks, bullet points, or headers. Write in plain prose.
Maximum 3 sentences.

If the developer was correct: briefly confirm and add one sentence explaining
why that answer is right — not just that it is right.

If the developer was incorrect: state the correct answer explicitly, explain
why it is right, and briefly note why their chosen answer doesn't fit.
Do not apologize or soften excessively.
```

**User turn template (correct)**:
```
Question: <question>
Choices:
  [1] <choice 0>  ← correct, chosen
  [2] <choice 1>
  [3] <choice 2>

The developer answered correctly.
```

**User turn template (incorrect)**:
```
Question: <question>
Choices:
  [1] <choice 0>  ← correct
  [2] <choice 1>  ← chosen (incorrect)
  [3] <choice 2>

The developer chose option 2 and was incorrect.
```

All choices are included so the AI has the full distractor set and can speak to why the chosen wrong answer doesn't fit.

`feedbackMaxTokens = 150` — enforces the 3-sentence ceiling. If the AI response exceeds this, the SDK truncates; the constraint is intentional discipline.

---

### 5. `tr ask` — `stateFeedback` state

**Decision**: New state added between `stateQuestion` and `stateDone`:

```
stateThinking
    │
    ▼ (question received)
stateQuestion
    │
    ├─── [1-3] pressed ──▶  postAnswer(id, answer_index) as tea.Cmd
    │                              │
    │                              ▼ (async, blocks Bubbletea)
    │                        stateFeedback
    │                        "Evaluating..."
    │                              │
    │                              ▼ (feedbackMsg received)
    │                        stateDone → alt-screen closes
    │
    ├─── Enter (skip) ────▶  postSkip(id) as tea.Cmd → stateDone
    │
    └─── q / Esc ─────────▶  stateDone (no POST — question remains unclaimed)
```

`stateFeedback` renders `"Evaluating..."` (no animation needed — the AI call is typically < 3 seconds). `postAnswer` is a `tea.Cmd` that returns a `feedbackMsg{correct, correctText, feedback}`. The model stores the result; after the alt-screen closes, `askCmd.RunE` prints the appropriate output.

**Post-alt-screen rendering** (printed to stdout after Bubbletea exits):

```
Correct case:
  ✓ Correct.

    <feedback sentence(s)>

Incorrect case:
  ✗ The answer was: <correct_text>

    <feedback sentence(s)>

Skip case:
  → Question saved for later.
```

Verdict line and `correct_text` are rendered by `tr ask` from response fields — not from AI output. The AI text is the paragraph only.

---

### 6. `recall://recent` enrichment

**Decision**: `RecentAnswered` is updated to SELECT and scan `answer_index`, `correct`, `feedback`. The `recall://recent` MCP resource includes these fields where non-null:

```json
[
  {
    "id": 1,
    "question": "Why is jitter added...",
    "choices": ["Prevent storms...", "Reduce memory", "Improve cache"],
    "correct_index": 0,
    "answer_index": 1,
    "correct": false,
    "feedback": "Jitter randomizes retry timing..."
  }
]
```

`correct_index` is included in `recall://recent` (unlike `recall_next`) because the question has already been answered — there is no integrity concern in exposing it after the fact.

---

### 7. `SaveQuestion` signature and `runPipeline` bridge

**Decision**: `SaveQuestion` gains a `correctIndex int` parameter:

```go
func (s *Store) SaveQuestion(ctx context.Context, question string, choices []string, correctIndex int) error
```

In `runPipeline`, the call becomes:
```go
s.store.SaveQuestion(ctx, q.Question, q.Choices, q.CorrectIndex)
```

`q.CorrectIndex` is already computed and shuffled by `recall.Synthesize` before `runPipeline` calls `SaveQuestion`. No changes to the synthesis flow.

---

## Risks / Trade-offs

**[Risk] Feedback AI call adds 2-5s latency to the answer UX** — the terminal user is already in a "pause and reflect" context post-commit; the latency is acceptable and contextually appropriate. Mitigation: `feedbackMaxTokens = 150` limits response time; a failed AI call degrades gracefully (answer is stored, feedback is blank, `tr ask` prints verdict without explanation paragraph).

**[Risk] `addColumnIfMissing` swallows errors** — the helper ignores "duplicate column name" but must propagate other errors (disk full, locked DB). Mitigation: error string inspection distinguishes duplicate-column from other SQLite errors; non-duplicate errors are returned and cause daemon startup failure.

**[Risk] `GetQuestion` adds a DB round-trip on every answer** — one extra SELECT before the UPDATE. At the scale of one question per commit, this is negligible. Mitigation: the SELECT and UPDATE can be combined into a single CTE if profiling ever reveals it as a bottleneck (future optimization, not necessary now).

**[Risk] Existing `answer` column retained alongside new `answer_index`** — minor schema redundancy. Mitigation: both columns are populated on every new answer; the text column retains human-readability for direct DB inspection; it is a natural input for any future `tr review` subcommand.
