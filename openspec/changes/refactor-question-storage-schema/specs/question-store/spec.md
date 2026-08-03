## MODIFIED Requirements

### Requirement: DB path is ~/.tr/memory.db (no legacy migration)
`store.Open()` SHALL open `~/.tr/memory.db` (or `$TR_HOME/memory.db` when `TR_HOME` is set). The legacy `concepts.db` → `memory.db` migration guard is removed — only `memory.db` is supported. No in-code schema migration (no `ALTER TABLE`, no row purge) SHALL run inside `Open()`. If `memory.db` does not exist, `Open()` creates it with the full new schema (`CREATE TABLE IF NOT EXISTS` for `questions`, `choices`, `question_events`, `selections`, with all final columns including `repo` and `branch` on `questions`, `question_type` and `status` CHECK-constrained enums, plus the `idx_qe_qid_time` index). If `memory.db` exists with the prior schema, `Open()`'s `CREATE TABLE IF NOT EXISTS` is a no-op against the stale tables — the maintainer deletes the stale `memory.db` manually (or runs the teardown SQL script `DROP TABLE IF EXISTS questions`) before the new build runs.

#### Scenario: Fresh install
- **WHEN** neither `memory.db` nor `concepts.db` exist in the data directory
- **THEN** `Open()` creates `memory.db` with the four new tables (`questions` reshaped, `choices`, `question_events`, `selections`) including the CHECK-constrained `question_type` and `status` columns on `questions`, plus the `idx_qe_qid_time` partial index, and returns without error

#### Scenario: Existing memory.db with current schema
- **WHEN** `memory.db` already exists with the new schema
- **THEN** `Open()` opens it directly without attempting any migration

#### Scenario: Existing memory.db with stale schema
- **WHEN** `memory.db` exists with the prior schema (positional `correct_index`/`answer_index` columns, no `choices`/`question_events`/`selections` tables)
- **THEN** `Open()`'s `CREATE TABLE IF NOT EXISTS` is a no-op against the existing `questions` table; the binary does not attempt any in-code `ALTER TABLE` or data purge; the maintainer runs the teardown SQL (`DROP TABLE IF EXISTS questions`) or deletes `memory.db` manually per the design's migration plan, then restarts the daemon

---

### Requirement: questions table schema is created idempotently with repo, branch, status, and question_type columns
`store.Open()` SHALL run `CREATE TABLE IF NOT EXISTS questions (...)` with columns: `id INTEGER PRIMARY KEY AUTOINCREMENT`, `question_type TEXT NOT NULL DEFAULT 'multiple_choice'` with CHECK constraint enforcing `('multiple_choice','multi_select','free_text')`, `status TEXT NOT NULL DEFAULT 'queued'` with CHECK constraint enforcing `('queued','delivered','answered','skipped')`, `question TEXT NOT NULL`, `repo TEXT NOT NULL`, `branch TEXT NOT NULL`, `correct_answer TEXT` (nullable, populated only for `question_type = 'free_text'`), and `feedback TEXT` (nullable). The `questions` table SHALL NOT contain `queued_at`, `delivered_at`, `claimed_by`, `answer`, `answer_index`, `correct_index`, `correct`, or `answered_at` columns — those concepts move to `question_events`, `selections`, `choices`, and the `status` cache column. A covering partial index SHALL be created idempotently to support repo-AND-branch-scoped atomic dequeue (now keyed on `status = 'queued'` rather than `delivered_at IS NULL`).

#### Scenario: Idempotent schema creation across repeated opens
- **WHEN** `Open()` is called twice in succession against the same `memory.db` file
- **THEN** the second call's `CREATE TABLE IF NOT EXISTS` is a no-op; `questions` has `repo TEXT NOT NULL`, `branch TEXT NOT NULL`, `question_type TEXT NOT NULL DEFAULT 'multiple_choice'`, `status TEXT NOT NULL DEFAULT 'queued'` (both with their CHECK constraints), and `correct_answer TEXT` (nullable); the partial index for repo+branch-scoped dequeue exists

#### Scenario: Inserting an unknown question_type fails
- **WHEN** an `INSERT INTO questions (question_type, ...)` is attempted with `question_type = 'ranking'`
- **THEN** the insert fails the CHECK constraint and the transaction rolls back

#### Scenario: Inserting an unknown status fails
- **WHEN** a question row is updated with `status = 'pending'`
- **THEN** the update fails the CHECK constraint and the transaction rolls back

---

