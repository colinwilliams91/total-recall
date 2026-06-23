## Requirements

### Requirement: DB path migrates from concepts.db to memory.db
`store.Open()` SHALL check for `~/.tr/memory.db`. If it does not exist but `~/.tr/concepts.db` does, it SHALL copy `concepts.db` → `memory.db` before opening, logging a one-time advisory. If neither exists, `Open()` creates `memory.db` from scratch.

#### Scenario: Fresh install (no legacy file)
- **WHEN** neither `memory.db` nor `concepts.db` exist in `~/.tr/`
- **THEN** `Open()` creates `~/.tr/memory.db` with the full schema and returns without error

#### Scenario: Existing Phase 3 install
- **WHEN** `~/.tr/concepts.db` exists and `~/.tr/memory.db` does not
- **THEN** `Open()` copies `concepts.db` → `memory.db`, logs `[store] migrated concepts.db → memory.db`, and opens the new file

#### Scenario: Already migrated
- **WHEN** `~/.tr/memory.db` already exists (regardless of `concepts.db`)
- **THEN** `Open()` opens `memory.db` directly without attempting migration

---

### Requirement: questions table schema is created idempotently
`store.Open()` SHALL run `CREATE TABLE IF NOT EXISTS questions (...)` on every open. The full schema includes: `id`, `question`, `choices`, `correct_index` (INTEGER NOT NULL DEFAULT 0), `queued_at`, `delivered_at`, `claimed_by`, `answer`, `answer_index` (INTEGER, nullable), `correct` (INTEGER, nullable), `feedback` (TEXT, nullable), `answered_at`.

#### Scenario: Fresh install — full schema
- **WHEN** `memory.db` does not exist
- **THEN** `Open()` creates the `questions` table with all columns including `correct_index`, `answer_index`, `correct`, `feedback`

---

### Requirement: addColumnIfMissing migrates existing installs idempotently
After running `CREATE TABLE IF NOT EXISTS`, `store.Open()` SHALL call `addColumnIfMissing` for each Phase 4C column (`correct_index`, `answer_index`, `correct`, `feedback`). The helper executes `ALTER TABLE ADD COLUMN` and ignores "duplicate column name" errors, propagating all other errors.

#### Scenario: Existing Phase 4A install — columns added
- **WHEN** `memory.db` exists with the Phase 4A questions table (no `correct_index`, `answer_index`, `correct`, `feedback` columns)
- **THEN** each missing column is added via `ALTER TABLE ADD COLUMN`
- **AND** existing rows remain intact with `correct_index = 0`, `answer_index = NULL`, `correct = NULL`, `feedback = NULL`

#### Scenario: Idempotent re-run
- **WHEN** `store.Open()` is called on a database that already has all columns
- **THEN** no error is returned (duplicate column errors are silently ignored)

---

### Requirement: SaveQuestion persists a synthesized question with correct_index
`(*Store).SaveQuestion(ctx, question string, choices []string, correctIndex int) error` SHALL insert one row into `questions` with the question text, JSON-marshalled choices, and `correct_index`. `delivered_at`, `answer`, `answer_index`, `correct`, `feedback`, and `answered_at` SHALL be NULL.

#### Scenario: Valid question saved with correct_index
- **WHEN** `SaveQuestion(ctx, q.Question, q.Choices, q.CorrectIndex)` is called with `CorrectIndex = 2`
- **THEN** a row appears in `questions` with `correct_index = 2`, `delivered_at IS NULL`, and `queued_at` set to the current time

---

### Requirement: NextQuestion atomically claims one question and returns correct_index
`(*Store).NextQuestion` SHALL use a single SQL `UPDATE ... RETURNING` statement to atomically select and mark the oldest unclaimed question (`delivered_at IS NULL`, ordered by `queued_at ASC`). It SHALL set `delivered_at = datetime('now')` and `claimed_by` to the provided caller identifier. The RETURNING clause SHALL include `correct_index` so the caller (REST or MCP) has access to it.

