## Why

The `question_type` enum and `questions.correct_answer` column were frontloaded to reserve a "free-text" question/answer format (deferred-grading, `selected_text` submissions). That future is no longer a potential reality — the free-text design is vestigial. Since the store is pre-release with zero live users and schema changes are manual (`CREATE TABLE IF NOT EXISTS`, no `ALTER TABLE`), ripping it out now is cheap; leaving it accrues dead schema surface, misleading spec language, and an FK-less correctness key channel nobody will ever read.

## What Changes

- **BREAKING** (schema + teardown): Drop the `questions.correct_answer` column — it is the free-text correctness key and is never populated by any shipped code path (synthesis emits MC-only via `Server` → `SaveQuestion` with no `correct_answer`).
- **BREAKING** (schema + teardown): Narrow the `question_type` CHECK constraint from `('multiple_choice','multi_select','free_text')` to `('multiple_choice','multi_select')`. `question_type` itself stays (multi-select remains a reserved future value); only the free-text value is removed.
- Remove `StoredQuestion.CorrectAnswer` and its `sql.NullString` scan/branch wiring in all four `StoredQuestion` queries (`RecentQuestions`, `RecentAnswered`, `GetQuestion`, `PeekNextQuestion`).
- Update the `deriveCorrectIndex` doc comment to drop free-text framing (-1 sentinel justification stays).
- Update `Open()`'s migration doc comment and `internal/cache/MIGRATION.md` to record this second destructive reshape (stale `memory.db` teardown required).
- Update specs so free-text requirements/scenarios are removed: `question-store` (schema requirement, choices requirement, free-text save scenarios, GetQuestion free-text clause), `question-delivery` (free-text client dispatch mention). No wire format change: the submit-response `correct_answer` field is the MC correct-choice text and stays.
- Run `/domain-modeling` using the parallel-branch "no free-text" documentation as the source of truth so existing docs land consistent with the removal (docs are being updated on another branch — the task is reconciliation, not first-draft authoring).

Out of scope (recorded assumption): multi-select stays reserved — the request is free-text only. The submit-response wire field `correct_answer` (MC correct-choice text) stays unchanged. No prompt assets or recall-synthesis prompt changes (they already emit MC-1 only).

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `question-store`: Schema requirement — drop `correct_answer` column, narrow `question_type` CHECK to exclude `free_text`; remove free-text choices/deferred-grading scenarios and GetQuestion free-text clause.
- `question-delivery`: Delivery requirement — `question_type` is a discriminator for future multi-select clients only (free-text client mention removed).

## Impact

- `internal/cache/store.go` — schema DDL, `StoredQuestion` struct + 4 query scan sites, doc comments.
- `internal/cache/MIGRATION.md` — teardown instructions for the second reshape.
- Specs: `openspec/specs/question-store/spec.md`, `openspec/specs/question-delivery/spec.md`.
- No engine/MCP/TUI behavior change (those touch `correct_answer` only as the MC correct-choice text). Existing `memory.db` files created by prior builds retain the vestigial column harmlessly but must be torn down per MIGRATION.md before running a build with the narrowed CHECK.
- Tests: `cmd/tr/cache_test.go` row-shape assertions and any spec-approval tests referencing deleted language.
