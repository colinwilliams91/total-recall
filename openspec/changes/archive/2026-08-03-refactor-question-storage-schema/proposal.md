## Why

The `questions` table encodes the engine's correctness key and the user's pick as positional references into a JSON array (`correct_index` / `answer_index`), stores audit timestamps as overloaded state discriminators, and uses the word "answer" for both the user's submission and the engine's key. As the system grows toward multi-select and free-text questions with possible adaptive resurfacing, the schema cannot represent multiple picks per question, has no audit spine for lifecycle extension, and conflates the two "answer" concepts. The refactor is cheap now (zero live users) and becomes prohibitively breaking later.

## What Changes

**BREAKING — Storage schema (full teardown migration via SQL, no in-app migration machinery):**
- Introduces a `choices` table (one row per choice, with `position`, `text`, `is_correct`) — the engine's correctness key becomes a row property, not a positional reference into JSON.
- Introduces a `selections` join table (M:N between `questions` and `choices`) — the user's pick(s) become FK rows, not a single positional integer.
- Introduces a `question_events` event-log table — every lifecycle transition (`queued`/`delivered`/`answered`/`skipped`) is recorded as a row with `event_type`, `occurred_at`, `actor`, and an extensible `payload` JSON column. Becomes the source of truth for audit/ordering; future lifecycle events (`claimed`, `graded`, `resurfaced`, etc.) are row inserts, not `ALTER TABLE`.
- Replaces three overloaded `DATETIME` columns on `questions` (`queued_at`, `delivered_at`, `answered_at`) with the event log. `questions` becomes pure content + a `status` cache column.
- Adds a `status` CHECK-constrained enum (`queued`/`delivered`/`answered`/`skipped`) column to `questions`. Denormalized cache for fast `WHERE` filtering; the source of truth is `question_events`. Replaces the `answer = 'skip'` magic-string sentinel and the implicit `delivered_at IS NULL` / `answered_at IS NOT NULL` state encoding.
- Adds a `question_type` CHECK-constrained enum (`multiple_choice`/`multi_select`/`free_text`) column to `questions`, frontloaded to dispatch future rendering/grading logic. Only `multiple_choice` ships now; `multi_select` and `free_text` are reserved values.
- Adds `questions.correct_answer` (nullable TEXT) — the free-text correctness key, populated only for `question_type = 'free_text'`. Scoring (exact-match, partial-credit, AI-graded) is deferred to a later phase.
- Drops `questions.choices` (JSON TEXT), `questions.correct_index`, `questions.answer_index`, `questions.answer`, `questions.correct`, `questions.delivered_at`, `questions.claimed_by`, `questions.answered_at`, `questions.queued_at`.
- `selections` carries no timestamp columns — "when a selection was made" is derived from the parent question's `answered` event in `question_events`.
- `correct_answer` text is denormalized on the wire response (send-both: `correct_index` + `correct_answer`) to support stateless MCP consumers; no denormalization in the DB (single source of truth).

**BREAKING — Wire contract (full break, "answer" banned on user side per the amended lexical rule):**
- Delivery (`GET /recall/next`, `recall_next`): withholds `correct_index` from the response. Fixes the delivery leak where the correctness key crosses the wire before submission.
- Submit (`POST /recall/answer` renamed to `POST /recall/select`, `recall_answer` renamed to `recall_select`): MC-1 sends `{id, selected_index}`; MC-N (when it lands) sends `{id, selected_indices:[...]}`; free-text (when it lands) sends `{id, selected_text}`; skip is uniform across types: `{id, skip:true}`.
- Submit-response: MC-1 sends `{ok, correct, correct_index, correct_answer}`; MC-N sends `{ok, correct, correct_indices, correct_answers}`; free-text sends `{ok, correct, correct_answer}`; skip sends `{ok, skipped:true}`.
- Lexical rule (amended): user side uses `select`/`selected` exclusively — "answer" is banned. Engine side permits `answer`/`correct` (engine-inference output). `correct_index`/`correct_answer` are engine-side; `selected_index`/`selected_indices` are user-side.

