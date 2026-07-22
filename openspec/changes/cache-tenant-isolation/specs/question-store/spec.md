## MODIFIED Requirements

### Requirement: DB path is ~/.tr/memory.db (no legacy migration)
`store.Open()` SHALL open `~/.tr/memory.db` (or `$TR_HOME/memory.db` when `TR_HOME` is set). The legacy `concepts.db` → `memory.db` migration guard is removed — only `memory.db` is supported. No in-code schema migration (no `ALTER TABLE`, no row purge) SHALL run inside `Open()`. If `memory.db` does not exist, `Open()` creates it with the full schema (`CREATE TABLE IF NOT EXISTS` with all final columns including `repo` and `branch`).

#### Scenario: Fresh install
- **WHEN** neither `memory.db` nor `concepts.db` exist in the data directory
- **THEN** `Open()` creates `memory.db` with the full schema including `repo TEXT NOT NULL` and `branch TEXT NOT NULL` columns on both `concepts` and `questions`, and returns without error

#### Scenario: Existing memory.db with current schema
- **WHEN** `memory.db` already exists with the current schema
- **THEN** `Open()` opens it directly without attempting any migration

#### Scenario: Existing memory.db with stale schema
- **WHEN** `memory.db` exists with the prior schema (missing `branch` column)
- **THEN** `Open()`'s `CREATE TABLE IF NOT EXISTS` is a no-op against the existing tables — the binary does not attempt any in-code `ALTER TABLE` or data purge; the maintainer deletes the stale `memory.db` manually per the design's migration plan

---

### Requirement: questions table schema is created idempotently with repo and branch columns
`store.Open()` SHALL run `CREATE TABLE IF NOT EXISTS questions (...)` including `repo TEXT NOT NULL` and `branch TEXT NOT NULL` columns (no `DEFAULT ''` — every insert must supply both values). A covering partial index `idx_questions_repo_branch_q ON questions(repo, branch, queued_at ASC) WHERE delivered_at IS NULL` SHALL be created idempotently to support repo-AND-branch-scoped atomic dequeue.

#### Scenario: Idempotent schema creation across repeated opens
- **WHEN** `Open()` is called twice in succession against the same `memory.db` file
- **THEN** the second call's `CREATE TABLE IF NOT EXISTS` is a no-op; `questions` has `repo TEXT NOT NULL` and `branch TEXT NOT NULL` columns; and the `idx_questions_repo_branch_q` partial index exists

---

### Requirement: SaveQuestion persists a synthesized question with repo and branch tags
`(*Store).SaveQuestion(ctx, repo, branch, question, choices, correctIndex)` SHALL insert one row into `questions` with the `repo` column and the `branch` column set to the provided values. Both `repo` and `branch` MUST be non-empty. If either is empty, `SaveQuestion` SHALL return early (no-op) and log `[store] skipping savequestion: empty repo or branch`. `delivered_at`, `answer`, and `answered_at` SHALL be NULL on insert.

#### Scenario: Valid question saved for a repo and branch
- **WHEN** `SaveQuestion` is called with `repo = "/path/to/repo"`, `branch = "feature-X"`, and a non-nil question
- **THEN** a row appears in `questions` with `repo = "/path/to/repo"`, `branch = "feature-X"`, `delivered_at IS NULL`, and `queued_at` set to the current time

#### Scenario: Empty repo or branch refuses to save
- **WHEN** `SaveQuestion` is called with either `repo = ""` or `branch = ""`
- **THEN** no row is inserted; the method returns `nil` (no error); the daemon logs `[store] skipping savequestion: empty repo or branch`

---

### Requirement: NextQuestion atomically claims one question scoped to repo and branch
`(*Store).NextQuestion(ctx, repo, branch, claimedBy)` SHALL use a single SQL `UPDATE ... RETURNING` statement to atomically select and mark the oldest unclaimed question where `delivered_at IS NULL AND repo = ? AND branch = ?`, ordered by `queued_at ASC`. It SHALL set `delivered_at = datetime('now')` and `claimed_by` to the provided caller identifier. Both `repo` and `branch` MUST be non-empty. If either is empty, the method SHALL return `(nil, nil)` without executing the query and without error. No "global pool" semantics exist for empty values.

#### Scenario: Question available for the repo and branch
- **WHEN** at least one question with `repo = "/path/X"`, `branch = "feature-X"` has `delivered_at IS NULL`
- **THEN** exactly one caller receives that question; concurrent callers receive `nil, nil`

#### Scenario: Queue empty for the repo and branch
- **WHEN** no questions with `repo = "/path/X"`, `branch = "feature-X"` have `delivered_at IS NULL`
- **THEN** `NextQuestion` returns `nil, nil` without error

