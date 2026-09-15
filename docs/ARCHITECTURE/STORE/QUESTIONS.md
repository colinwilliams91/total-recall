## Schema

```sql
CREATE TABLE IF NOT EXISTS questions (
    id              INTEGER  PRIMARY KEY AUTOINCREMENT,
    question_type   TEXT     NOT NULL DEFAULT 'multiple_choice',
                    -- CHECK ('multiple_choice','multi_select')
    status          TEXT     NOT NULL DEFAULT 'queued',
                    -- CHECK ('queued','delivered','answered','skipped')
    question        TEXT     NOT NULL,
    repo            TEXT     NOT NULL,
    branch          TEXT     NOT NULL,
    feedback        TEXT                     -- AI explanation, NULL for MCP rows
);

CREATE TABLE IF NOT EXISTS choices (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL,           -- display order (recall shuffle)
    text        TEXT    NOT NULL,
    is_correct  INTEGER NOT NULL DEFAULT 0  -- the Answer Key, per row
);

CREATE TABLE IF NOT EXISTS selections (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    choice_id   INTEGER NOT NULL REFERENCES choices(id)   ON DELETE CASCADE,
    UNIQUE (question_id, choice_id)         -- one row per user pick
);

CREATE TABLE IF NOT EXISTS question_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    event_type  TEXT    NOT NULL,
                -- CHECK ('queued','delivered','answered','skipped')
    occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actor       TEXT,
    payload     TEXT
);
```

## Key properties

- The Answer Key is the `is_correct` flag on each `choices` row — never a
  positional index into a JSON array, never free text on the Question.
- The user's picks live in `selections` as (question, choice) pairs; there is
  no `answer_text`/`answer_index` column. "When was this pick made?" is
  answered by the parent question's `'answered'` event in `question_events`.
- Delivery/claim/skip state is the `status` enum, mirrored by `question_events`
  rows — there are no `queued_at`/`delivered_at`/`claimed_by`/`answered_at`
  datetime columns.
- `repo` and `branch` are NOT NULL on every question row; no cross-repo or
  cross-branch pooling.
- Schema changes are manual teardowns: `CREATE TABLE IF NOT EXISTS` cannot
  reshape an existing table. See `internal/cache/MIGRATION.md`.
