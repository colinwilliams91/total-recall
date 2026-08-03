## Context

The current `questions` table is a single-table god-row: question content, the engine's correctness key, the user's pick, delivery state, claim state, answer state, grading, and feedback all live on one row. The correctness key is a positional integer into a JSON blob (`correct_index`), the user's pick is also a positional integer (`answer_index`), state is encoded implicitly via three overloaded `DATETIME` columns, and the word "answer" is used for both the user's submission and the engine's key. The repository is pre-release with zero live users, so a structural breaking change is cheap now and prohibitively expensive later. Roadmap signals (adaptive resurfacing, multi-select "select all that apply", free-text questions with partial-credit grading) all point toward needing a schema that can model multiple selections per question and audit lifecycle events without `ALTER TABLE` churn.

## Goals / Non-Goals

**Goals:**
- Move correctness key from positional JSON to a row property (`choices.is_correct`). Survives JSON reorder/truncate; self-describing.
- Move user pick from positional integer to a join table (`selections` referencing `choices.id`). Models multi-select as N rows without schema change.
- Introduce a `question_events` audit spine so every lifecycle transition is a row insert. `questions` and `selections` become pure-content tables; `questions.status` is a denormalized cache column for fast filtering.
- Replace the `answer='skip'` magic-string sentinel and the implicit datetime-encoded state machine with an explicit `status` enum column.
- Frontload `question_type` enum so non-MC question types can be discriminated without a later schema migration.
- Fix the delivery leak: `correct_index` (and per-type equivalents) SHALL NOT cross the wire at delivery time, only at submit-response time.
- Establish a single lexical rule: user side uses `select`/`selected` exclusively; engine side may use `answer`/`correct` (engine-inference output).

**Non-Goals:**
- Multi-select presentation/grading logic — schema supports it, implementation deferred (only `multiple_choice` ships).
- Free-text question type presentation/grading — schema supports it, implementation deferred. Scoring (exact-match, rubric, partial-credit) is a later phase.
- Adaptive difficulty / resurfacing — `question_events` is shaped to support it (`resurfaced`/`graded` event types extensible without `ALTER TABLE`), but no algorithm lands here.
- Retry / multiple-attempt support — not a near-term need; selections are first-attempt-only. If retries land, `selection_events` is the natural addition (symmetric sibling to `question_events`), but is not built speculatively.
- Removal of `correct_index` from the wire response — `correct_index` and `correct_answer` are both sent in the submit-response (send-both) to support stateless MCP consumers generating feedback without cross-call state. The "no redundancy" principle applied to DB denormalization (thread 1) does not extend to wire denormalization (no drift risk; response is computed once per submit).
- Per-pick timestamps — `selections` carries no timestamp. Per-pick time IS the parent question's `answered` event time (atomic). Incremental submittal is explicitly out of scope.
- Detection/healing of stale `~/.tr/memory.db` in `Open()`. The maintainer runs a teardown SQL script (or deletes the file) before deploying the new build. No `PRAGMA user_version` probe or in-app migration code.

## Decisions

### Decision: Schema shape — `choices` table + `is_correct` row flag (Option 3)

Resolves the "where does the correctness key live" question by making it a property of a choice row, not a positional reference into a JSON array.

**Alternatives considered:**
- *Option 1 (stable opaque choice IDs in JSON)* — least invasive, additive `ALTER TABLE` only. Rejected: left the JSON bag intact, no FK enforcement, single-row god-row retained.
- *Option 2 (3-table, `selections` separate)* — symmetric to Option 3a-i. Rejected for the *key location* question; 3a-i and 3a-ii differ in selection storage, not key storage.
- *Option 3b (polymorphic `correct_choice_id` FK on questions)* — single "key column" abstraction, secretly polymorphic (FK for MC, `expected_answer` text for free-text). Rejected: loses the per-row self-describing property, and a single FK cannot model multi-select (which has multiple correct rows).

### Decision: Selection storage — `selections` join table (Option 3a-i)

Symmetric M:N relationship between `questions` and `choices`: `choices.is_correct` models the engine's key (M:N), `selections.choice_id` models the user's pick (M:N). Both are overlays on the same set of choice rows, modeled symmetrically.

