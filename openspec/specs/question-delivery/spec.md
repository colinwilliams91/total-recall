## Purpose

Define the daemon HTTP endpoints the terminal (`tr ask`) and MCP paths use to fetch the next recall question, submit a selection or skip it, and report per-branch queued-question counts — all scoped to repo and branch with no global pool.

## Requirements

### Requirement: GET /recall/next returns the next pending question or 204, scoped to repo and branch, with the correctness key WITHHELD
`GET /recall/next` SHALL accept `repo` and `branch` query parameters (the absolute repository path from `git rev-parse --show-toplevel` and the branch name from `git rev-parse --abbrev-ref HEAD`). It SHALL call `store.NextQuestion(ctx, repo, branch, "shell")` where `repo` and `branch` are the query param values. Both parameters MUST be non-empty; if either is empty the handler SHALL respond `204 No Content` without calling the store (the client is outside a git worktree or in detached HEAD). When a question is available, it SHALL respond `200 OK` with `{"id":N,"question_type":"multiple_choice","question":"...","choices":["..."]}`. The correctness key (`correct_index` and `correct_answer`) SHALL NOT be included — it is withheld until the user submits via `POST /recall/select`. The `question_type` field is the discriminator that future multi-select and free-text clients dispatch on.

#### Scenario: Question available for the requested repo and branch
- **WHEN** `GET /recall/next?repo=/path/X&branch=feature-X` is called and a question with `repo=/path/X` AND `branch=feature-X` exists with `status = 'queued'`
- **THEN** the response is `200 OK` with `{"id":N,"question_type":"multiple_choice","question":"...","choices":["..."]}` (no `correct_index` field) and the row is atomically marked delivered (`status = 'delivered'`, a `'delivered'` event inserted)

#### Scenario: Empty repo or branch returns 204 without touching the store
- **WHEN** `GET /recall/next` is called without `repo` or `branch`, or with either empty
- **THEN** the handler responds `204 No Content` without calling `store.NextQuestion`; the daemon logs `[recall] next called with empty repo or branch — client is outside git or detached HEAD`

#### Scenario: Queue empty for the repo and branch
- **WHEN** `GET /recall/next?repo=/path/Y&branch=main` is called and no questions with `status = 'queued'` exist for `/path/Y` AND `main`
- **THEN** the response is `204 No Content`

#### Scenario: Concurrent callers race
- **WHEN** two callers invoke `GET /recall/next?repo=/path/X&branch=feature-X` simultaneously with one question queued for that repo and branch
- **THEN** exactly one receives `200 OK`; the other receives `204 No Content`

#### Scenario: Correctness key is not in the delivery response
- **WHEN** the delivery response body for a `multiple_choice` question is inspected
- **THEN** the JSON keys are `id`, `question_type`, `question`, `choices`; the key `correct_index` is absent

---

### Requirement: POST /recall/select evaluates correctness and optionally generates feedback (ID-keyed, no repo/branch filtering)
`POST /recall/select` SHALL accept a JSON body dispatched on `question_type`. For `multiple_choice` the body is `{"id":N,"selected_index":N,"skip":bool}`. The handler SHALL:
1. If `skip == true`: call `store.SkipQuestion(ctx, id)`, respond `{"ok":true,"skipped":true}`.
2. Otherwise: call `store.GetQuestion(ctx, id)`. If nil, respond `404`. If `selected_index` is nil or out of range, respond `400`.
3. Compute `correct = (selected_choice.IsCorrect)`, `correctIndex = question.CorrectIndex`, `correctText = choices[correctIndex].Text`. Persist the selection via `store.SubmitSelection(ctx, id, []int64{selectedChoice.ID}, feedback)`.
4. If `?feedback=true` query param is present: call `recallEngine.GenerateFeedback(...)`.
5. Respond `200 OK` with `{"ok":true,"correct":<bool>,"correct_index":<int>,"correct_answer":"<text>","feedback":"<text>"|null}` — send-both (the correctness key withheld at delivery is now revealed post-submission for stateless MCP consumers).

The legacy user-side field `answer_index` SHALL be rejected with `400 Bad Request` (lexical rule enforcement).

#### Scenario: Terminal select - correct, feedback returned
- **WHEN** `POST /recall/select?feedback=true` is called with `{"id": 1, "selected_index": 0}` and selected_index 0 is correct
- **THEN** the server evaluates `correct = true`, calls `GenerateFeedback`, persists the selection via `SubmitSelection`, and responds `{"ok":true,"correct":true,"correct_index":0,"correct_answer":"...","feedback":"<AI text>"}`

