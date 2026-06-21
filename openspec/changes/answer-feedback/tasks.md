## 1. Schema Extension (answer-schema)

- [ ] 1.1 Add helper `addColumnIfMissing(ctx context.Context, db *sql.DB, table, column, definition string) error` in `internal/cache/store.go` — executes `ALTER TABLE <table> ADD COLUMN <column> <definition>`; inspects the error string for `"duplicate column name"` and returns nil in that case; propagates all other errors
- [ ] 1.2 Update `createQuestionsTableSQL` constant to include all Phase 4C columns for fresh installs:
  ```sql
  CREATE TABLE IF NOT EXISTS questions (
      id             INTEGER  PRIMARY KEY AUTOINCREMENT,
      question       TEXT     NOT NULL,
      choices        TEXT     NOT NULL,
      correct_index  INTEGER  NOT NULL DEFAULT 0,
      queued_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
      delivered_at   DATETIME,
      claimed_by     TEXT,
      answer         TEXT,
      answer_index   INTEGER,
      correct        INTEGER,
      feedback       TEXT,
      answered_at    DATETIME
  );
  ```
- [ ] 1.3 In `store.Open()`, after running `createQuestionsTableSQL`, call `addColumnIfMissing` for each new column on existing installs:
  - `correct_index INTEGER NOT NULL DEFAULT 0`
  - `answer_index INTEGER`
  - `correct INTEGER`
  - `feedback TEXT`
- [ ] 1.4 Update `StoredQuestion` struct: add `CorrectIndex int`, `AnswerIndex *int`, `Correct *bool`, `Feedback *string`
- [ ] 1.5 Update `SaveQuestion(ctx context.Context, question string, choices []string, correctIndex int) error` — add `correctIndex` parameter; update INSERT to include `correct_index`
- [ ] 1.6 Add `(*Store).GetQuestion(ctx context.Context, id int64) (*StoredQuestion, error)` — `SELECT id, question, choices, correct_index, queued_at FROM questions WHERE id = ?`; returns `nil, nil` if not found
- [ ] 1.7 Update `(*Store).AnswerQuestion(ctx context.Context, id int64, answerIndex int, answerText string, correct bool, feedback string) error` — UPDATE sets `answer = ?`, `answer_index = ?`, `correct = ?`, `feedback = ?` (NULL if empty string), `answered_at = datetime('now')` WHERE `id = ?`
- [ ] 1.8 Add `(*Store).SkipQuestion(ctx context.Context, id int64) error` — `UPDATE questions SET answer = 'skip', answered_at = datetime('now') WHERE id = ?`; leaves `answer_index`, `correct`, `feedback` NULL
- [ ] 1.9 Update `(*Store).RecentAnswered` to SELECT and scan `correct_index`, `answer_index`, `correct`, `feedback` in addition to existing fields; use nullable scan types (`*int`, `*bool`, `*string`) for nullable columns
- [ ] 1.10 Update `runPipeline` in `internal/engine/server.go`: change `s.store.SaveQuestion(ctx, q.Question, q.Choices)` to `s.store.SaveQuestion(ctx, q.Question, q.Choices, q.CorrectIndex)`
- [ ] 1.11 Update `(*Store).NextQuestion` in `internal/cache/store.go`: add `correct_index` to the RETURNING clause; populate `sq.CorrectIndex` from the scanned value. Required so the MCP `recall_next` tool (task 4.1) can return the real `correct_index` to AI agents — without this, `q.CorrectIndex` is always 0 on the dequeue path
- [ ] 1.12 Update `(*Store).PeekNextQuestion` in `internal/cache/store.go`: add `correct_index` to the SELECT list; populate `sq.CorrectIndex`. Keeps `StoredQuestion.CorrectIndex` consistent across all read paths (used by the `recall://queue` resource)
- [ ] 1.13 Run `go build ./...` and `go vet ./...` — verify clean

## 2. Feedback Generation (feedback-generation)

