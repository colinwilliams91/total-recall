## ADDED Requirements

### Requirement: question_events table records lifecycle transitions as rows
`store.Open()` SHALL create a `question_events` table with columns: `id INTEGER PRIMARY KEY AUTOINCREMENT`, `question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE`, `event_type TEXT NOT NULL`, `occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`, `actor TEXT`, `payload TEXT` (nullable JSON for future extension). A covering partial index `idx_qe_qid_time ON question_events(question_id, occurred_at DESC)` SHALL be created idempotently. Every lifecycle transition on a question SHALL insert exactly one row into `question_events` in the same transaction that updates the question's `status` cache column.

#### Scenario: Fresh install creates question_events with the covering index
- **WHEN** `Open()` is called against a fresh data directory
- **THEN** `question_events` exists with `question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE`, `event_type TEXT NOT NULL`, `occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`, `actor TEXT`, `payload TEXT`, and the `idx_qe_qid_time` partial index exists

#### Scenario: Idempotent schema creation across repeated opens
- **WHEN** `Open()` is called twice in succession against the same `memory.db`
- **THEN** the second call's `CREATE TABLE IF NOT EXISTS` is a no-op; `question_events` and `idx_qe_qid_time` both remain

---

### Requirement: event_type is CHECK-constrained to a controlled vocabulary
The `question_events.event_type` column SHALL carry a CHECK constraint enforcing membership in `('queued','delivered','answered','skipped')`. Future lifecycle events (e.g., `'claimed'`, `'graded'`, `'resurfaced'`) SHALL be added to the CHECK constraint when those features land — no other schema migration is needed for new event types.

#### Scenario: Inserting an unknown event_type fails
- **WHEN** an `INSERT INTO question_events (question_id, event_type, ...) VALUES (?, 'graded', ...)` is attempted before `'graded'` is added to the CHECK
- **THEN** the insert fails the CHECK constraint and the transaction rolls back

#### Scenario: Adding a new event_type is an additive schema change
- **WHEN** a future change adds `'graded'` to the CHECK constraint values
- **THEN** existing rows are unaffected; no data migration is required

---

### Requirement: The 'queued' event SHALL be inserted atomically with the question row
When a new question is persisted, `(*Store).SaveQuestion` SHALL, in a single transaction, insert the `questions` row with `status = 'queued'` and insert a `question_events` row with `event_type = 'queued'`, `question_id` set to the new question's ID, and `actor` set to `'system'`. Both inserts succeed or both roll back.

#### Scenario: Successful SaveQuestion inserts a 'queued' event
- **WHEN** `SaveQuestion` is called with valid `repo = "/path/X"`, `branch = "feature-X"` and a non-nil question
- **THEN** a `questions` row with `status = 'queued'` exists AND a `question_events` row with `event_type = 'queued'`, `question_id` set to the new question's ID, and `actor = 'system'` exists, both from the same transaction

#### Scenario: SaveQuestion failure rolls back the event row
- **WHEN** the `question_events` insert fails mid-transaction
- **THEN** the `questions` insert is rolled back; no orphaned `questions` row exists without an event row

---

### Requirement: The 'delivered' event SHALL be inserted atomically with the status transition
When `NextQuestion` claims a question for delivery, it SHALL, in a single transaction, update the `questions` row to `status = 'delivered'` AND insert a `question_events` row with `event_type = 'delivered'`, `question_id` set to the claimed question's ID, and `actor` set to the provided `claimedBy` parameter. The atomic claim SHALL preserve exactly-once delivery — concurrent callers receive distinct outcomes (one gets the question, others get `nil`).

#### Scenario: Successful NextQuestion inserts a 'delivered' event
- **WHEN** `NextQuestion(ctx, "/path/X", "feature-X", "shell")` claims a pending question
- **THEN** the `questions` row has `status = 'delivered'` AND a `question_events` row with `event_type = 'delivered'` and `actor = 'shell'` exists, both from the same transaction