**Application-side cascades:**
- `recall.Question` struct reshapes from `Choices []string` + `CorrectIndex int` to `Choices []Choice` where `Choice { Text string; IsCorrect bool }`. The `rand.Shuffle` in `Synthesize` stays (presentation logic) but produces the `Position` ordering; the `correctIdx == i` index arithmetic disappears.
- `recall.GenerateFeedback` signature reshapes from `(question string, choices []string, correctIndex, answerIndex int, ...)` to take `[]Choice` (or `correctIndex` + `answerIndex` derived from the choice rows).
- `cache.Store` methods reshape: `SaveQuestion` inserts `questions` + `choices` rows in one transaction; `AnswerQuestion` becomes `SubmitSelection` (insert `selections` rows + `answered` event row); `SkipQuestion` becomes `SkipQuestion` (insert `skipped` event row). `GetQuestion` joins `choices`; `NextQuestion` claims via `status`/event transaction; `RecentAnswered` filters on `status = 'answered'`, `RecentSkipped` filters on `status = 'skipped'`, `RecentQuestions` filters on `status IN ('answered','skipped')` — three distinct methods.
- No `PRAGMA user_version` probe in `Open()`; maintainer runs a teardown SQL script (or deletes `~/.tr/memory.db`) before running the new build.

## Capabilities

### New Capabilities
- `question-events`: Audit event-log spine backing the `questions` lifecycle. Records every transition as a row (`queued`/`delivered`/`answered`/`skipped`, extensible to `claimed`/`graded`/`resurfaced` later) with `event_type`, `occurred_at`, `actor`, and `payload`. Source of truth for ordering and audit; `questions.status` is a denormalized cache updated atomically in the same transaction.

### Modified Capabilities
- `question-store`: Schema reshape — `choices` and `selections` join tables replace positional JSON and `answer_index`/`correct_index` columns; `status` enum replaces implicit datetime-based state discrimination and the `answer='skip'` sentinel; `question_type` enum discriminates MC vs. multi-select vs. free-text; `question_events` replaces audit timestamps on `questions`; `RecentAnswered`/`RecentSkipped`/`RecentQuestions` split replaces the single `RecentAnswered` query.
- `question-delivery`: Wire contract — `correct_index` withheld at delivery (leak fix); submit endpoint renamed from `/recall/answer` to `/recall/select` and request fields renamed from `answer_index` to `selected_index`; submit-response includes both `correct_index` and `correct_answer` text; "answer" banned from the user side of the wire, replaced with "select".
- `recall-engine`: `Question` struct reshapes from `Choices []string` + `CorrectIndex int` to `Choices []Choice{Text, IsCorrect}`; shuffle stays in `recall` (presentation logic) but produces the choice-row `Position` ordering rather than the `correctIdx == i` index arithmetic; `GenerateFeedback` signature reshapes to consume choice-row semantics.

## Impact

- **Code:** `internal/cache/store.go` (full schema rewrite, all query methods, ~410 lines affected); `internal/recall/engine.go` (struct + shuffle + feedback signature); `internal/engine/server.go` (submit handler rename + response fields + delivery leak fix); `internal/engine/mcp.go` (tool rename `recall_answer`→`recall_select`, workflow instructions update, response fields); `cmd/tr/ask.go` (wire client `answer_index`→`selected_index`).
- **Tests:** `cmd/tr/cache_test.go`, `cmd/tr/integration_test.go`, `cmd/tr/ask_test.go`, `cmd/tr/golden_test.go` all touched by the rename + schema; golden files regenerated via `UPDATE_GOLDEN=1`.
- **APIs:** Daemon HTTP `/recall/answer`→`/recall/select`; MCP `recall_answer`→`recall_select` tool; field renames on delivery + submit-response.
- **Dependencies:** None new (still `modernc.org/sqlite`, still `gioui.app/u/`-free, no CGo).
- **Migration:** Teardown SQL script (drop `questions` table or delete `~/.tr/memory.db`); no in-app migration code; maintainer runs once before deploying the new build. Zero data preservation.
- **Future alignments:** Adaptive resurfacing can reuse `question_events` rows (`resurfaced` event) without schema migration; multi-select delivery uses `selected_indices` with no further schema change; free-text delivery uses `selected_text` and stores no `selections`/`choices` rows.