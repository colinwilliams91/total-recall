# MIGRATION: schema reshape

This change reshapes the `questions` table. The teardown is intentionally
destructive and applies to any `memory.db` created by an earlier build:

- First reshape (prior): dropped `correct_index`, `answer_index`, `answer`,
  `correct`, `delivered_at`, `answered_at`, `queued_at`, `claimed_by`,
  `feedback` as in-row columns, and `choices` as JSON; added `choices`,
  `selections`, and `question_events` tables.
- Second reshape (current): drops the free-text correctness-key column
  `correct_answer` and narrows the `question_type` CHECK constraint to
  `('multiple_choice','multi_select')` (removing `'free_text'`).

There is no incremental migration path. A stale `questions` table that still
carries `correct_answer` and the `'free_text'` CHECK value must be torn down:
the new build's narrowed CHECK cannot exist in it, and inserts the new build
wants to reject would still succeed there.

Before running the new build for the first time, run ONE of:

```sql
DROP TABLE IF EXISTS questions;
```

Or simply delete the data file:

```
rm ~/.tr/memory.db   # or wherever TR_HOME points
```

`cache.Open()` runs `CREATE TABLE IF NOT EXISTS` for the new schema (four
tables: `questions` reshaped, `choices`, `question_events`, `selections`).
On an existing `memory.db` with the prior schema, the `CREATE TABLE IF NOT
EXISTS questions` is a no-op (the prior table already exists), the new
tables are not created, and the application will misbehave. Hence the
teardown requirement.