### Requirement: choices table stores one row per choice with the engine's correctness key as a row property
`store.Open()` SHALL create a `choices` table with columns: `id INTEGER PRIMARY KEY AUTOINCREMENT`, `question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE`, `position INTEGER` (display order, produced by the recall shuffle), `text TEXT NOT NULL`, `is_correct INTEGER NOT NULL DEFAULT 0` (the engine's correctness key — `1` for the correct choice, `0` for distractors). A question with `question_type = 'free_text'` SHALL have zero `choices` rows. A question with `question_type = 'multiple_choice'` (single-select) SHALL have exactly one row with `is_correct = 1` and at least one row with `is_correct = 0`. A question with `question_type = 'multi_select` `'` (when it lands) SHALL have one or more rows with `is_correct = 1`. A covering index `idx_choices_qid ON choices(question_id)` SHALL be created idempotently. The correctness key survives JSON reorder/truncate because it is a row property, not a positional reference into a serialized array.

#### Scenario: Multiple-choice question has exactly one is_correct=1 row
- **WHEN** a `multiple_choice` question is saved with 4 choices (one correct, three distractors) at positions [0,1,2,3]
- **THEN** exactly 4 `choices` rows exist for that question; exactly one row has `is_correct = 1` (the correct choice); the other 3 have `is_correct = 0`; the correct choice's `position` is whatever the recall shuffle assigned

#### Scenario: Free-text question has zero choices rows
- **WHEN** a `free_text` question is saved (when free-text ships)
- **THEN** no `choices` rows exist for that question; the `correct_answer` column on the `questions` row carries the key

#### Scenario: Free-text does not require correct_answer to be populated at save time
- **WHEN** a `free_text` question is saved without `correct_answer` populated (deferred-grading mode, e.g., AI rubric)
- **THEN** the `questions.correct_answer` column is NULL until a future grading pass populates it

---

### Requirement: selections table stores user picks as M:N rows between questions and choices
`store.Open()` SHALL create a `selections` table with columns: `id INTEGER PRIMARY KEY AUTOINCREMENT`, `question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE`, `choice_id INTEGER NOT NULL REFERENCES choices(id) ON DELETE CASCADE`, and a `UNIQUE(question_id, choice_id)` constraint enforcing one row per `(question, choice)` pair. For `multiple_choice` (single-select) exactly one `selections` row exists per submitted question. For `multi_select` (when it lands) N rows exist per submitted question. The `selections` table SHALL NOT carry a timestamp column — "when was this selection made?" is derived from the parent question's `'answered'` event in `question_events`. For a skipped question, no `selections` rows are inserted; the skip lives in `questions.status = 'skipped'` (atomic with a `'skipped'` event insert), never derived from absent `selections` rows. For free-text questions, a future `selected_text` column (or `selections.text TEXT` nullable) will carry the user's prose — deferred to free-text phase; the schema here does not presuppose that column.

#### Scenario: Single-select MC submission inserts exactly one selections row
- **WHEN** a user submits `selected_index = 2` for a `multiple_choice` question with 4 choices
- **THEN** exactly one `selections` row exists for that question, with `choice_id` set to the `choices.id` of the choice at position 2

#### Scenario: Skipped question has zero selections rows
- **WHEN** a user skips a question
- **THEN** no `selections` rows exist for that question; `questions.status = 'skipped'` and a `'skipped'` event was inserted atomically with the status update

#### Scenario: Duplicate selection for the same question+choice is refused
- **WHEN** an insert attempts to add a second `selections` row with the same `(question_id, choice_id)` pair
- **THEN** the insert fails the UNIQUE constraint and rolls back

---

### Requirement: SaveQuestion persists a synthesized question with repo, branch, and choices rows in one transaction
`(*Store).SaveQuestion(ctx, repo, branch, question, choices []Choice, questionType string)` SHALL, in a single transaction, insert one `questions` row with the `repo` column and the `branch` column set to the provided values, `question_type` set to the provided value (or `'multiple_choice'` as the default if empty), `status = 'queued'`, AND insert one `choices` row per element of `choices` with `position`, `text`, and `is_correct` set from the input, AND insert a `question_events` row with `event_type = 'queued'`. Both `repo` and `branch` MUST be non-empty. If either is empty, `SaveQuestion` SHALL return early (no-op) and log `[store] skipping savequestion: empty repo or branch`. The transaction commits atomically — the `questions` row, all `choices` rows, and the `question_events` 'queued' row appear together or none appear.

#### Scenario: Valid MC question saved for a repo and branch
- **WHEN** `SaveQuestion` is called with `repo = "/path/to/repo"`, `branch = "feature-X"`, and a `[]Choice` with 4 elements where exactly one has `IsCorrect = true`
- **THEN** one `questions` row with `status = 'queued'` exists, four `choices` rows with the correct `position`/`text`/`is_correct` values exist, and one `question_events` row with `event_type = 'queued'` and `actor = 'system'` exists, all from the same transaction

#### Scenario: Empty repo or branch refuses to save
- **WHEN** `SaveQuestion` is called with either `repo = ""` or `branch = ""`
- **THEN** no row is inserted in any table; the method returns `nil` (no error); the daemon logs `[store] skipping savequestion: empty repo or branch`

---

### Requirement: NextQuestion atomically claims one question scoped to repo and branch, inserting a 'delivered' event
`(*Store).NextQuestion(ctx, repo, branch, claimedBy)` SHALL, in a single transaction, atomically select and mark the oldest question where `status = 'queued' AND repo = ? AND branch = ?`, ordered by the `occurred_at` of the `'queued'` event for that question ascending. It SHALL update the `questions` row to `status = 'delivered'` AND insert a `question_events` row with `event_type = 'delivered'` and `actor` set to the provided `claimedBy`. Both `repo` and `branch` MUST be non-empty. If either is empty, the method SHALL return `(nil, nil)` without executing the query and without error. No "global pool" semantics exist for empty values.

#### Scenario: Question available for the repo and branch
- **WHEN** at least one question with `repo = "/path/X"`, `branch = "feature-X"` has `status = 'queued'`
- **THEN** exactly one caller receives that question (with its `choices` rows joined in); concurrent callers receive `(nil, nil)`; the claimed question has `status = 'delivered'` and a `'delivered'` event row exists in `question_events`

#### Scenario: Queue empty for the repo and branch
- **WHEN** no questions with `repo = "/path/X"`, `branch = "feature-X"` have `status = 'queued'`
- **THEN** `NextQuestion` returns `(nil, nil)` without error

#### Scenario: Repo-isolation - X's question not served to Y
- **WHEN** `NextQuestion(ctx, "/path/Y", "feature-X", "shell")` is called and only questions with `repo = "/path/X"` and `status = 'queued'` exist
- **THEN** `NextQuestion` returns `(nil, nil)` — repo Y does not receive repo X's questions

#### Scenario: Branch-isolation - feature-X's question not served to main
- **WHEN** `NextQuestion(ctx, "/path/X", "main", "shell")` is called and only questions with `branch = "feature-X"` and `status = 'queued'` exist for `/path/X`
- **THEN** `NextQuestion` returns `(nil, nil)` — `main` does not receive `feature-X`'s questions

---

### Requirement: GetQuestion fetches a question joined with its choices
`(*Store).GetQuestion(ctx, id)` SHALL return the `questions` row joined with its `choices` rows (empty slice for `free_text` questions). ID-keyed (globally unique) — no `repo` or `branch` parameter needed. Returns `(nil, nil)` when the row does not exist. The returned `StoredQuestion` carries the `choices` as a `[]Choice` with `Text` and `IsCorrect` flags and a derived `CorrectIndex` (the position of the `is_correct = 1` row), but the correctness key lives on the choice row itself — the `CorrectIndex` is a presentation aid for clients, not the source of truth.

#### Scenario: Existing MC question returned with choices
- **WHEN** `GetQuestion(ctx, 42)` is called for an existing `multiple_choice` question with 4 choices
- **THEN** the returned `StoredQuestion` has the question text, the 4 choices ordered by `position`, and a non-nil `CorrectIndex` pointing at the `is_correct = 1` choice's position

#### Scenario: Unknown question ID returns nil
- **WHEN** `GetQuestion(ctx, 99999)` is called
- **THEN** the method returns `(nil, nil)` without error

---

### Requirement: SubmitSelection records the user's pick as selections rows with an 'answered' event, atomically
`(*Store).SubmitSelection(ctx, questionID int64, selectedChoiceIDs []int64)` SHALL, in a single transaction, insert one `selections` row per `selectedChoiceID` (each `(question_id, choice_id)` pair keyed by UNIQUE), update the `questions` row to `status = 'answered'`, AND insert a `question_events` row with `event_type = 'answered'`. The transaction is atomic — all inserts and the status update succeed or all roll back. ID-keyed (globally unique) — no `repo` or `branch` parameter needed. The `question_events` row's `occurred_at` is the authoritative "when was this selection made" timestamp; `selections` carries no timestamp.

#### Scenario: Successful single-select MC submission
- **WHEN** `SubmitSelection(ctx, 42, []int64{7})` is called for an existing question with `status = 'delivered'` and a choice with `id = 7`
- **THEN** one `selections` row is inserted with `(question_id = 42, choice_id = 7)`; the `questions` row has `status = 'answered'`; one `question_events` row with `event_type = 'answered'` is inserted — all in one transaction

#### Scenario: Submit for an already-terminal question is rejected
- **WHEN** `SubmitSelection(ctx, 42, []int64{7})` is called for a question whose `status` is `'answered'` or `'skipped'`
- **THEN** the submit is rejected; no `selections` row is inserted; no `'answered'` event is inserted; `status` is unchanged

---

### Requirement: SkipQuestion records a skip via the status enum and a 'skipped' event, atomically
`(*Store).SkipQuestion(ctx, id)` SHALL, in a single transaction, update the `questions` row to `status = 'skipped'` AND insert a `question_events` row with `event_type = 'skipped'`. No `selections` rows are inserted. ID-keyed (globally unique) — no `repo` or `branch` parameter needed. This replaces the prior `answer = 'skip'` magic-string sentinel — the skip lives in the `status` column (atomic with the `'skipped'` event), never derived from absent `selections` rows.

#### Scenario: Skip recorded
- **WHEN** `SkipQuestion(ctx, 42)` is called for an existing question
- **THEN** the `questions` row has `status = 'skipped'`; one `question_events` row with `event_type = 'skipped'` exists; no `selections` rows exist for that question — all in one transaction

---

### Requirement: QueueDepth returns queued count scoped to repo and branch
`(*Store).QueueDepth(ctx, repo, branch)` SHALL return `SELECT COUNT(*) FROM questions WHERE status = 'queued' AND repo = ? AND branch = ?`. Both `repo` and `branch` MUST be non-empty. If either is empty, the method SHALL return `(0, nil)` without executing the query.

#### Scenario: Queue depth for repo and branch
- **WHEN** `QueueDepth(ctx, "/path/X", "feature-X")` is called and 3 questions with `status = 'queued'` exist for that repo AND branch
- **THEN** the method returns `(3, nil)`

#### Scenario: Empty repo or branch returns zero without query
- **WHEN** `QueueDepth` is called with `repo = ""` or `branch = ""`
- **THEN** the method returns `(0, nil)` without executing the query

---

### Requirement: PeekNextQuestion returns the next queued question scoped to repo and branch
`(*Store).PeekNextQuestion(ctx, repo, branch)` SHALL return the oldest question where `status = 'queued' AND repo = ? AND branch = ?`, ordered by the `occurred_at` of the `'queued'` event for that question ascending, WITHOUT claiming it. Both `repo` and `branch` MUST be non-empty. If either is empty, the method SHALL return `(nil, nil)` without executing the query. Used by the `recall://queue` resource handler.

#### Scenario: Peek returns oldest queued without claiming
- **WHEN** `PeekNextQuestion(ctx, "/path/X", "feature-X")` is called and a queued question exists for that repo AND branch
- **THEN** the method returns that question (joined with its choices) without updating `status` — `status` remains `'queued'` and no `'delivered'` event is inserted

#### Scenario: Empty repo or branch returns nil without query
- **WHEN** `PeekNextQuestion` is called with `repo = ""` or `branch = ""`
- **THEN** the method returns `(nil, nil)` without executing the query

---

### Requirement: RecentAnswered returns answered questions scoped to repo and branch
`(*Store).RecentAnswered(ctx, repo, branch, limit)` SHALL return up to `limit` questions where `status = 'answered' AND repo = ? AND branch = ?`, ordered by the `occurred_at` of the `'answered'` event descending. Skipped questions are EXCLUDED — `RecentSkipped` covers that case. Both `repo` and `branch` MUST be non-empty. If either is empty, the method SHALL return `(nil, nil)` without executing the query. Each returned `StoredQuestion` joins its `selections` rows for grading display.

#### Scenario: Recent answered questions for repo and branch excludes skips
- **WHEN** `RecentAnswered(ctx, "/path/X", "feature-X", 10)` is called, 5 answered questions exist for that repo AND branch, and 2 skipped questions also exist
- **THEN** the method returns the 5 answered questions ordered by `occurred_at DESC`; the 2 skipped questions are NOT included

#### Scenario: Empty repo or branch returns nil without query
- **WHEN** `RecentAnswered` is called with `repo = ""` or `branch = ""`
- **THEN** the method returns `(nil, nil)` without executing the query

---

### Requirement: RecentSkipped returns skipped questions scoped to repo and branch
`(*Store).RecentSkipped(ctx, repo, branch, limit)` SHALL return up to `limit` questions where `status = 'skipped' AND repo = ? AND branch = ?`, ordered by the `occurred_at` of the `'skipped'` event descending. Both `repo` and `branch` MUST be non-empty. If either is empty, the method SHALL return `(nil, nil)` without executing the query. New method — replaces the prior behavior where `RecentAnswered` returned both answered and skipped questions indistinguishably.

#### Scenario: Recent skipped questions for repo and branch excludes answers
- **WHEN** `RecentSkipped(ctx, "/path/X", "feature-X", 10)` is called, 2 skipped questions exist for that repo AND branch, and 5 answered questions also exist
- **THEN** the method returns the 2 skipped questions ordered by `occurred_at DESC`; the 5 answered questions are NOT included

#### Scenario: Empty repo or branch returns nil without query
- **WHEN** `RecentSkipped` is called with `repo = ""` or `branch = ""`
- **THEN** the method returns `(nil, nil)` without executing the query

---

### Requirement: RecentQuestions returns all terminal-state questions scoped to repo and branch
`(*Store).RecentQuestions(ctx, repo, branch, limit)` SHALL return up to `limit` questions where `status IN ('answered','skipped') AND repo = ? AND branch = ?`, ordered by the `occurred_at` of the most recent terminal event descending. Both `repo` and `branch` MUST be non-empty. If either is empty, the method SHALL return `(nil, nil)` without executing the query. Enables full-history rendering (both answered and skipped) without the prior design's inability to distinguish them.

#### Scenario: Recent questions for repo and branch includes both answered and skipped
- **WHEN** `RecentQuestions(ctx, "/path/X", "feature-X", 10)` is called, 5 answered questions and 2 skipped questions exist for that repo AND branch
- **THEN** the method returns 7 questions ordered by terminal-event `occurred_at DESC`; each row's `status` column distinguishes answered from skipped

#### Scenario: Empty repo or branch returns nil without query
- **WHEN** `RecentQuestions` is called with `repo = ""` or `branch = ""`
- **THEN** the method returns `(nil, nil)` without executing the query

---

### Requirement: StalePerBranch returns per-branch counts of queued questions
`(*Store).StalePerBranch(ctx, repo)` SHALL return a map of branch name → count of questions where `status = 'queued' AND repo = ?`, grouped by branch. Repo is required; an empty value returns an empty map. Used by `GET /recall/stale` to back the `tr status` advisory. Branches with zero queued questions are omitted.

#### Scenario: Three branches with queued questions
- **WHEN** `StalePerBranch(ctx, "/path/X")` is called and `/path/X` has 3 queued questions on `feature-X`, 0 on `main`, and 2 on `bugfix-Y`
- **THEN** the returned map is `{"feature-X":3,"bugfix-Y":2}` (main omitted, count 0)

#### Scenario: No queued questions on any branch
- **WHEN** `StalePerBranch(ctx, "/path/X")` is called and no questions for `/path/X` have `status = 'queued'`
- **THEN** the returned map is `{}` (empty)

#### Scenario: Empty repo returns empty map without query
- **WHEN** `StalePerBranch(ctx, "")` is called
- **THEN** the method returns an empty map without executing the query

## REMOVED Requirements

### Requirement: AnswerQuestion and SkipQuestion remain ID-keyed
**Reason**: The single `AnswerQuestion(ctx, id, answerIndex, answerText, correct, feedback)` method conflated the user's pick (positional `answerIndex`), the engine's key (computed `correct`), and denormalized display fields (`answerText`, `feedback`) on one row. The refactor splits these into: `SubmitSelection(ctx, questionID, selectedChoiceIDs)` (selections rows + 'answered' event, atomic), `SkipQuestion(ctx, id)` (status + 'skipped' event, atomic), and the `choices.is_correct` row property as the engine's key source of truth. The `feedback` column stays on `questions` as before; it's no longer set inside the same call as the user's pick — the caller composes feedback separately and persists it via a new `SetFeedback` (or merges into `SubmitSelection`) flow.
**Migration**: The teardown SQL drops the prior `questions` table; existing answers are not preserved. New code uses `SubmitSelection` and `SkipQuestion` (renamed signature).

### Requirement: RecentAnswered returns answered questions scoped to repo and branch (single-method form)
**Reason**: The single `RecentAnswered` method returned both answered AND skipped questions (filtered only on `answered_at IS NOT NULL`), indistinguishable in the response without inspecting the `answer` text column for the `'skip'` sentinel. The refactor splits into `RecentAnswered` (status='answered' only), `RecentSkipped` (status='skipped' only), and `RecentQuestions` (both, with `status` field distinguishing them).
**Migration**: Three new queries replace the one; consumers (notably the `recall://recent` MCP resource handler) select the semantic they want. The single `RecentAnswered` form is superseded — see MODIFIED section above for the new `RecentAnswered` definition.