# Question storage schema: choices / selections / question_events

The pre-refactor `questions` table was a single-row accumulator for question content, correctness key, user pick, and lifecycle state, with a positional index into a JSON array as the correctness key and the word "answer" meaning both the user's pick and the engine's key. Roadmap signals (multi-select, adaptive resurfacing) needed multiple selections per question and lifecycle events without schema churn. We rebuilt the schema as four tables: `questions` (pure content + a denormalized `status` enum), `choices` (with `is_correct` as the Answer Key, replacing the positional index), `selections` (a join table for user picks, one row per pick), and `question_events` (an event log that is the source of truth for lifecycle; `status` is an atomically-updated fast-filter cache). The wire contract broke atomically with the schema — user side renamed to select/selected, engine side retains answer/correct — since the repo is pre-release with zero live users and a hard break was cheap now and expensive later.

Decisions folded in: (1) the Answer Key lives entirely on Choice rows — the polymorphic alternative (one FK + one text column on `questions`) was rejected because a single FK cannot model multi-select's multiple correct rows; (2) `selections` carries no per-pick timestamp — the parent's `answered` event is the time record, first-attempt-only; (3) `status` is a denormalized cache coupled to the event log in the same transaction, not pure derivation; (4) the migration is a maintainer-run teardown, no `PRAGMA user_version` machinery; (5) the submit-response sends both `correct_index` and `correct_answer` on the wire — wire redundancy is acceptable (computed once, no drift risk) where DB redundancy is not; (6) shuffle stays in recall as presentation logic.

Full design, rejected alternatives, and implementation tasks: [openspec change archive](../openspec/changes/archive/2026-08-03-refactor-question-storage-schema/design.md).

## Consequences

- `questions.correct_answer` is reserved storage only — nothing writes it today; the key always lives on `choices.is_correct`.
- Retries/multi-attempt selections would need a symmetric `selection_events` sibling; deliberately not built speculatively.
- The event-type CHECK leaves room for future types (`claimed`/`graded`/`resurfaced`) without `ALTER TABLE`.
- Delivery leaks no Answer Key: correct indices are withheld at delivery, sent only in the submit-response.
