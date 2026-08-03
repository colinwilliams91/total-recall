## 1. Teardown & Schema

- [x] 1.1 Write teardown SQL script (drop `questions`, or delete `~/.tr/memory.db`) — document in change README or commit message; no app code changes
- [x] 1.2 Reshape `internal/cache/store.go` `createQuestionsTableSQL` — drop `queued_at`, `delivered_at`, `claimed_by`, `answer`, `answer_index`, `correct_index`, `correct`, `answered_at`, `choices` (JSON); add `question_type` CHECK enum, `status` CHECK enum, `correct_answer` nullable TEXT, keep `feedback`
- [x] 1.3 Add `createChoicesTableSQL` — `id` PK, `question_id` FK CASCADE, `position`, `text`, `is_correct` INTEGER NOT NULL DEFAULT 0; covering index `idx_choices_qid`
- [x] 1.4 Add `createSelectionsTableSQL` — `id` PK, `question_id` FK CASCADE, `choice_id` FK CASCADE, `UNIQUE(question_id, choice_id)`; no timestamp column
- [x] 1.5 Add `createQuestionEventsTableSQL` — `id` PK, `question_id` FK CASCADE, `event_type` CHECK enum, `occurred_at` DEFAULT CURRENT_TIMESTAMP, `actor`, `payload TEXT` nullable; partial index `idx_qe_qid_time`
- [x] 1.6 Update `Open()` to run all four `CREATE TABLE IF NOT EXISTS` calls + indexes idempotently

## 2. cache.Store types & persistence

- [x] 2.1 Replace `StoredQuestion` struct: `Choices []Choice` with `Choice{ID int64, Text string, IsCorrect bool, Position int}`; keep `CorrectIndex int` as derived field; replace `AnswerIndex *int`/`Correct *bool`/`Feedback *string` with a `Status` field and joined `Selections []Selection`
- [x] 2.2 Add `Selection` struct: `ID int64, QuestionID int64, ChoiceID int64`
- [x] 2.3 Rewrite `SaveQuestion(ctx, repo, branch, question, choices []Choice, questionType string)` — single tx: insert `questions` row (`status='queued'`), insert N `choices` rows with position/text/is_correct, insert `question_events` `'queued'` row with `actor='system'`
- [x] 2.4 Rewrite `NextQuestion` — single tx: atomic UPDATE `status='delivered'` + insert `question_events` `'delivered'` row with `actor=claimedBy`; join `choices` for the returned question; preserve exactly-once delivery semantics
- [x] 2.5 Rewrite `GetQuestion` — join `choices` ordered by `position`; populate derived `CorrectIndex` from the `is_correct=1` choice's position
- [x] 2.6 Replace `AnswerQuestion` with `SubmitSelection(ctx, questionID, selectedChoiceIDs []int64)` — single tx: guard `status='delivered'`, insert `selections` rows, update `status='answered'`, insert `question_events` `'answered'` row
- [x] 2.7 Rewrite `SkipQuestion(ctx, id)` — single tx: guard `status IN ('queued','delivered')`, update `status='skipped'`, insert `question_events` `'skipped'` row; no `selections` inserts
- [x] 2.8 Rewrite `QueueDepth` — `WHERE status='queued' AND repo=? AND branch=?`
- [x] 2.9 Rewrite `PeekNextQuestion` — `WHERE status='queued' AND repo=? AND branch=?` ordered by the `'queued'` event `occurred_at` ASC; no claim (no status update, no event insert)
- [x] 2.10 Rewrite `RecentAnswered` — `WHERE status='answered' AND repo=? AND branch=?` ordered by the `'answered'` event `occurred_at` DESC; exclude skipped
- [x] 2.11 Add `RecentSkipped(ctx, repo, branch, limit)` — `WHERE status='skipped' AND repo=? AND branch=?` ordered by the `'skipped'` event `occurred_at` DESC
- [x] 2.12 Add `RecentQuestions(ctx, repo, branch, limit)` — `WHERE status IN ('answered','skipped') AND repo=? AND branch=?` ordered by terminal-event `occurred_at` DESC; returned rows carry their `Status` for distinction
- [x] 2.13 Update `StalePerBranch` — `WHERE status='queued' AND repo=? AND branch != '' GROUP BY branch`
- [x] 2.14 Add `SetFeedback(ctx, id, feedback string)` — separate feedback persistence; the synchronous submit path persists selection first, feedback second (failure-tolerant)

## 3. recall.Engine

- [x] 3.1 Reshape `recall.Question` struct: `Choices []Choice` where `Choice{Text string; IsCorrect bool}`; keep `CorrectIndex int` derived (computed once post-shuffle)
- [x] 3.2 Rewrite `Synthesize` shuffle — wrap AI-returned `[]string` into `[]Choice` with `IsCorrect=true` at index 0 and `IsCorrect=false` elsewhere; shuffle mutates the slice order without the `correctIdx == i` arithmetic
- [x] 3.3 Reshape `GenerateFeedback(ctx, question string, choices []Choice, selectedIndex, correctIndex int, model string)` signature
- [x] 3.4 Update `SynthesisRequest` to keep the AI contract (`choices[0]` is the correct answer)
- [x] 3.5 Update `FeedbackRequest` to annotate choices using the typed `Choice` rows (the `IsCorrect` flag iterates directly — no index lookup for "which is correct")

## 4. engine.Server HTTP handlers

