# MIGRATION: schema reshape

This change reshapes the `concepts` table into the two-layer identity model
and adds three new tables. The teardown is intentionally destructive and
applies to any `memory.db` created by an earlier build:

- First reshape (prior): dropped `correct_index`, `answer_index`, `answer`,
  `correct`, `delivered_at`, `answered_at`, `queued_at`, `claimed_by`,
  `feedback` as in-row columns, and `choices` as JSON; added `choices`,
  `selections`, and `question_events` tables.
- Second reshape (prior): dropped the free-text correctness-key column
  `correct_answer` and narrowed the `question_type` CHECK constraint to
  `('multiple_choice','multi_select')` (removing `'free_text'`).
- Third reshape (current): two-layer concept identity model. `concepts`
  becomes the repo/branch-scoped sightings table — it gains a `slug` column
  FK'd to the new `concept_registry` table (user-scoped canonical identity;
  `first_seen` minted there). Also added: `concept_aliases` (exact
  alias → target pressure valve) and `question_concepts` (provenance join
  written with each saved question).

There is no incremental migration path. A stale `concepts` table that still
carries the prose-only shape (no `slug` column) must be torn down: the new
build's sightings inserts reference `concepts.slug`, which cannot exist in
the stale table, so every pipeline save to an existing DB fails loudly. In
general, `cache.Open()`'s `CREATE TABLE IF NOT EXISTS` is a no-op against an
existing table — stale schema + `IF NOT EXISTS` = silent half-migration.

Before running the new build for the first time, delete the data file:

```
rm ~/.tr/memory.db   # or wherever TR_HOME points
```

`cache.Open()` runs `CREATE TABLE IF NOT EXISTS` for the new schema
(`concept_registry`, `concepts` reshaped, `concept_aliases`, `questions`
reshaped, `choices`, `question_events`, `selections`, `question_concepts`).
On an existing `memory.db` with the prior schema, the `CREATE TABLE IF NOT
EXISTS concepts` is a no-op (the prior table already exists), the new
tables are created but unlinkable, and the application will misbehave.
Hence the teardown requirement.