#### Scenario: Malformed body
- **WHEN** `POST /recall/select` is sent with non-JSON body
- **THEN** the response is `400 Bad Request`

#### Scenario: Lexical rule enforcement — "answer" banned on user side
- **WHEN** a request body contains an `answer_index` field
- **THEN** the response is `400 Bad Request` with `{"error":"answer_index is no longer accepted; use selected_index"}`

---

### Requirement: POST /recall/select skip path
When the request body contains `"skip": true`, the handler SHALL call `store.SkipQuestion(ctx, id)` and respond `{"ok":true,"skipped":true}`. No evaluation, no AI call, no `selections` row insert. ID-keyed — no `repo` or `branch` filtering applies. The skip lives in `questions.status = 'skipped'` (atomic with a `'skipped'` event in `question_events`), not in the prior `answer = 'skip'` magic-string sentinel.

#### Scenario: Skip submitted
- **WHEN** `POST /recall/select {"id":3,"skip":true}` is sent
- **THEN** the response is `200 OK {"ok":true,"skipped":true}` and the row has `status = 'skipped'` (no `selections` rows, one `'skipped'` event row)

---

### Requirement: POST /recall/select error handling
The handler SHALL respond `400 Bad Request` with `{"error":"selected_index out of range"}` when `selected_index` is negative or >= `len(choices)`. It SHALL respond `404 Not Found` when no question with the given ID exists. No DB write SHALL occur on either error path.

#### Scenario: Out-of-range selected index
- **WHEN** `POST /recall/select {"id":1,"selected_index":99}` is sent and the question has 3 choices
- **THEN** the response is `400 Bad Request` with `{"error":"selected_index out of range"}` and no DB write occurs

#### Scenario: Unknown question ID
- **WHEN** `POST /recall/select {"id":9999,"selected_index":0}` is sent and no question with that ID exists
- **THEN** the response is `404 Not Found` and no DB write occurs

---

### Requirement: Feedback AI call failure degrades gracefully
If `GenerateFeedback` returns an error or empty string, the handler SHALL still call `SubmitSelection` (the user's pick is recorded regardless of feedback failure) and respond `200 OK` with `"feedback":null`. Feedback failure MUST never block the selection from being stored. The `feedback` column on `questions` MAY be updated separately after-the-fact if feedback generation is retried asynchronously, but the synchronous submit path records the selection first.

#### Scenario: Feedback AI error - selection still recorded
- **WHEN** `POST /recall/select?feedback=true {"id":1,"selected_index":0}` is sent and `GenerateFeedback` returns an error
- **THEN** `SubmitSelection` is still called (selection persisted, `'answered'` event inserted, `status = 'answered'`) and the response is `200 OK` with `"feedback":null`

---

### Requirement: GET /recall/stale returns per-branch queued question counts
`GET /recall/stale?repo=<path>` SHALL respond `200 OK` with `{"repo":"<path>","branches":{"<branch>":<count>, ...}}` where each branch key is a non-empty branch name in the `questions` table with `status = 'queued' AND repo = ?`, and each value is the count of such queued questions for that branch. The `repo` query parameter MUST be non-empty; if empty, the handler SHALL respond `400 Bad Request` with `{"error":"repo query parameter is required"}`. Branches with zero queued questions SHALL be omitted from the result (only branches with `count > 0` appear). When no questions are queued for the repo, the response is `{"repo":"<path>","branches":{}}`. The predicate shifts from `delivered_at IS NULL` (prior implicit datetime state) to `status = 'queued'` (explicit enum cache column).

#### Scenario: Three branches with queued questions
- **WHEN** `GET /recall/stale?repo=/path/X` is called and `/path/X` has 3 queued questions on `feature-X`, 0 on `main`, and 2 on `bugfix-Y`
- **THEN** the response is `{"repo":"/path/X","branches":{"feature-X":3,"bugfix-Y":2}}` (`main` is omitted because its count is 0)

#### Scenario: No queued questions on any branch
- **WHEN** `GET /recall/stale?repo=/path/X` is called and no questions for `/path/X` have `status = 'queued'`
- **THEN** the response is `{"repo":"/path/X","branches":{}}`

#### Scenario: Missing repo parameter
- **WHEN** `GET /recall/stale` is called with no `repo` query parameter
- **THEN** the handler responds `400 Bad Request` with `{"error":"repo query parameter is required"}`
