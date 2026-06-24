## 1. Cache Tests (cache_test.go)

- [ ] 1.1 Add `TestSaveQuestionPersistsCorrectIndex` — save a question with `correctIndex=2`, retrieve via `GetQuestion`, assert `CorrectIndex == 2` and choices match
- [ ] 1.2 Add `TestGetQuestionReturnsFullRow` — save and claim a question, call `GetQuestion`, assert all fields (`ID`, `Question`, `Choices`, `CorrectIndex`, `QueuedAt`); also assert `GetQuestion(ctx, 99999)` returns `nil, nil`
- [ ] 1.3 Add `TestSkipQuestionLeavesNulls` — save, claim, and skip a question; query the row directly (or via `RecentAnswered`) and assert `answer == "skip"`, `AnswerIndex == nil`, `Correct == nil`, `Feedback == nil`
- [ ] 1.4 Add `TestAnswerQuestionWithFeedback` — save with `correctIndex=0`, answer with `AnswerQuestion(ctx, id, 1, "b", false, "A is correct because...")`; verify via `RecentAnswered` that `Correct` is `*false`, `Feedback` is non-nil with the feedback text
- [ ] 1.5 Add `TestAnswerQuestionEmptyFeedbackStoresNull` — answer with `feedback=""`; verify via `RecentAnswered` that `Feedback` is nil (not empty string)
- [ ] 1.6 Add `TestRecentAnsweredEnrichedFields` — answer three questions (correct with feedback, incorrect without feedback, skipped); call `RecentAnswered`; assert `CorrectIndex` on all, `AnswerIndex`/`Correct`/`Feedback` nil for skip, `Feedback` nil for MCP-style, `Feedback` non-nil for terminal-style
- [ ] 1.7 Add `TestAddColumnIfMissingMigration` — create a DB with the old Phase 4A schema (no new columns), call `cache.Open()`, verify all 4 columns exist; call `cache.Open()` again and verify no error (idempotent)

## 2. Ask State Machine Tests (ask_test.go)

- [ ] 2.1 Add `TestUpdateFeedbackReceivesFeedbackMsg` — create a model in `stateFeedback`, call `updateFeedback(feedbackMsg{correct: true, correctText: "X", feedback: "Y"})`, assert `state == stateDone`, `feedbackResult` populated, cmd is `tea.Quit`
- [ ] 2.2 Add `TestUpdateFeedbackReceivesSkipMsg` — call `updateFeedback(skipMsg{})`, assert `state == stateDone`, `skipped == true`, cmd is `tea.Quit`
- [ ] 2.3 Add `TestUpdateFeedbackCtrlC` — call `updateFeedback(tea.KeyMsg{Type: tea.KeyCtrlC})`, assert `state == stateDone`, cmd is `tea.Quit`
- [ ] 2.4 Add `TestStateFeedbackView` — set `m.state = stateFeedback`, call `m.View()`, assert output contains `"Evaluating..."` and does not contain `"Thinking."`
- [ ] 2.5 Add `TestPostAnswerParsesResponse` — start a test daemon, seed a question, execute `postAnswer(id, 0, client)` as a `tea.Cmd`, assert the returned `tea.Msg` is a `feedbackMsg` with `correct` and `correctText` populated from the server response
- [ ] 2.6 Add `TestPostSkipReturnsSkipMsg` — start a test daemon, seed a question, execute `postSkip(id, client)` as a `tea.Cmd`, assert the returned `tea.Msg` is `skipMsg{}`
- [ ] 2.7 Add `TestPostAltScreenRendering` — set `feedbackResult`/`skipped`/`advisory` on `askModel` values and assert the rendered output for each case: correct (`✓ Correct.` + feedback), incorrect (`✗ The answer was:` + feedback), skip (`→ Question saved for later.`), advisory (verbatim), q/Esc (empty). Capture stdout via `os.Pipe` or extract render logic if needed.
- [ ] 2.8 Add `TestAskModelQKeyExitsSilently` — transition to `stateQuestion`, press `q`, assert `state == stateDone`, cmd is `tea.Quit` (not `postAnswer`/`postSkip`), and no output would be printed (feedbackResult/skipped/advisory all zero)

## 3. Integration Tests (integration_test.go)