#### Scenario: Concurrent callers see consistent claim semantics
- **WHEN** two callers invoke `NextQuestion(ctx, "/path/X", "feature-X", "shell")` simultaneously with one question queued for that repo+branch
- **THEN** exactly one caller receives the question (and a `'delivered'` event is inserted); the other receives `(nil, nil)` with no `'delivered'` event

---

### Requirement: The 'answered' event SHALL be inserted atomically with the selections rows
When the user submits a selection, `(*Store).SubmitSelection` SHALL, in a single transaction, insert one or more `selections` rows, update the `questions` row to `status = 'answered'`, AND insert a `question_events` row with `event_type = 'answered'`. All three operations are atomic. For `multiple_choice` (single-select) exactly one `selections` row is inserted per submit; for `multi_select` (when it lands) `N` rows are inserted per submit. "When was selection X made?" is derived from the parent question's `'answered'` event — `selections` itself carries no timestamp.

#### Scenario: Successful single-select MC submission
- **WHEN** `SubmitSelection` is called with a valid choice ID for a `multiple_choice` question
- **THEN** exactly one `selections` row exists, the `questions` row has `status = 'answered'`, and a `question_events` row with `event_type = 'answered'` exists, all in one transaction

#### Scenario: SubmitSelection failure rolls back selections and event
- **WHEN** the `question_events` insert fails mid-transaction during a submit
- **THEN** the previously inserted `selections` rows are rolled back AND `questions.status` is not updated

#### Scenario: Selecting the same choice twice for one question is refused
- **WHEN** a second submit arrives with the same `(question_id, choice_id)` pair for a question whose `status` is already `'answered'`
- **THEN** the submit is rejected — no duplicate `selections` row is created (enforced by the `UNIQUE(question_id, choice_id)` constraint and the `status` guard)

---

### Requirement: The 'skipped' event SHALL be inserted atomically with the status transition
When the user skips a question, `(*Store).SkipQuestion(ctx, id)` SHALL, in a single transaction, update the `questions` row to `status = 'skipped'` AND insert a `question_events` row with `event_type = 'skipped'`. No `selections` rows are inserted for a skip. "The user selected nothing" is encoded by `status = 'skipped'`, NOT by derivation from absent `selections` rows — the status column is the source of truth for the skip, dissolving the prior `answer = 'skip'` sentinel string.

#### Scenario: Successful skip submission
- **WHEN** `SkipQuestion(ctx, id)` is called for an existing question
- **THEN** the `questions` row has `status = 'skipped'` AND a `question_events` row with `event_type = 'skipped'` exists, both in one transaction; no `selections` rows exist for that question

#### Scenario: Skip and submit are mutually exclusive terminal states
- **WHEN** a question has `status = 'skipped'` and a subsequent `SubmitSelection` is attempted for the same question
- **THEN** the submit is rejected — the question is already in a terminal state

---

### Requirement: question_events is the source of truth; questions.status is a denormalized cache
The most recent `question_events` row for a question determines its true status; `questions.status` SHALL be updated atomically with each event insert in the same transaction. Application code SHALL filter on `questions.status` for performance (single-column `WHERE` predicates); audit and lifecycle queries SHALL read from `question_events`. The CHECK constraint on `questions.status` SHALL mirror the CHECK on `question_events.event_type` for the four shipped values (`queued`/`delivered`/`answered`/`skipped`).

#### Scenario: Filtering pending questions uses the status cache column
- **WHEN** the application queries for pending questions via `WHERE status = 'queued'`
- **THEN** the query returns the same set of questions as a subquery against `question_events` for the latest per-question `event_type = 'queued'`, but without a JOIN

#### Scenario: Audit history reads from question_events
- **WHEN** the application asks "what lifecycle events has question #42 experienced?"
- **THEN** the query reads `SELECT event_type, occurred_at, actor, payload FROM question_events WHERE question_id = 42 ORDER BY occurred_at ASC` and returns every transition row