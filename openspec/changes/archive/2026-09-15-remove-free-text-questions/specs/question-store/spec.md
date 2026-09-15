## MODIFIED Requirements

### Requirement: questions table schema is created idempotently with repo, branch, status, and question_type columns
`store.Open()` SHALL run `CREATE TABLE IF NOT EXISTS questions (...)` with columns: `id INTEGER PRIMARY KEY AUTOINCREMENT`, `question_type TEXT NOT NULL DEFAULT 'multiple_choice'` with CHECK constraint enforcing `('multiple_choice','multi_select')`, `status TEXT NOT NULL DEFAULT 'queued'` with CHECK constraint enforcing `('queued','delivered','answered','skipped')`, `question TEXT NOT NULL`, `repo TEXT NOT NULL`, `branch TEXT NOT NULL`, and `feedback TEXT` (nullable). The `questions` table SHALL NOT contain `queued_at`, `delivered_at`, `claimed_by`, `answer`, `answer_index`, `correct_index`, `correct`, `correct_answer`, or `answered_at` columns — those concepts move to `question_events`, `selections`, and `choices`, the `status` cache column, or cease to exist with the removed free-text format. Because `CREATE TABLE IF NOT EXISTS` cannot reshape an existing table, `memory.db` files created by a prior build MUST be torn down per `internal/cache/MIGRATION.md` before running the new build.

#### Scenario: Idempotent schema creation across repeated opens
- **WHEN** `Open()` is called twice in succession against the same `memory.db` file
- **THEN** the second call's `CREATE TABLE IF NOT EXISTS` is a no-op; `questions` has `repo TEXT NOT NULL`, `branch TEXT NOT NULL`, `question_type TEXT NOT NULL DEFAULT 'multiple_choice'`, `status TEXT NOT NULL DEFAULT 'queued'` (both with their CHECK constraints), and `feedback TEXT` (nullable)

#### Scenario: Inserting an unknown question_type fails
- **WHEN** an `INSERT INTO questions (question_type, ...)` is attempted with `question_type = 'poll'`
- **THEN** the insert fails the CHECK constraint and the transaction rolls back

#### Scenario: Inserting the removed free_text question_type fails
- **WHEN** an `INSERT INTO questions (question_type, ...)` is attempted with `question_type = 'free_text'` against a store created by the new schema
- **THEN** the insert fails the CHECK constraint and the transaction rolls back

#### Scenario: Inserting an unknown status fails
- **WHEN** a question row is updated with `status = 'pending'`
- **THEN** the update fails the CHECK constraint and the transaction rolls back

---

### Requirement: choices table stores one row per choice with the engine's correctness key as a row property
`store.Open()` SHALL create a `choices` table with columns: `id INTEGER PRIMARY KEY AUTOINCREMENT`, `question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE`, `position INTEGER` (display order, produced by the recall shuffle), `text TEXT NOT NULL`, `is_correct INTEGER NOT NULL DEFAULT 0` (the engine's correctness key — `1` for the correct choice, `0` for distractors). A question with `question_type = 'multiple_choice'` (single-select) SHALL have exactly one row with `is_correct = 1` and at least one row with `is_correct = 0`. A covering index `idx_choices_qid ON choices(question_id)` SHALL be created idempotently. The correctness key survives JSON reorder/truncate because it is a row property, not a positional reference into a serialized array — the choice row is the only correctness-key channel in the system.

#### Scenario: Multiple-choice question has exactly one is_correct=1 row
- **WHEN** a `multiple_choice` question is saved with 4 choices (one correct, three distractors) at positions [0,1,2,3]
- **THEN** exactly 4 `choices` rows exist for that question; exactly one row has `is_correct = 1` (the correct choice); the other 3 have `is_correct = 0`; the correct choice's `position` is whatever the recall shuffle assigned

#### Scenario: Free-text question has zero choices rows
- **WHEN** a `question_type = 'free_text'` insert is attempted against a store created by the new schema
- **THEN** the insert fails the CHECK constraint and rolls back — no question without `choices` rows can exist, and the correctness key always lives on `choices` rows

#### Scenario: Free-text does not require correct_answer to be populated at save time
- **WHEN** a `memory.db` created by the new schema is inspected
- **THEN** the `questions` table has no `correct_answer` column — the free-text correctness-key channel does not exist

---

### Requirement: GetQuestion fetches a question joined with its choices
`(*Store).GetQuestion(ctx, id)` SHALL return the `questions` row joined with its `choices` rows (in position order). ID-keyed (globally unique) — no `repo` or `branch` parameter needed. Returns `(nil, nil)` when the row does not exist. The returned `StoredQuestion` carries the `choices` as a `[]Choice` with `Text` and `IsCorrect` flags and a derived `CorrectIndex` (the position of the `is_correct = 1` row), but the correctness key lives on the choice row itself — the `CorrectIndex` is a presentation aid for clients, not the source of truth. The `StoredQuestion` SHALL NOT carry a free-text answer key field.

#### Scenario: Existing MC question returned with choices
- **WHEN** `GetQuestion(ctx, 42)` is called for an existing `multiple_choice` question with 4 choices
- **THEN** the returned `StoredQuestion` has the question text, the 4 choices ordered by `position`, and a non-nil `CorrectIndex` pointing at the `is_correct = 1` choice's position

#### Scenario: Unknown question ID returns nil
- **WHEN** `GetQuestion(ctx, 99999)` is called
- **THEN** the method returns `(nil, nil)` without error
