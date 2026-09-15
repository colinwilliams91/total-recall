## Context

Free-text was only ever *reserved*, never shipped: no code path writes `questions.correct_answer`, no client sends `selected_text`, no presentation or grading logic branches on `question_type = 'free_text'`. The vestigial surface is confined to the cache layer plus spec language. The write path hardcodes `"multiple_choice"` (internal/engine/server.go → `SaveQuestion`); the read path carries `StoredQuestion.CorrectAnswer` through four SELECT scans (`RecentQuestions`, `RecentAnswered`, `GetQuestion`, `PeekNextQuestion`) but nothing consumes it.

Two same-named things must not be conflated: the DB column `questions.correct_answer` (free-text key — goes away) and the wire field `correct_answer` in the `POST /recall/select` response (MC correct-choice text — stays; populated from `choices`, not the column).

Schema changes are manual per project convention: `Open()` uses `CREATE TABLE IF NOT EXISTS`, no `ALTER TABLE` path; stale `memory.db` files are torn down by hand.

## Goals / Non-Goals

**Goals:**
- Remove the free-text schema surface: `correct_answer` column, `'free_text'` CHECK value, `StoredQuestion.CorrectAnswer` field and its scan wiring.
- Remove free-text language from the `question-store` and `question-delivery` requirements and scenarios.
- Leave the MC wire contract (delivery and submit responses) byte-identical.
- Reconcile existing documentation with the parallel-branch "no free-text" docs after implementation.

**Non-Goals:**
- Removing `multi_select` — it remains a reserved future value; only free-text was declared vestigial.
- Removing the `question_type` column or the `question_type` wire fields — both still discriminate MC vs. the reserved multi-select.
- Any migration tooling for existing `memory.db` files (manual teardown per `internal/cache/MIGRATION.md`).

## Decisions

- **Narrow the CHECK constraint rather than dropping `question_type`.** The column still carries `multiple_choice` today and `multi_select` as a reserved value; dropping the column would un-bootstraps multi-select later. Alternative considered — dropping the column entirely and treating all questions as MC implicitly: rejected because re-adding a type discriminator later is the exact "prohibitively expensive later" shape the original refactor avoided.
- **Delete `StoredQuestion.CorrectAnswer` outright rather than keeping it as dead field.** Nothing reads it, and keeping a NULL-always field invites accidental wire leakage of a concept the spec reserve should not exist. Alternative — keep until multi-select ships: rejected; it models free-text grading, not multi-select.
- **Second destructive reshape with the same teardown ritual.** Add the new drop set (`correct_answer` column, `free_text` CHECK value) to `Open()`'s migration doc comment and `internal/cache/MIGRATION.md`'s prior-schema list. Old DBs that keep `correct_answer` are harmless if torn down late, but the narrowed CHECK on a stale table means inserts of `free_text` would still succeed there — hence the doc must call the teardown mandatory before running the new build.
- **CHECK-probe dummy value must not name a possible future format.** The unknown-value scenario earlier probed with `'ranking'`; the design-space decision (explored and confirmed): `question_type` discriminates grading contracts, not pedagogy flavors — true/false is a two-choice `multiple_choice` question, not a separate type, so `true_false` is NOT a reserved candidate. `multi_select` remains reserved in the enum and `ranking` remains a candidate future type (a genuinely different contract: N ordered picks). The probe uses `'poll'` (arbitrary, never planned). No deferral to design grading for those types is created by this change — only `multiple_choice` ships rendering/grading; `multi_select` stays reserved with its `selections` join model intact; `ranking`'s answer handling (where pick-order lives) is a separate future decision.
- **Docs reconciliation is deferred to a dedicated task** (not folded into spec deltas): the "no free-text" documentation is being authored on a parallel branch, so its final location and wording are not yet knowable. Run `/domain-modeling` after the code lands, treating that documentation as the source of truth, and align any repo-side docs (e.g., `CONTEXT.md`, AGENTS.md schema blurbs) if they stray.

## Risks / Trade-offs

- [Stale `memory.db` + narrowed CHECK = silent half-migration] → `MIGRATION.md` and the `Open()` doc comment call out the mandatory teardown; the doc comment is the established convention for this.
- [Name collision confusion (`correct_answer` column vs. `correct_answer` wire field)] → delta specs keep the wire field's requirement text intact; tasks call out that the wire field is sourced from `Choices`, verified by the existing delivery-leak integration test.
- [Parallel-branch docs may land after this change] → the docs task is written as a reconciliation step that references the parallel branch's output rather than duplicating its content; if the branch hasn't landed, the task blocks on it by design.