#### Scenario: Repo-isolation - X's question not served to Y
- **WHEN** `NextQuestion(ctx, "/path/Y", "feature-X", "shell")` is called and only questions with `repo = "/path/X"` are pending
- **THEN** `NextQuestion` returns `nil, nil` — repo Y does not receive repo X's questions

#### Scenario: Branch-isolation - feature-X's question not served to main
- **WHEN** `NextQuestion(ctx, "/path/X", "main", "shell")` is called and only questions with `branch = "feature-X"` are pending for `/path/X`
- **THEN** `NextQuestion` returns `nil, nil` — `main` does not receive `feature-X`'s questions

---

### Requirement: QueueDepth returns unclaimed count scoped to repo and branch
`(*Store).QueueDepth(ctx, repo, branch)` SHALL return `SELECT COUNT(*) FROM questions WHERE delivered_at IS NULL AND repo = ? AND branch = ?`. Both `repo` and `branch` MUST be non-empty. If either is empty, the method SHALL return `(0, nil)` without executing the query.

#### Scenario: Queue depth for repo and branch
- **WHEN** `QueueDepth(ctx, "/path/X", "feature-X")` is called and 3 unclaimed questions exist for that repo AND branch
- **THEN** the method returns `(3, nil)`

#### Scenario: Empty repo or branch returns zero without query
- **WHEN** `QueueDepth` is called with `repo = ""` or `branch = ""`
- **THEN** the method returns `(0, nil)` without executing the query

---

### Requirement: PeekNextQuestion returns the next unclaimed question scoped to repo and branch
`(*Store).PeekNextQuestion(ctx, repo, branch)` SHALL return the oldest unclaimed question where `delivered_at IS NULL AND repo = ? AND branch = ?` without claiming it. Both `repo` and `branch` MUST be non-empty. If either is empty, the method SHALL return `(nil, nil)` without executing the query. Used by the `recall://queue` resource handler.

#### Scenario: Peek returns oldest unclaimed without claiming
- **WHEN** `PeekNextQuestion(ctx, "/path/X", "feature-X")` is called and an unclaimed question exists for that repo AND branch
- **THEN** the method returns that question without marking it delivered (`delivered_at` remains `NULL`)

#### Scenario: Empty repo or branch returns nil without query
- **WHEN** `PeekNextQuestion` is called with `repo = ""` or `branch = ""`
- **THEN** the method returns `(nil, nil)` without executing the query

---

### Requirement: RecentAnswered returns answered questions scoped to repo and branch
`(*Store).RecentAnswered(ctx, repo, branch, limit)` SHALL return up to `limit` answered questions where `answered_at IS NOT NULL AND repo = ? AND branch = ?`, ordered by `answered_at DESC`. Both `repo` and `branch` MUST be non-empty. If either is empty, the method SHALL return `(nil, nil)` without executing the query.

#### Scenario: Recent answered questions for repo and branch
- **WHEN** `RecentAnswered(ctx, "/path/X", "feature-X", 10)` is called and 5 answered questions exist for that repo AND branch
- **THEN** the method returns all 5, ordered by `answered_at DESC`

#### Scenario: Empty repo or branch returns nil without query
- **WHEN** `RecentAnswered` is called with `repo = ""` or `branch = ""`
- **THEN** the method returns `(nil, nil)` without executing the query

---

### Requirement: AnswerQuestion and SkipQuestion remain ID-keyed
`(*Store).AnswerQuestion` and `(*Store).SkipQuestion` SHALL act on the row with the given ID regardless of `repo` or `branch`. The AUTOINCREMENT primary key is globally unique across repos and branches, so no `repo` or `branch` parameter is needed for these operations.

#### Scenario: Answer recorded
- **WHEN** `AnswerQuestion(ctx, id, answerIndex, answerText, correct, feedback)` is called
- **THEN** the row with that ID is updated regardless of its `repo` or `branch` value

#### Scenario: Skip recorded
- **WHEN** `SkipQuestion(ctx, id)` is called
- **THEN** the row with that ID has `answer = "skip"` and `answered_at` set, regardless of its `repo` or `branch` value

## REMOVED Requirements

### Requirement: Repo column migration purges un-tagged legacy questions
**Reason**: In-code schema migrations have been removed entirely (Decision 2 in `cache-tenant-isolation/design.md`). The `cache.Open()` migration block (`addColumnIfMissing` for `repo` and the subsequent `DELETE FROM questions` purge) is deleted. Fresh databases are created with the full schema including `repo` and `branch` columns.
**Migration**: The maintainer (only user) runs `rm ~/.tr/memory.db` once. The next `tr serve` creates a fresh DB with the new schema.