# MIGRATION: schema reshape

This change reshapes the `questions` table and adds `choices`, `selections`,
and `question_events` tables. The teardown is intentionally destructive: the
prior schema (`correct_index`, `answer_index`, `answer`, `correct`,
`delivered_at`, `answered_at`, `queued_at`, `claimed_by`, `feedback` as
in-row columns, `choices` as JSON) is gone. There is no incremental migration
path.

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
