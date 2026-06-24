## Requirements

### Requirement: Cache layer tests cover 4C schema operations
`cache_test.go` SHALL include tests verifying: `SaveQuestion` persists non-zero `correct_index`, `GetQuestion` returns full rows and nil for missing IDs, `SkipQuestion` leaves nullable columns NULL, `AnswerQuestion` stores feedback as NULL when empty and non-NULL when provided, `RecentAnswered` returns enriched nullable fields, and `addColumnIfMissing` migration is idempotent on existing databases.

#### Scenario: SaveQuestion with non-zero correct_index
- **WHEN** `SaveQuestion(ctx, "q", []string{"a","b","c"}, 2)` is called
- **THEN** `GetQuestion(ctx, id)` returns a `StoredQuestion` with `CorrectIndex = 2`

#### Scenario: GetQuestion returns nil for missing ID
- **WHEN** `GetQuestion(ctx, 99999)` is called
- **THEN** `nil, nil` is returned (not an error)

#### Scenario: SkipQuestion leaves nulls
- **WHEN** `SkipQuestion(ctx, id)` is called
- **THEN** the row has `answer = "skip"` and `answer_index`, `correct`, `feedback` are NULL

#### Scenario: AnswerQuestion with empty feedback stores NULL
- **WHEN** `AnswerQuestion(ctx, id, 0, "a", true, "")` is called
- **THEN** the `feedback` column is NULL (not an empty string)

#### Scenario: AnswerQuestion with feedback stores text
- **WHEN** `AnswerQuestion(ctx, id, 1, "b", false, "A is correct because...")` is called
- **THEN** the row has `correct = 0`, `feedback = "A is correct because..."`

#### Scenario: RecentAnswered returns enriched nullable fields
- **WHEN** three questions are answered (correct with feedback, incorrect without feedback, skipped)
- **THEN** `RecentAnswered` returns all three with `CorrectIndex` populated, `AnswerIndex`/`Correct`/`Feedback` nil for the skipped row, `Feedback` nil for the MCP-style row, `Feedback` non-nil for the terminal-style row

#### Scenario: Migration adds columns idempotently
- **WHEN** a database exists with the Phase 4A schema (no `correct_index`, `answer_index`, `correct`, `feedback`)
- **AND** `cache.Open()` is called on it
- **THEN** all four columns are added and existing rows remain intact
- **AND** calling `cache.Open()` again does not error

---

### Requirement: Ask state machine tests cover stateFeedback and post-alt-screen rendering
`ask_test.go` SHALL include tests verifying: `updateFeedback` handles `feedbackMsg`/`skipMsg`/Ctrl+C correctly, `stateFeedback` view renders "Evaluating...", `postAnswer` cmd parses server response into `feedbackMsg`, `postSkip` cmd returns `skipMsg`, post-alt-screen rendering produces correct output for correct/incorrect/skip/advisory/q-exit cases, and q/Esc exits silently without posting.

#### Scenario: updateFeedback receives feedbackMsg
- **WHEN** `updateFeedback(feedbackMsg{correct: true, correctText: "...", feedback: "..."})` is called
- **THEN** the model transitions to `stateDone`, stores the result in `feedbackResult`, and returns `tea.Quit`

#### Scenario: updateFeedback receives skipMsg
- **WHEN** `updateFeedback(skipMsg{})` is called
- **THEN** the model transitions to `stateDone`, sets `skipped = true`, and returns `tea.Quit`

#### Scenario: updateFeedback handles Ctrl+C
- **WHEN** `updateFeedback(tea.KeyMsg{Type: tea.KeyCtrlC})` is called
- **THEN** the model transitions to `stateDone` and returns `tea.Quit`

#### Scenario: stateFeedback view renders static message
- **WHEN** `View()` is called on a model in `stateFeedback`
- **THEN** the output contains `"Evaluating..."` and does not advance an animation frame

#### Scenario: postAnswer cmd parses response
- **WHEN** `postAnswer(id, answerIndex, client)` is executed against a test daemon returning `{"ok":true,"correct":true,"correct_text":"...","feedback":"..."}`
- **THEN** the returned `tea.Msg` is a `feedbackMsg` with `correct=true`, `correctText` and `feedback` populated from the response

#### Scenario: postSkip cmd returns skipMsg
- **WHEN** `postSkip(id, client)` is executed against a test daemon
- **THEN** the returned `tea.Msg` is a `skipMsg{}`

#### Scenario: Post-alt-screen rendering — correct answer
- **WHEN** `askModel` has `feedbackResult{correct: true, feedback: "..."}`
- **THEN** the rendered output contains `"✓ Correct."` followed by the feedback paragraph

#### Scenario: Post-alt-screen rendering — incorrect answer
- **WHEN** `askModel` has `feedbackResult{correct: false, correctText: "X", feedback: "Y"}`
- **THEN** the rendered output contains `"✗ The answer was: X"` followed by the feedback paragraph

#### Scenario: Post-alt-screen rendering — skip
- **WHEN** `askModel` has `skipped = true`
- **THEN** the rendered output contains `"→ Question saved for later."`

#### Scenario: Post-alt-screen rendering — advisory
- **WHEN** `askModel` has `advisory = "caught up message"`
- **THEN** the rendered output is the advisory message verbatim