- [ ] 3.1 Add `TestRecallAnswerWithFeedbackTrue` — seed a question with `correctIndex=0`, `POST /recall/answer?feedback=true` with `{"id":N,"answer_index":0}`, assert response includes `correct`, `correct_text`, `feedback` fields (feedback may be null if no AI provider on test daemon)
- [ ] 3.2 Add `TestRecallAnswerCorrectEvaluation` — seed with `correctIndex=0`, answer with `answer_index=0`, assert `correct == true`
- [ ] 3.3 Add `TestRecallAnswerIncorrectEvaluation` — seed with `correctIndex=0`, answer with `answer_index=1`, assert `correct == false` and `correct_text` is the choice at index 0
- [ ] 3.4 Add `TestRecallAnswerOutOfRange` — seed a question with 3 choices, `POST /recall/answer?feedback=true` with `answer_index=99`, assert `400` status and `{"error":"answer_index out of range"}`
- [ ] 3.5 Add `TestRecallAnswerUnknownID` — `POST /recall/answer?feedback=true` with `id=99999`, assert `404` status
- [ ] 3.6 Add `TestMCPRecallNextReturnsCorrectIndex` — seed a question with `correctIndex=2`, call `recall_next` via MCP JSON-RPC over HTTP, assert response includes `"correct_index": 2`
- [ ] 3.7 Add `TestMCPRecallAnswerReturnsCorrectness` — seed a question, call `recall_answer` via MCP JSON-RPC with `{"id":N,"answer_index":0}`, assert response includes `ok`, `correct`, `correct_index`, `correct_text`
- [ ] 3.8 Add `TestMCPRecallAnswerSkip` — seed a question, call `recall_answer` via MCP JSON-RPC with `{"id":N,"skip":true}`, assert `{"ok":true}`
- [ ] 3.9 Add `TestRecallRecentEnriched` — answer questions (correct, skip), read `recall://recent` via MCP JSON-RPC, assert rows include `correct_index`, `answer_index`, `correct`, `feedback` with correct nullability

## 4. Golden File (golden_test.go)

- [ ] 4.1 Add `TestGoldenAskFeedbackView` — set `m.state = stateFeedback`, call `m.View()`, compare against `testdata/TestGoldenAskFeedbackView.golden`
- [ ] 4.2 Generate the golden file: `$env:UPDATE_GOLDEN=1; go test -run TestGoldenAskFeedbackView ./cmd/total-recall/...` then verify without the flag

## 5. Recall Package Tests (recall_test.go — NEW FILE)

- [ ] 5.1 Create `cmd/total-recall/recall_test.go` with `mockProvider` struct implementing `ai.Provider` (fields: `response string`, `err error`; method: `Complete` returns canned response or error)
- [ ] 5.2 Add `TestFeedbackRequestCorrectCase` — call `FeedbackRequest` with `correctIndex == answerIndex`, assert user turn contains `← correct, chosen` and ends with `"The developer answered correctly."`
- [ ] 5.3 Add `TestFeedbackRequestIncorrectCase` — call `FeedbackRequest` with `correctIndex != answerIndex`, assert user turn contains `← correct` on correct choice, `← chosen (incorrect)` on chosen choice, ends with `"The developer chose option N and was incorrect."`
- [ ] 5.4 Add `TestFeedbackRequestTokenBudget` — call `FeedbackRequest`, assert `MaxTokens == 150` and `JSON == false` on the returned `ai.CompletionRequest`
- [ ] 5.5 Add `TestGenerateFeedbackDegradation` — create `recall.New(mockProvider{err: errors.New("timeout")}, store)`, call `GenerateFeedback`, assert returns `"", nil` (not an error)

## 6. ROADMAP Fixes

- [ ] 6.1 Update `ROADMAP.md`: change `### Phase 4D — Extended AI Providers (Planned)` to `### Phase 4D — Extended AI Providers (Shipped)`
- [ ] 6.2 Update `ROADMAP.md`: move `### Phase 4B — VS Code Extension (Next)` to `### Phase 4B — VS Code Extension (Deferred)` with explanation that it's moved out of Phase 4 scope; the REST API and MCP server are stable delivery surfaces; the extension is a UX enhancement, not a capability gap

## 7. Final Verification

- [ ] 7.1 Run `go build ./... && go vet ./... && go test ./...` — verify all tests pass, build and vet clean
- [ ] 7.2 Run `$env:UPDATE_GOLDEN=1; go test -run TestGolden ./cmd/total-recall/...` then `go test -run TestGolden ./cmd/total-recall/...` — verify golden files are stable