#### Scenario: Question available
- **WHEN** at least one question has `delivered_at IS NULL`
- **THEN** exactly one caller receives the question (including `CorrectIndex`); concurrent callers receive `nil, nil`

#### Scenario: Queue empty
- **WHEN** no questions have `delivered_at IS NULL`
- **THEN** `NextQuestion` returns `nil, nil` without error

---

### Requirement: GetQuestion fetches a single question by ID
`(*Store).GetQuestion(ctx, id int64) (*StoredQuestion, error)` SHALL `SELECT id, question, choices, correct_index, queued_at FROM questions WHERE id = ?`. It SHALL return `nil, nil` when no row with the given ID exists.

#### Scenario: Question exists
- **WHEN** `GetQuestion(ctx, 1)` is called and question 1 exists
- **THEN** a `*StoredQuestion` is returned with `ID`, `Question`, `Choices`, `CorrectIndex`, and `QueuedAt` populated

#### Scenario: Question not found
- **WHEN** `GetQuestion(ctx, 99999)` is called and no row with ID 99999 exists
- **THEN** `nil, nil` is returned (not an error)

---

### Requirement: AnswerQuestion records all answer fields
`(*Store).AnswerQuestion(ctx, id int64, answerIndex int, answerText string, correct bool, feedback string) error` SHALL update the row with the given ID, setting `answer`, `answer_index`, `correct` (1 for true, 0 for false), `feedback` (NULL if empty string), and `answered_at = datetime('now')`.

#### Scenario: Answer with feedback (terminal path)
- **WHEN** `AnswerQuestion(ctx, 1, 1, "B", false, "A is correct because...")` is called
- **THEN** the row has `answer = "B"`, `answer_index = 1`, `correct = 0`, `feedback = "A is correct because..."`, `answered_at = now()`

#### Scenario: Answer with empty feedback (MCP path)
- **WHEN** `AnswerQuestion(ctx, 1, 0, "A", true, "")` is called
- **THEN** the row has `answer = "A"`, `answer_index = 0`, `correct = 1`, `feedback = NULL`, `answered_at = now()`

---

### Requirement: SkipQuestion records a skip with no evaluation
`(*Store).SkipQuestion(ctx, id int64) error` SHALL `UPDATE questions SET answer = 'skip', answered_at = datetime('now') WHERE id = ?`. It SHALL leave `answer_index`, `correct`, and `feedback` NULL.

#### Scenario: Skip recorded
- **WHEN** `SkipQuestion(ctx, 1)` is called
- **THEN** the row has `answer = "skip"`, `answered_at = now()`
- **AND** `answer_index`, `correct`, `feedback` remain NULL

---

### Requirement: RecentAnswered returns enriched rows
`(*Store).RecentAnswered` SHALL SELECT and scan `correct_index`, `answer_index`, `correct`, and `feedback` in addition to existing fields. Nullable columns SHALL use nullable scan types (`*int`, `*bool`, `*string`) so that pointer fields are nil when the DB column is NULL.

#### Scenario: Mixed answer history
- **WHEN** three questions have been answered (two correct, one skipped)
- **THEN** all three rows are returned ordered by `answered_at DESC`
- **AND** each row includes `CorrectIndex`, `AnswerIndex` (nil for skip), `Correct` (nil for skip), `Feedback` (nil for skip and MCP rows)

---

### Requirement: StoredQuestion struct carries the full answer lifecycle
`StoredQuestion` SHALL have fields: `ID int64`, `Question string`, `Choices []string`, `CorrectIndex int`, `QueuedAt time.Time`, `AnswerIndex *int`, `Correct *bool`, `Feedback *string`. Pointer fields SHALL be nil when the DB column is NULL.

#### Scenario: Nullable fields on skipped question
- **WHEN** a `StoredQuestion` is scanned from a skipped row
- **THEN** `AnswerIndex`, `Correct`, and `Feedback` are nil
