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
`store.Open()` SHALL run `CREATE TABLE IF NOT EXISTS questions (...)` on every open. This is safe to run on both freshly created and migrated databases.

---

### Requirement: SaveQuestion persists a synthesized question
`(*Store).SaveQuestion` SHALL insert one row into `questions` with the question text and JSON-marshalled choices. `delivered_at`, `answer`, and `answered_at` SHALL be NULL.

#### Scenario: Valid question saved
- **WHEN** `SaveQuestion` is called with a non-nil `*recall.Question`
- **THEN** a row appears in `questions` with `delivered_at IS NULL` and `queued_at` set to the current time

---

### Requirement: NextQuestion atomically claims one question
`(*Store).NextQuestion` SHALL use a single SQL `UPDATE ... RETURNING` statement to atomically select and mark the oldest unclaimed question (`delivered_at IS NULL`, ordered by `queued_at ASC`). It SHALL set `delivered_at = datetime('now')` and `claimed_by` to the provided caller identifier.

#### Scenario: Question available
- **WHEN** at least one question has `delivered_at IS NULL`
- **THEN** exactly one caller receives the question; concurrent callers receive `nil, nil`

#### Scenario: Queue empty
- **WHEN** no questions have `delivered_at IS NULL`
- **THEN** `NextQuestion` returns `nil, nil` without error

---

### Requirement: AnswerQuestion records the user's response
`(*Store).AnswerQuestion` SHALL update the row with the given ID, setting `answer` to the provided string and `answered_at = datetime('now')`. The string `"skip"` is a valid answer value.

#### Scenario: Answer recorded
- **WHEN** `AnswerQuestion(ctx, id, "Prevent retry synchronization")` is called
- **THEN** the row has `answer = "Prevent retry synchronization"` and `answered_at` is non-null

#### Scenario: Skip recorded
- **WHEN** `AnswerQuestion(ctx, id, "skip")` is called
- **THEN** the row has `answer = "skip"` and `answered_at` is non-null
