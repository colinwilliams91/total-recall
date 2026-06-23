## Requirements

### Requirement: GET /recall/next returns the next pending question or 204
`GET /recall/next` SHALL call `store.NextQuestion(ctx, "shell")`. When a question is available, it SHALL respond `200 OK` with `Content-Type: application/json` and body `{"id":N,"question":"...","choices":["...","...","..."]}`. When no question is available, it SHALL respond `204 No Content` with an empty body. `correct_index` SHALL NOT be included in the terminal response — the terminal client cannot pre-evaluate.

#### Scenario: Question available
- **WHEN** `GET /recall/next` is called and a question exists with `delivered_at IS NULL`
- **THEN** the response is `200 OK` with the question JSON (`id`, `question`, `choices` — no `correct_index`) and the row is atomically marked delivered

#### Scenario: Queue empty
- **WHEN** `GET /recall/next` is called and no unclaimed questions exist
- **THEN** the response is `204 No Content`

#### Scenario: Concurrent callers race
- **WHEN** two callers invoke `GET /recall/next` simultaneously with one question in the queue
- **THEN** exactly one receives `200 OK` with the question; the other receives `204 No Content`

---

### Requirement: POST /recall/answer evaluates correctness and optionally generates feedback
`POST /recall/answer` SHALL accept a JSON body `{"id":N,"answer_index":N,"skip":bool}`. The handler SHALL:
1. If `skip == true`: call `store.SkipQuestion(ctx, id)`, respond `{"ok":true}`.
2. Otherwise: call `store.GetQuestion(ctx, id)`. If nil, respond `404`. If `answer_index` is nil or out of range, respond `400`.
3. Compute `correct = (answer_index == question.correct_index)`, `answerText = choices[answer_index]`, `correctText = choices[correct_index]`.
4. If `?feedback=true` query param is present: call `recallEngine.GenerateFeedback(...)`.
5. Call `store.AnswerQuestion(ctx, id, answer_index, answerText, correct, feedback)`.
6. Respond `200 OK` with `{"ok":true,"correct":<bool>,"correct_text":"<text>","feedback":"<text>"|null}`.

#### Scenario: Terminal answer — correct, feedback returned
- **WHEN** `POST /recall/answer?feedback=true` is called with `{"id": 1, "answer_index": 0}` and answer 0 is correct
- **THEN** the server evaluates `correct = true`, calls `GenerateFeedback`, stores all fields, and responds `{"ok":true,"correct":true,"correct_text":"...","feedback":"<AI text>"}`

#### Scenario: Terminal answer — incorrect, correct answer named in feedback
- **WHEN** `POST /recall/answer?feedback=true` is called with `{"id": 1, "answer_index": 1}` and correct_index is 0
- **THEN** `correct = false`, the AI prompt labels the correct and chosen choices, and the response includes `"correct":false`, `"correct_text":"..."`, `"feedback":"<explanation>"`

#### Scenario: No feedback when ?feedback=true is absent
- **WHEN** `POST /recall/answer` is called without the `?feedback=true` query parameter
- **THEN** `GenerateFeedback` is NOT called, `AnswerQuestion` is called with `feedback = ""`, and the response has `"feedback":null`

#### Scenario: Malformed body
- **WHEN** `POST /recall/answer` is sent with non-JSON body
- **THEN** the response is `400 Bad Request`

---

### Requirement: POST /recall/answer skip path
When the request body contains `"skip": true`, the handler SHALL call `store.SkipQuestion(ctx, id)` and respond `{"ok":true}`. No evaluation, no AI call, and `answer_index`, `correct`, `feedback` remain NULL in the DB.

#### Scenario: Skip submitted
- **WHEN** `POST /recall/answer {"id":3,"skip":true}` is sent
- **THEN** the response is `200 OK {"ok":true}` and the row has `answer = "skip"` with `answer_index`, `correct`, `feedback` NULL

---

### Requirement: POST /recall/answer error handling
The handler SHALL respond `400 Bad Request` with `{"error":"answer_index out of range"}` when `answer_index` is negative or >= `len(choices)`. It SHALL respond `404 Not Found` when no question with the given ID exists. No DB write SHALL occur on either error path.

#### Scenario: Invalid answer_index — 400
- **WHEN** `POST /recall/answer?feedback=true` is called with `{"id": 1, "answer_index": 5}` and the question has 3 choices
- **THEN** the response is `400 Bad Request` with `{"error":"answer_index out of range"}` and no DB write occurs

#### Scenario: Unknown question ID — 404
- **WHEN** `POST /recall/answer?feedback=true` is called with `{"id": 99999, "answer_index": 0}` and no question 99999 exists
- **THEN** the response is `404 Not Found`

---

### Requirement: Feedback AI call failure degrades gracefully
If `GenerateFeedback` returns an error or empty string, the handler SHALL still call `AnswerQuestion` with `feedback = ""` and respond `200 OK` with `"feedback":null`. The answer is recorded regardless of feedback failure. The HTTP status SHALL NOT reflect the AI failure.

#### Scenario: AI provider fails during feedback
- **WHEN** `POST /recall/answer?feedback=true` is called and the AI provider returns an error
- **THEN** `GenerateFeedback` logs the error and returns `""`
- **AND** `AnswerQuestion` is still called, the response is `200 OK` with `"feedback":null`