- [ ] 2.1 Add `feedbackMaxTokens = 150` constant to `internal/recall/prompts.go`
- [ ] 2.2 Add `FeedbackRequest(question string, choices []string, correctIndex, answerIndex int, model string) ai.CompletionRequest` to `internal/recall/prompts.go`:
  - System prompt: direct, plain prose, no markdown, max 3 sentences; correct case — confirm + explain why right; incorrect case — name correct answer explicitly, explain why right, briefly note why chosen answer doesn't fit, no apologizing
  - User turn: lists all choices with `← correct, chosen` / `← correct` / `← chosen (incorrect)` / (unlabeled) annotations; closes with verdict sentence (`"The developer answered correctly."` or `"The developer chose option N and was incorrect."`)
  - Returns `ai.CompletionRequest{Model: model, System: system, UserTurn: userTurn, MaxTokens: feedbackMaxTokens, JSON: false}`
- [ ] 2.3 Add `(*Engine).GenerateFeedback(ctx context.Context, question string, choices []string, correctIndex, answerIndex int, model string) (string, error)` to `internal/recall/engine.go`:
  - Calls `FeedbackRequest(...)` then `e.provider.Complete(ctx, req)`
  - On error: logs `[recall] feedback AI call failed: <err>` and returns `"", nil` (degraded, not fatal)
  - Returns the raw response string (plain prose, not JSON)
- [ ] 2.4 Update `handleRecallAnswer` in `internal/engine/server.go`:
  - Request body struct: `struct{ ID int64; AnswerIndex *int; Skip bool }` — `AnswerIndex` is a pointer to distinguish "not provided" from 0
  - Decode body; if `Skip == true` → call `s.store.SkipQuestion(ctx, id)` → respond `{"ok":true}`
  - Otherwise: call `s.store.GetQuestion(ctx, id)` — if nil respond 404; if `AnswerIndex` nil or out of range respond 400
  - Compute `correct = (*req.AnswerIndex == q.CorrectIndex)`; `answerText = q.Choices[*req.AnswerIndex]`; `correctText = q.Choices[q.CorrectIndex]`
  - If `r.URL.Query().Get("feedback") == "true"` → call `s.recallEngine.GenerateFeedback(ctx, q.Question, q.Choices, q.CorrectIndex, *req.AnswerIndex, model)`
  - Call `s.store.AnswerQuestion(ctx, id, *req.AnswerIndex, answerText, correct, feedback)`
  - Respond `200 OK`: `{"ok":true,"correct":<bool>,"correct_text":"<text>","feedback":"<text>"|null}`
- [ ] 2.5 Update `GET /recall/next` response: verify `correct_index` is NOT included in the JSON response for terminal callers (confirm existing `handleRecallNext` only returns `id`, `question`, `choices`)
- [ ] 2.6 Run `go build ./...` and `go vet ./...` — verify clean; manually test correct and incorrect paths with `curl` against a running daemon

## 3. tr ask Feedback UX (tr-ask-feedback)

- [ ] 3.1 Add `stateFeedback` to the `askState` enum in `cmd/total-recall/ask.go`
- [ ] 3.2 Add message types: `feedbackMsg{correct bool; correctText string; feedback string}` and `skipMsg{}`
- [ ] 3.3 Update `postAnswer(id int64, answerIndex int) tea.Cmd` — change signature to accept `answerIndex int` (not choice text); POST body `{"id": N, "answer_index": N}`; URL `POST /recall/answer?feedback=true`; parse response into `feedbackMsg{correct, correctText, feedback}`; on connection error return `feedbackMsg{}` with zero values. Also delete the unused `postAnswerCmd` method (dead code in current `ask.go:244-249`, never invoked)
- [ ] 3.4 Add `postSkip(id int64) tea.Cmd` — POST body `{"id": N, "skip": true}` to `/recall/answer`; return `skipMsg{}`; on connection error return `skipMsg{}` (best-effort)
- [ ] 3.5 Update `updateQuestion` keypress handler:
  - `[1-3]` press: compute `answerIndex = int(key[0] - '1')`; transition to `stateFeedback`; return `postAnswer(id, answerIndex)` as `tea.Cmd`
  - `Enter`: transition to `stateDone`; return `postSkip(id)` as `tea.Cmd`
  - `q` / `Esc` / `Ctrl+C`: transition to `stateDone`; no POST; return `tea.Quit`
- [ ] 3.6 Add `updateFeedback(msg tea.Msg) (tea.Model, tea.Cmd)` state handler:
  - `feedbackMsg` received: store in model; transition to `stateDone`; return `tea.Quit`
  - `tea.KeyMsg` with Ctrl+C: transition to `stateDone`; return `tea.Quit`
  - All other messages: return `m, nil`