**Alternatives considered:**
- *Option 3a-ii (fold `selected` column onto `choices`)* — `choices.selected INTEGER`. Single-table grading query, no JOIN. Rejected: conflates two roles (engine key vs. user pick) onto one table; the "self-describing" property lives on rows that have both `is_correct` and `selected` columns, which is awkward when a choice is neither (before the user picks). Loses clean symmetry with retries/multi-attempt futures.
- *"Skip = no selections rows" derivation* — considered and rejected. Without an explicit `status` enum, "no selections rows" would conflate `delivered` (pending) with `skipped`. The `status` enum dissolves this: skip lives in `status='skipped'`, derived from nothing.

### Decision: Lifecycle audit — `question_events` event log (Variant B)

Source of truth for temporal/audit data. Every transition is a row insert with `event_type`, `occurred_at`, `actor`, extensible `payload` (JSON). `questions` and `selections` carry zero `DATETIME` columns.

**Alternatives considered:**
- *Variant A (timestamps as columns on the row)* — simpler, 3-4 nullable `DATETIME`s. Rejected: the future-fit signals (resurfacing, re-grades, multi-attempt) would force `ALTER TABLE` for each new event type; the event-log spine is cheap now and expensive to backfill later.
- *Generic `events` table (polymorphic `entity_type`+`entity_id`)* — rejected: cannot enforce FK integrity with SQLite, dispatches on `entity_type`. Entity-scoped `question_events` enforces a real FK and avoids polymorphism. `selection_events` is the symmetric sibling if `selections` later needs its own audit (e.g., retries), not built speculatively.

### Decision: `questions.status` is a denormalized cache, not source of truth

`status` column with CHECK constraint (`queued`/`delivered`/`answered`/`skipped`) for fast `WHERE` filtering without JOINs. The source of truth is `question_events` — the latest event of a question determines its status. Both are updated atomically in the same transaction (insert event row + update `status`). The CHECK on `status` constrains the cache column to the controlled vocabulary; `question_events.event_type` has a CHECK with the same values plus room for future additions (`claimed`/`graded`/`resurfaced`).

**Alternative considered:** pure normalization (drop `status`, derive on every read via subquery on `question_events`). Rejected: uglier SQL, harder to index, no real benefit at current scale (single in-process SQLite connection).

### Decision: Wire contract — full break, index-based, "select"/"correct" lexical split

- Endpoint rename: `POST /recall/answer` → `POST /recall/select`. MCP tool rename: `recall_answer` → `recall_select`.
- User-side fields use `select`/`selected` exclusively (`selected_index`, `selected_indices`, `selected_text`).
- Engine-side response fields use `correct`/`correct_answer` (`correct_index`, `correct_answer`, `correct_indices`, `correct_answers`).
- "answer" is banned on the user side (verb ambiguity, collides with the engine key); permitted on the engine side (output of inference pipeline).
- `correct_index` (and per-type equivalents) withheld at delivery — fixes the leak where the client knows which choice is correct before submitting.
- Submit-response sends both `correct_index` and `correct_answer` (send-both) — wire-denormalization for stateless MCP consumers (an LLM generating feedback cannot rely on retaining the `choices` array from a prior `recall_select` tool call). DB-level "no redundancy" rule does not extend to the wire (no drift risk; response is computed once per submit).

**Alternatives considered:**
- *Index-based vs. ID-based wire.* Index-based chosen: choices are not addressable across requests (each question is delivered-once-answered-once), so opaque IDs provide no addressing value. SQLite AUTOINCREMENT IDs are sequential and offer no unguessability. Index preserves the existing pattern (mcp.go:95 already uses `correct_index`, ask.go:306 already uses `answer_index`).
- *Translation layer (stable wire, internal DB refactor)* — rejected: would require two names for one thing (the thing the user explicitly wanted to avoid). Per the ubiquity rule, the wire must move atomically with the schema.

### Decision: Migration — pure SQL teardown, no in-app migration code, no `PRAGMA user_version`

`cache.Open()` continues to use `CREATE TABLE IF NOT EXISTS` for the new schema. The maintainer runs a teardown SQL script (`DROP TABLE IF EXISTS questions, choices, question_events, selections;` or simply deletes `~/.tr/memory.db`) before the first run of the new build. No `PRAGMA user_version` probe, no row purge, no `ALTER TABLE` migration path.

