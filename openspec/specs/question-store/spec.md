## Requirements

### Requirement: DB path is ~/.tr/memory.db (no legacy migration)
`store.Open()` SHALL open `~/.tr/memory.db` (or `$TR_HOME/memory.db` when `TR_HOME` is set). The legacy `concepts.db` → `memory.db` migration guard is removed — only `memory.db` is supported. If `memory.db` does not exist, `Open()` creates it with the full schema.

#### Scenario: Fresh install
- **WHEN** neither `memory.db` nor `concepts.db` exist in the data directory
- **THEN** `Open()` creates `memory.db` with the full schema and returns without error

#### Scenario: Existing memory.db
- **WHEN** `memory.db` already exists
- **THEN** `Open()` opens it directly without attempting any migration

---

### Requirement: questions table schema is created idempotently with repo column
`store.Open()` SHALL run `CREATE TABLE IF NOT EXISTS questions (...)` including a `repo TEXT NOT NULL DEFAULT ''` column on every open. A covering partial index `idx_questions_repo_q ON questions(repo, queued_at ASC) WHERE delivered_at IS NULL` SHALL be created idempotently to support repo-scoped atomic dequeue.

---

### Requirement: SaveQuestion persists a synthesized question with repo tag
`(*Store).SaveQuestion(ctx, repo, question, choices, correctIndex)` SHALL insert one row into `questions` with the `repo` column set to the provided repository path. `delivered_at`, `answer`, and `answered_at` SHALL be NULL.

#### Scenario: Valid question saved for a repo
- **WHEN** `SaveQuestion` is called with `repo = "/path/to/repo"` and a non-nil question
- **THEN** a row appears in `questions` with `repo = "/path/to/repo"`, `delivered_at IS NULL`, and `queued_at` set to the current time

---

### Requirement: NextQuestion atomically claims one question scoped to repo
`(*Store).NextQuestion(ctx, repo, claimedBy)` SHALL use a single SQL `UPDATE ... RETURNING` statement to atomically select and mark the oldest unclaimed question where `delivered_at IS NULL AND repo = ?`, ordered by `queued_at ASC`. It SHALL set `delivered_at = datetime('now')` and `claimed_by` to the provided caller identifier. When `repo = ""`, it SHALL dequeue from the global pool.

#### Scenario: Question available for the repo
- **WHEN** at least one question with `repo = "/path/X"` has `delivered_at IS NULL`
- **THEN** exactly one caller receives that question; concurrent callers receive `nil, nil`

#### Scenario: Queue empty for the repo
- **WHEN** no questions with `repo = "/path/X"` have `delivered_at IS NULL`
- **THEN** `NextQuestion` returns `nil, nil` without error

#### Scenario: Repo isolation — X's question not served to Y
- **WHEN** `NextQuestion(ctx, "/path/Y", "shell")` is called and only questions with `repo = "/path/X"` are pending
- **THEN** `NextQuestion` returns `nil, nil` — repo Y does not receive repo X's questions

---

### Requirement: QueueDepth returns unclaimed count scoped to repo
`(*Store).QueueDepth(ctx, repo)` SHALL return `SELECT COUNT(*) FROM questions WHERE delivered_at IS NULL AND repo = ?`. When `repo = ""`, it SHALL count the global pool.

---

### Requirement: PeekNextQuestion returns the next unclaimed question scoped to repo
`(*Store).PeekNextQuestion(ctx, repo)` SHALL return the oldest unclaimed question where `delivered_at IS NULL AND repo = ?` without claiming it. Used by the `recall://queue` resource handler.

---

### Requirement: RecentAnswered returns answered questions scoped to repo
`(*Store).RecentAnswered(ctx, repo, limit)` SHALL return up to `limit` answered questions where `answered_at IS NOT NULL AND repo = ?`, ordered by `answered_at DESC`. When `repo = ""`, it SHALL return the global answered history.

---

### Requirement: AnswerQuestion and SkipQuestion remain ID-keyed
`(*Store).AnswerQuestion` and `(*Store).SkipQuestion` SHALL act on the row with the given ID regardless of repo. The AUTOINCREMENT primary key is globally unique across repos, so no `repo` parameter is needed for these operations.

#### Scenario: Answer recorded
- **WHEN** `AnswerQuestion(ctx, id, answerIndex, answerText, correct, feedback)` is called
- **THEN** the row with that ID is updated regardless of its `repo` value

#### Scenario: Skip recorded
- **WHEN** `SkipQuestion(ctx, id)` is called
- **THEN** the row with that ID has `answer = "skip"` and `answered_at` set, regardless of its `repo` value

---

### Requirement: Repo column migration purges un-tagged legacy questions
When `store.Open()` adds the `repo` column to an existing `questions` table (idempotent `ALTER TABLE ADD COLUMN`), it SHALL subsequently `DELETE FROM questions` to purge rows that have no repo tag. This purge SHALL run exactly once — gated on the column-add. The purge SHALL be logged.

#### Scenario: Existing database upgraded
- **WHEN** `Open()` is called on a `memory.db` that has `questions` rows but no `repo` column
- **THEN** the `repo TEXT NOT NULL DEFAULT ''` column is added, all existing question rows are deleted, and the purge is logged