- [ ] 3.7 Update `View()` for `stateFeedback`: return `"Evaluating..."` (static, no animation frame)
- [ ] 3.8 Wire `updateFeedback` into the `Update()` dispatch switch
- [ ] 3.9 Update `askModel` struct: add `feedbackResult feedbackMsg` (correct/incorrect result with `correctText` and `feedback`); add `skipped bool`; rename the existing `feedback string` field to `advisory string` (it holds caught-up and daemon-unreachable terminal messages — not quiz feedback)
- [ ] 3.10 Update post-alt-screen rendering in `askCmd.RunE`: inspect `am.skipped`, `am.feedbackResult`, and `am.advisory` in this order (mutually exclusive — exactly one is set per run):
  - `am.skipped`: print `"→ Question saved for later."`
  - `am.feedbackResult` populated — Correct: print `"✓ Correct.\n\n  <feedback>"` (omit feedback paragraph if empty)
  - `am.feedbackResult` populated — Incorrect: print `"✗ The answer was: <correct_text>\n\n  <feedback>"` (omit feedback paragraph if empty)
  - `am.advisory != ""` (caught up / daemon unreachable): print `am.advisory` as-is
  - q/Esc exit: print nothing
- [ ] 3.11 Update `cmd/total-recall/ask_test.go` existing tests that read the renamed `feedback` field:
  - `TestAskTimeoutSetsCaughtUpFeedback` — change `got.feedback` to `got.advisory`
  - `TestAskDaemonUnreachableShowsAdvisory` — change `got.feedback` to `got.advisory`
  - `TestAskProgramRunKeepsCaughtUpFeedbackOnTimeout` — change `got.feedback` to `got.advisory` (and the comparison value)
- [ ] 3.12 Run `go build ./...` and `go vet ./...` — verify clean; manually test all five exit paths (correct, incorrect, skip, q-exit, caught-up) with a running daemon and a queued question

## 4. MCP Tool Updates (mcp-answer-updates)

- [ ] 4.1 Update `recall_next` tool response in `internal/engine/mcp.go`: add `"correct_index": q.CorrectIndex` to the JSON map returned when a question is found
- [ ] 4.2 Update `recallAnswerIn` struct: change from `{ID int64; Answer string}` to `{ID int64; AnswerIndex *int \`json:"answer_index"\`; Skip bool \`json:"skip"\`}`
- [ ] 4.3 Update `recall_answer` tool handler:
  - If `in.Skip == true` → call `s.store.SkipQuestion(ctx, in.ID)` → return `{"ok":true}`
  - Otherwise: call `s.store.GetQuestion(ctx, in.ID)` — if nil return tool error; validate `answer_index` in range
  - Compute `correct`, `answerText`, `correctText` — same arithmetic as REST handler
  - Call `s.store.AnswerQuestion(ctx, in.ID, *in.AnswerIndex, answerText, correct, "")` — no feedback string on MCP path
  - Return `{"ok":true,"correct":<bool>,"correct_index":<N>,"correct_text":"<text>"}`
- [ ] 4.4 Update `recallWorkflowInstructions` constant: after recording an answer, instruct the agent to tell the user whether they were correct and provide a brief explanation using its own knowledge — especially for incorrect answers; explicitly state not to call a separate AI for this
- [ ] 4.5 Update `recall://recent` resource handler: iterate `answered` slice; include `correct_index`, `answer_index`, `correct`, `feedback` in each item map; use `nil` for NULL fields
- [ ] 4.6 Run `go build ./...` and `go vet ./...` — verify clean

## 5. Documentation

- [ ] 5.1 Update `DOCS/ARCHITECTURE/DELIVERY.md`: add Phase 4C subsection under Phase 4A describing the answer evaluation loop, `?feedback=true` path, MCP self-explanation pattern, and skip behavior
- [ ] 5.2 Update `ROADMAP.md`: add Phase 4C entry — answer evaluation, AI feedback for terminal users, correctness persistence, foundation for spaced repetition
- [ ] 5.3 Add Phase 4C section to `DOCS/TESTING/E2E.md`: sections covering correct answer feedback render, incorrect answer feedback render, skip acknowledgement, feedback AI failure degradation, MCP correctness return, `recall://recent` enrichment
- [ ] 5.4 Run `go build ./...` and `go vet ./...` — final clean build verification