**Alternative considered:** `PRAGMA user_version`-based migrator that detects schema mismatch and deletes the DB on `Open()`. Rejected by the user as out-of-scope for app code ("just run SQL"); also too aggressive if it deleted on every `Open()`. The maintainer-initiated teardown matches "blow it away" intent without introducing migration machinery.

### Decision: Shuffle stays in `recall` (presentation logic)

`recall.Synthesize` retains the `rand.Shuffle` to randomize display order, but produces a `[]Choice` with a `Position` ordering rather than mutating a `[]string` and tracking `correctIdx == i`. The store receives the ordered `[]Choice` and persists `position`/`text`/`is_correct` per row. The shuffle is presentation logic; the store does not know display order needs randomizing.

**Alternative considered:** push shuffle into the store (option b) or into SQL `ORDER BY RANDOM()` (option c). Rejected: (b) blurs the "recall decides, store persists" boundary; (c) hides randomization in a way that's hard to test deterministically.

## Risks / Trade-offs

- **[Risk] Wire contract is a hard break with no deprecation window.** → Mitigated by zero live users. Daemon, TUI, hooks, and MCP all update atomically in this change.
- **[Risk] Teardown migration deletes all existing concepts and questions.** → Mitigated by pre-release status. Documented in tasks; maintainer runs the teardown SQL once.
- **[Risk] `status` cache column can drift from `question_events` source of truth.** → Mitigated by atomic transaction: every transition inserts the event row and updates `status` in a single `tx.Commit()`. Single in-process SQLite connection (`MaxOpenConns=1`) serializes all access.
- **[Trade-off] `selections` has no per-row timestamp.** "When was this selection made?" requires a JOIN to `question_events` (the parent question's `answered` event). Acceptable because first-attempt is the only mode today; incremental submittal is explicitly out of scope. If it lands, `selection_events` is the symmetric sibling to add.
- **[Trade-off] Denormalized `status` violates the thread-1 "no redundancy" principle.** Acceptable because the principle was about DB denormalization where it could drift silently; `status` + `question_events` are atomically coupled and the cache unlocks trivial `WHERE` filters (`RecentAnswered`, `NextQuestion`, `QueueDepth`). Without the cache, every filter becomes a subquery on the event log.
- **[Trade-off] `question_type` is frontloaded with two unused values (`multi_select`, `free_text`).** Acceptable: one column, one CHECK, no destructive cost. Dispatch logic is reserved until those types ship, so no speculatively-built presentation/grading code accrues.
- **[Trade-off] Wire submit-response sends `correct_answer` text in addition to `correct_index`.** Slight wire-size cost (~50-100 chars per submit). Acceptable for stateless MCP consumer feedback generation; DB-level normalization is preserved.
- **[Risk] Delivering `correct_index` in the submit-response could leak if the client never submits.** → Mitigated by design: submit-response only fires *after* a submit is received, so the correctness key is delivered only to clients who have already picked. The leak being fixed is the *delivery* step (pre-submit), not the submit-response (post-submit).
- **[Open] Free-text grading mechanism (exact-match / rubric / partial-credit / AI-graded) is undecided.** Schema accommodates all via `questions.correct_answer` (nullable) + a future `score REAL` column; the choice is deferred to a later phase.

## Migration Plan

1. Maintainer runs teardown SQL:
   ```sql
   DROP TABLE IF EXISTS questions;
   -- choices/question_events/selections don't exist yet; the new Open() creates them.
   -- Or, equivalently: rm ~/.tr/memory.db
   ```
2. New build's `Open()` runs `CREATE TABLE IF NOT EXISTS` for the four new tables (`questions` reshaped, `choices`, `question_events`, `selections`) and the new index.
3. First `post-commit` hook generates new `question_events` `queued` rows; first `tr ask` claims via `delivered` event; first submit inserts `selections` rows + `answered` event.
4. Rollback: delete `~/.tr/memory.db` and reinstall the prior build. No data preservation.

## Open Questions

- Free-text grading mechanism — deferred to its own phase. Schema is ready; presentation/grading/UI not built speculatively.
- Multi-select presentation flow — delivered wire shape is decided (`selected_indices`, `correct_indices`); UI/presentation not built speculatively.
- Adaptive resurfacing algorithm — `question_events` is shaped to support `resurfaced` events without schema change; algorithm deferred.
- `selection_events` for per-pick audit — not built speculatively; would be the symmetric sibling to `question_events` if retries or incremental submittal land.