#### Scenario: q/Esc exit — no POST, no output
- **WHEN** `q` or `Esc` is pressed in `stateQuestion`
- **THEN** the model transitions to `stateDone`, no `postAnswer`/`postSkip` cmd is returned, and nothing is printed after alt-screen closes

---

### Requirement: Integration tests cover 4C HTTP and MCP endpoints
`integration_test.go` SHALL include tests verifying: `POST /recall/answer?feedback=true` returns `correct`/`correct_text`/`feedback` fields, evaluation correctness for right and wrong answers, 400 for out-of-range `answer_index`, 404 for unknown question ID, MCP `recall_next` returns `correct_index`, MCP `recall_answer` with `answer_index` returns correctness data, MCP `recall_answer` with `skip` returns `{"ok":true}`, and `recall://recent` includes enriched fields.

#### Scenario: POST /recall/answer?feedback=true returns verdict and feedback
- **WHEN** `POST /recall/answer?feedback=true` is called with `{"id":N,"answer_index":0}`
- **THEN** the response includes `"correct"`, `"correct_text"`, and `"feedback"` fields

#### Scenario: Correct answer evaluation
- **WHEN** `answer_index` matches the stored `correct_index`
- **THEN** the response has `"correct": true`

#### Scenario: Incorrect answer evaluation
- **WHEN** `answer_index` does not match `correct_index`
- **THEN** the response has `"correct": false`

#### Scenario: Out-of-range answer_index — 400
- **WHEN** `POST /recall/answer?feedback=true` is called with `answer_index` >= `len(choices)`
- **THEN** the response is `400 Bad Request` with `{"error":"answer_index out of range"}`

#### Scenario: Unknown question ID — 404
- **WHEN** `POST /recall/answer?feedback=true` is called with an ID that doesn't exist
- **THEN** the response is `404 Not Found`

#### Scenario: MCP recall_next returns correct_index
- **WHEN** an MCP client calls `recall_next` and a question is available
- **THEN** the response JSON includes `"correct_index"` with the correct 0-based index

#### Scenario: MCP recall_answer returns correctness data
- **WHEN** an MCP client calls `recall_answer` with `{"id":N,"answer_index":0}`
- **THEN** the response includes `"ok"`, `"correct"`, `"correct_index"`, and `"correct_text"`

#### Scenario: MCP recall_answer skip
- **WHEN** an MCP client calls `recall_answer` with `{"id":N,"skip":true}`
- **THEN** the response is `{"ok":true}` and no AI call is made

#### Scenario: recall://recent includes enriched fields
- **WHEN** questions have been answered and an MCP client reads `recall://recent`
- **THEN** each row includes `correct_index`, `answer_index`, `correct`, and `feedback` (null where appropriate)

---

### Requirement: Golden file test covers stateFeedback view
`golden_test.go` SHALL include a `TestGoldenAskFeedbackView` test that snapshots `askModel.View()` when `state = stateFeedback`. The golden file `testdata/TestGoldenAskFeedbackView.golden` SHALL be committed and marked `-text` in `.gitattributes`.

#### Scenario: stateFeedback golden snapshot
- **WHEN** `m.state = stateFeedback` and `m.View()` is called
- **THEN** the output matches `TestGoldenAskFeedbackView.golden` byte-for-byte

---

### Requirement: Recall package tests cover FeedbackRequest and GenerateFeedback
A new `recall_test.go` in `cmd/total-recall/` SHALL test `recall.FeedbackRequest` prompt construction (correct case, incorrect case, token budget) and `recall.Engine.GenerateFeedback` degradation (AI error → empty string, not propagated). Tests SHALL use a `mockProvider` struct implementing `ai.Provider` with canned responses.

#### Scenario: FeedbackRequest correct case annotations
- **WHEN** `FeedbackRequest` is called with `correctIndex == answerIndex`
- **THEN** the user turn contains `← correct, chosen` on the correct choice
- **AND** ends with `"The developer answered correctly."`

#### Scenario: FeedbackRequest incorrect case annotations
- **WHEN** `FeedbackRequest` is called with `correctIndex != answerIndex`
- **THEN** the user turn contains `← correct` on the correct choice and `← chosen (incorrect)` on the chosen choice
- **AND** ends with `"The developer chose option N and was incorrect."`

#### Scenario: FeedbackRequest token budget
- **WHEN** `FeedbackRequest` is called
- **THEN** `MaxTokens` is 150 and `JSON` is false

#### Scenario: GenerateFeedback degradation on AI error
- **WHEN** `GenerateFeedback` is called with a mock provider that returns an error
- **THEN** `GenerateFeedback` returns `"", nil` (not an error)
- **AND** the error is logged

---

### Requirement: ROADMAP reflects shipped and deferred phase status
`ROADMAP.md` SHALL mark Phase 4D as "(Shipped)" and move Phase 4B to a "(Deferred)" subsection with rationale explaining the deferral.

#### Scenario: Phase 4D status corrected
- **WHEN** `ROADMAP.md` is read
- **THEN** Phase 4D says "(Shipped)" not "(Planned)"

#### Scenario: Phase 4B deferred
- **WHEN** `ROADMAP.md` is read
- **THEN** Phase 4B says "(Deferred)" with explanation that it's moved out of Phase 4 scope
