## 1. Cache layer removal

- [ ] 1.1 In `internal/cache/store.go`, narrow `createQuestionsTableSQL`: drop the `correct_answer TEXT` column and remove `'free_text'` from the `question_type` CHECK constraint; verify by grepping the file for zero remaining free-text/`correct_answer` column references
- [ ] 1.2 Remove the `CorrectAnswer *string` field from `StoredQuestion` and the `correct_answer` SELECT terms plus `sql.NullString` scan/branch wiring in `RecentQuestions`, `RecentAnswered`, `GetQuestion`, and `PeekNextQuestion`; verify with `go build ./...`
- [ ] 1.3 Update `deriveCorrectIndex` doc comment to drop free-text framing while keeping the -1 sentinel justification; verify with `go vet ./...`
- [ ] 1.4 Update `Open()`'s migration doc comment and `internal/cache/MIGRATION.md` to record this second destructive reshape (dropped `correct_answer` column, narrowed `question_type` CHECK) and the mandatory stale-`memory.db` teardown; verify the doc names both removals
- [ ] 1.5 Update/remove cache-layer tests asserting the old row shape or free-text CHECK semantics (e.g., row-shape tests in `cmd/tr/cache_test.go`); add a test that inserting `question_type = 'free_text'` against the new schema violates the CHECK; verify with `go test ./internal/cache/... ./cmd/tr/ -run TestSaveQuestion`

## 2. Spec deltas

- [ ] 2.1 Apply the `question-store` delta (schema requirement, choices requirement, GetQuestion requirement) to `openspec/specs/question-store/spec.md`; verify the main spec has zero free-text mentions via `rg -n free_text openspec/specs/`
- [ ] 2.2 Apply the `question-delivery` delta (multi-select-only discriminator sentence) to `openspec/specs/question-delivery/spec.md`; verify only multi-select remains as a reserved-value mention
- [ ] 2.3 Run `openspec validate --change remove-free-text-questions` after spec edits; resolves clean

## 3. Wire and behavior verification

- [ ] 3.1 Confirm the MC wire contract is unchanged: delivery response (`GET /recall/next`) keys and submit response (`POST /recall/select`) keys are byte-identical to before, with `correct_answer` in the submit response still sourced from `choices` text; verify `cmd/tr/integration_test.go` delivery-leak and submit-response tests pass
- [ ] 3.2 Run the full suite (`go build ./... && go vet ./... && go test ./...`) against a torn-down `$TR_HOME` fresh store; all green

## 4. Documentation reconciliation (parallel-branch dependency)

- [ ] 4.1 Run the `/domain-modeling` skill using the parallel branch's "no free-text" documentation as the source of truth; reconcile repo-side docs (e.g., `CONTEXT.md`, `AGENTS.md`, in-package doc comments) wherever they still describe or imply the free-text format; if the parallel branch has not landed yet, block this task until it does
- [ ] 4.2 Verify no remaining free-text references in code or docs outside archived OpenSpec changes: `rg -n "free_text|free-text|selected_text" --glob '!openspec/changes/archive/**'` returns nothing
