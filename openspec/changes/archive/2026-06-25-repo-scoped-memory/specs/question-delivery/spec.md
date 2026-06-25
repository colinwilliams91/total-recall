## MODIFIED Requirements

### Requirement: GET /recall/next returns the next pending question or 204, scoped to repo
`GET /recall/next` SHALL accept an optional `repo` query parameter (the absolute repository path). It SHALL call `store.NextQuestion(ctx, repo, "shell")` where `repo` is the query param value or `""` when absent. When a question is available, it SHALL respond `200 OK` with `{"id":N,"question":"...","choices":["..."]}`. When no question is available, it SHALL respond `204 No Content`. `correct_index` SHALL NOT be included.

#### Scenario: Question available for the requested repo
- **WHEN** `GET /recall/next?repo=/path/X` is called and a question with `repo=/path/X` exists with `delivered_at IS NULL`
- **THEN** the response is `200 OK` with the question JSON and the row is atomically marked delivered

#### Scenario: No repo param — global dequeue
- **WHEN** `GET /recall/next` is called without a `repo` query parameter
- **THEN** the handler dequeues from the global pool (`repo=""`)

#### Scenario: Queue empty for the repo
- **WHEN** `GET /recall/next?repo=/path/Y` is called and no unclaimed questions exist for `/path/Y`
- **THEN** the response is `204 No Content`; the daemon logs an advisory if `/path/Y` is non-empty and no rows match (possible repo move)

#### Scenario: Concurrent callers race
- **WHEN** two callers invoke `GET /recall/next?repo=/path/X` simultaneously with one question in the queue for `/path/X`
- **THEN** exactly one receives `200 OK`; the other receives `204 No Content`

---

### Requirement: POST /recall/answer evaluates correctness and optionally generates feedback
`POST /recall/answer` SHALL accept a JSON body `{"id":N,"answer_index":N,"skip":bool}` and an optional `repo` query parameter. The `repo` parameter is accepted for symmetry but is not strictly required for correctness — `AnswerQuestion` and `SkipQuestion` are ID-keyed (the ID is globally unique). The handler SHALL:
1. If `skip == true`: call `store.SkipQuestion(ctx, id)`, respond `{"ok":true}`.
2. Otherwise: call `store.GetQuestion(ctx, id)`. If nil, respond `404`. If `answer_index` is nil or out of range, respond `400`.
3. Compute `correct = (answer_index == question.correct_index)`, `answerText = choices[answer_index]`, `correctText = choices[correct_index]`.
4. If `?feedback=true` query param is present: call `recallEngine.GenerateFeedback(...)`.
5. Call `store.AnswerQuestion(ctx, id, answer_index, answerText, correct, feedback)`.
6. Respond `200 OK` with `{"ok":true,"correct":<bool>,"correct_text":"<text>","feedback":"<text>"|null}`.

#### Scenario: Terminal answer — correct, feedback returned
- **WHEN** `POST /recall/answer?feedback=true` is called with `{"id": 1, "answer_index": 0}` and answer 0 is correct
- **THEN** the server evaluates `correct = true`, calls `GenerateFeedback`, stores all fields, and responds `{"ok":true,"correct":true,"correct_text":"...","feedback":"<AI text>"}`

#### Scenario: Malformed body
- **WHEN** `POST /recall/answer` is sent with non-JSON body
- **THEN** the response is `400 Bad Request`

---

### Requirement: POST /recall/answer skip path
When the request body contains `"skip": true`, the handler SHALL call `store.SkipQuestion(ctx, id)` and respond `{"ok":true}`. No evaluation, no AI call.

#### Scenario: Skip submitted
- **WHEN** `POST /recall/answer {"id":3,"skip":true}` is sent
- **THEN** the response is `200 OK {"ok":true}` and the row has `answer = "skip"`

---

### Requirement: POST /recall/answer error handling
The handler SHALL respond `400 Bad Request` with `{"error":"answer_index out of range"}` when `answer_index` is negative or >= `len(choices)`. It SHALL respond `404 Not Found` when no question with the given ID exists. No DB write SHALL occur on either error path.

---

### Requirement: Feedback AI call failure degrades gracefully
If `GenerateFeedback` returns an error or empty string, the handler SHALL still call `AnswerQuestion` with `feedback = ""` and respond `200 OK` with `"feedback":null`. The answer is recorded regardless of feedback failure.