- [x] 4.1 Rename `/recall/answer` → `/recall/select` in route registration and handler
- [x] 4.2 Update submit-request parsing: `selected_index *int` (was `answer_index`); reject legacy `answer_index` field explicitly with 400 if present (lexical rule enforcement)
- [x] 4.3 Update submit handler to call `SubmitSelection(ctx, id, []int64{choiceID at selected_index})` instead of `AnswerQuestion`
- [x] 4.4 Update submit-response: rename wire field `correct_text` → `correct_answer`; also add `correct_index` (send-both — was withheld at delivery, delivered at submit-response); skip response shape `{"ok":true,"skipped":true}`
- [x] 4.5 Update delivery response (`GET /recall/next`): drop `correct_index`; add `question_type` field; response keys are exactly `id, question_type, question, choices`
- [x] 4.6 Add 404 handler for `/recall/answer` (soft rejection of legacy path) — or simply let the unregistered route return 404 by default; verify behavior
- [x] 4.7 Update error response string `"answer_index out of range"` → `"selected_index out of range"`
- [x] 4.8 Update `GET /recall/stale` predicate from `WHERE delivered_at IS NULL` to `WHERE status='queued'` (this is in the store; the handler remains a passthrough)
- [x] 4.9 Update `recall://recent` MCP resource handler — choose between `RecentAnswered` (probably the right default) vs. `RecentQuestions`; update displayed fields per the new wire shape

## 5. engine.MCP tools

- [x] 5.1 Rename MCP tool `recall_answer` → `recall_select` in tool registration
- [x] 5.2 Update `recallSelectIn` struct field `AnswerIndex *int` → `SelectedIndex *int`; update error string `"answer_index is required"` → `"selected_index is required"`; `"answer_index out of range"` → `"selected_index out of range"`
- [x] 5.3 Update `recall_next` response: drop `correct_index` (delivery leak fix); add `question_type`
- [x] 5.4 Update `recall_select` response keys: `correct_text` → `correct_answer` (send-both with `correct_index` retained)
- [x] 5.5 Update `recallWorkflowInstructions` (mcp.go:16) — adjust wire field references in the LLM-facing instructions ("`selected_index`" not "`answer_index`", "`correct_answer`" not "`correct_text`", note that `correct_index` is withheld at delivery and arrives in the submit-response)
- [x] 5.6 Update MCP recent-history resource — refresh field names in item map (`answer_index` → drop or replace with `selected_indices`; `correct_index` stays in key form)

## 6. TUI client (cmd/tr/ask.go)

- [x] 6.1 Update submit-request body marshal (ask.go:306): `answer_index` → `selected_index`; the HTTP path `/recall/answer` → `/recall/select`
- [x] 6.2 Update submit-response unmarshal: `correct_text` → `correct_answer` (ask.go around line 316); skip-response now carries `skipped: true`
- [x] 6.3 Verify the TUI flow never depended on `correct_index` at delivery time — the TUI stores the question text and choices, submits the user's pick, then reads `correct_index`/`correct_answer` from the submit-response; this flow is unaffected by the delivery leak fix

## 7. Tests

- [x] 7.1 Update `cmd/tr/cache_test.go` for new schema: `setupCache` opens new schema; `TestSaveQuestionPersistsCorrectIndex` validates `choices.is_correct` row instead of `questions.correct_index` column; `TestRecentAnsweredEnrichedFields` re-splits to cover `RecentAnswered` (excludes skips), `RecentSkipped`, `RecentQuestions`
- [x] 7.2 Update `cmd/tr/provider_test.go` if `newProvider()` factory touches the recall engine (likely unaffected — adapter layer)
- [x] 7.3 Update `cmd/tr/ask_test.go` — submit-request body and submit-response unmarshal changes; skip-response `"skipped":true` handling
- [x] 7.4 Update `cmd/tr/main_test.go` if `buildPostCommitHookScript()` references the renamed endpoint — check hooks/ scripts for `/recall/answer` URL
- [x] 7.5 Update `cmd/tr/integration_test.go` — endpoint rename `/recall/answer` → `/recall/select`, body field `answer_index` → `selected_index`, error string, delivery response (assert no `correct_index`), submit-response shape (assert both `correct_index` and `correct_answer`), MCP `recall_answer` → `recall_select`, skip-response `"skipped":true`
- [x] 7.6 Update golden files via `$env:UPDATE_GOLDEN=1; go test -run TestGolden ./cmd/tr/...` — the TUI view snapshots change because the submit-response field name changed (`correct_text` → `correct_answer`)
- [x] 7.7 Add new integration test for delivery leak: GET `/recall/next` response body MUST NOT contain the string `correct_index`
- [x] 7.8 Add new integration tests for `SubmitSelection`/`SkipQuestion` atomicity — successful terminal state, terminal-state guard (cannot submit after answered, cannot skip after answered, cannot submit after skipped)
- [x] 7.9 Add new integration test for `RecentAnswered` excluding skips, `RecentSkipped` excluding answers, `RecentQuestions` including both with `Status` field distinguishing them

## 8. Verification

- [x] 8.1 `go build ./...`
- [x] 8.2 `go vet ./...`
- [x] 8.3 `go test ./...`
- [x] 8.4 `golangci-lint run` (if installed)
- [x] 8.5 Manual: run teardown SQL, start daemon (`tr serve`), trigger a `post-commit` hook, confirm `question_events` `'queued'` row appears, then `tr ask` to claim (confirm `'delivered'` event + `status='delivered'`), then submit a selection (confirm `selections` row + `'answered'` event + `status='answered'`)
- [x] 8.6 Manual: confirm `~/.tr/memory.db` no longer contains the `questions.queued_at`/`delivered_at`/`answered_at`/`answer`/`answer_index`/`correct_index` column names (schema fully reshaped)
