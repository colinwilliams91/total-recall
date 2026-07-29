## MODIFIED Requirements

### Requirement: GET /recall/next returns the next pending question or 204, scoped to repo and branch
`GET /recall/next` SHALL accept `repo` and `branch` query parameters (the absolute repository path from `git rev-parse --show-toplevel` and the branch name from `git rev-parse --abbrev-ref HEAD`). It SHALL call `store.NextQuestion(ctx, repo, branch, "shell")` where `repo` and `branch` are the query param values. Both parameters MUST be non-empty; if either is empty the handler SHALL respond `204 No Content` without calling the store (the client is outside a git worktree or in detached HEAD). When a question is available, it SHALL respond `200 OK` with `{"id":N,"question":"...","choices":["..."]}`. `correct_index` SHALL NOT be included.

#### Scenario: Question available for the requested repo and branch
- **WHEN** `GET /recall/next?repo=/path/X&branch=feature-X` is called and a question with `repo=/path/X` AND `branch=feature-X` exists with `delivered_at IS NULL`
- **THEN** the response is `200 OK` with the question JSON and the row is atomically marked delivered

#### Scenario: Empty repo or branch returns 204 without touching the store
- **WHEN** `GET /recall/next` is called without `repo` or `branch`, or with either empty
- **THEN** the handler responds `204 No Content` without calling `store.NextQuestion`; the daemon logs `[recall] next called with empty repo or branch — client is outside git or detached HEAD`

#### Scenario: Queue empty for the repo and branch
- **WHEN** `GET /recall/next?repo=/path/Y&branch=main` is called and no unclaimed questions exist for `/path/Y` AND `main`
- **THEN** the response is `204 No Content`

#### Scenario: Concurrent callers race
- **WHEN** two callers invoke `GET /recall/next?repo=/path/X&branch=feature-X` simultaneously with one question in the queue for that repo and branch
- **THEN** exactly one receives `200 OK`; the other receives `204 No Content`

---

### Requirement: GET /recall/stale returns per-branch undelivered question counts
`GET /recall/stale?repo=<path>` SHALL respond `200 OK` with `{"repo":"<path>","branches":{"<branch>":<count>, ...}}` where each branch key is a non-empty branch name in the `questions` table with `delivered_at IS NULL AND repo = ?`, and each value is the count of such undelivered questions for that branch. The `repo` query parameter MUST be non-empty; if empty, the handler SHALL respond `400 Bad Request` with `{"error":"repo query parameter is required"}`. Branches with zero undelivered questions SHALL be omitted from the result (only branches with `count > 0` appear). When no questions are undelivered for the repo, the response is `{"repo":"<path>","branches":{}}`.

#### Scenario: Three branches with pending questions
- **WHEN** `GET /recall/stale?repo=/path/X` is called and `/path/X` has 3 undelivered questions on `feature-X`, 0 on `main`, and 2 on `bugfix-Y`
- **THEN** the response is `{"repo":"/path/X","branches":{"feature-X":3,"bugfix-Y":2}}` (`main` is omitted because its count is 0)

#### Scenario: No pending questions on any branch
- **WHEN** `GET /recall/stale?repo=/path/X` is called and no questions for `/path/X` have `delivered_at IS NULL`
- **THEN** the response is `{"repo":"/path/X","branches":{}}`

#### Scenario: Missing repo parameter
- **WHEN** `GET /recall/stale` is called with no `repo` query parameter
- **THEN** the handler responds `400 Bad Request` with `{"error":"repo query parameter is required"}`

---

### Requirement: POST /recall/answer evaluates correctness and optionally generates feedback (ID-keyed, no repo/branch filtering)
`POST /recall/answer` SHALL accept a JSON body `{"id":N,"answer_index":N,"skip":bool}` and an optional `repo` query parameter (kept for symmetry only). The `repo` and `branch` parameters SHALL NOT be used for filtering — `AnswerQuestion`, `SkipQuestion`, and `GetQuestion` are ID-keyed and globally unique. The handler SHALL:
1. If `skip == true`: call `store.SkipQuestion(ctx, id)`, respond `{"ok":true}`.
2. Otherwise: call `store.GetQuestion(ctx, id)`. If nil, respond `404`. If `answer_index` is nil or out of range, respond `400`.
3. Compute `correct = (answer_index == question.correct_index)`, `answerText = choices[answer_index]`, `correctText = choices[correct_index]`.
4. If `?feedback=true` query param is present: call `recallEngine.GenerateFeedback(...)`.
5. Call `store.AnswerQuestion(ctx, id, answer_index, answerText, correct, feedback)`.
6. Respond `200 OK` with `{"ok":true,"correct":<bool>,"correct_text":"<text>","feedback":"<text>"|null}`.

#### Scenario: Terminal answer - correct, feedback returned
- **WHEN** `POST /recall/answer?feedback=true` is called with `{"id": 1, "answer_index": 0}` and answer 0 is correct
- **THEN** the server evaluates `correct = true`, calls `GenerateFeedback`, stores all fields, and responds `{"ok":true,"correct":true,"correct_text":"...","feedback":"<AI text>"}`

#### Scenario: Malformed body
- **WHEN** `POST /recall/answer` is sent with non-JSON body
- **THEN** the response is `400 Bad Request`

---

### Requirement: POST /recall/answer skip path
When the request body contains `"skip": true`, the handler SHALL call `store.SkipQuestion(ctx, id)` and respond `{"ok":true}`. No evaluation, no AI call. ID-keyed — no `repo` or `branch` filtering applies.

#### Scenario: Skip submitted
- **WHEN** `POST /recall/answer {"id":3,"skip":true}` is sent
- **THEN** the response is `200 OK {"ok":true}` and the row has `answer = "skip"`

---

### Requirement: POST /recall/answer error handling
The handler SHALL respond `400 Bad Request` with `{"error":"answer_index out of range"}` when `answer_index` is negative or >= `len(choices)`. It SHALL respond `404 Not Found` when no question with the given ID exists. No DB write SHALL occur on either error path.

#### Scenario: Out-of-range answer index
- **WHEN** `POST /recall/answer {"id":1,"answer_index":99}` is sent and the question has 3 choices
- **THEN** the response is `400 Bad Request` with `{"error":"answer_index out of range"}` and no DB write occurs

#### Scenario: Unknown question ID
- **WHEN** `POST /recall/answer {"id":9999,"answer_index":0}` is sent and no question with that ID exists
- **THEN** the response is `404 Not Found` and no DB write occurs

---

### Requirement: Feedback AI call failure degrades gracefully
If `GenerateFeedback` returns an error or empty string, the handler SHALL still call `AnswerQuestion` with `feedback = ""` and respond `200 OK` with `"feedback":null`. The answer is recorded regardless of feedback failure.

#### Scenario: Feedback AI error - answer still recorded
- **WHEN** `POST /recall/answer?feedback=true {"id":1,"answer_index":0}` is sent and `GenerateFeedback` returns an error
- **THEN** `AnswerQuestion` is still called with `feedback = ""` and the response is `200 OK` with `"feedback":null`