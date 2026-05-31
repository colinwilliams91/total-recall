## Requirements

### Requirement: GET /recall/next returns the next pending question or 204
`GET /recall/next` SHALL call `store.NextQuestion(ctx, "shell")`. When a question is available, it SHALL respond `200 OK` with `Content-Type: application/json` and body `{"id":N,"question":"...","choices":["...","...","..."]}`. When no question is available, it SHALL respond `204 No Content` with an empty body.

#### Scenario: Question available
- **WHEN** `GET /recall/next` is called and a question exists with `delivered_at IS NULL`
- **THEN** the response is `200 OK` with the question JSON and the row is atomically marked delivered

#### Scenario: Queue empty
- **WHEN** `GET /recall/next` is called and no unclaimed questions exist
- **THEN** the response is `204 No Content`

#### Scenario: Concurrent callers race
- **WHEN** two callers invoke `GET /recall/next` simultaneously with one question in the queue
- **THEN** exactly one receives `200 OK` with the question; the other receives `204 No Content`

---

### Requirement: POST /recall/answer records a user response
`POST /recall/answer` SHALL accept a JSON body `{"id":N,"answer":"..."}`. It SHALL call `store.AnswerQuestion(ctx, id, answer)` and respond `200 OK` with `{"ok":true}`. On malformed body, it SHALL respond `400 Bad Request`.

#### Scenario: Valid answer submitted
- **WHEN** `POST /recall/answer {"id":3,"answer":"Prevent retry synchronization"}` is sent
- **THEN** the response is `200 OK {"ok":true}` and the row has `answer` and `answered_at` set

#### Scenario: Skip submitted
- **WHEN** `POST /recall/answer {"id":3,"answer":"skip"}` is sent
- **THEN** the response is `200 OK {"ok":true}` and the row has `answer = "skip"`

#### Scenario: Malformed body
- **WHEN** `POST /recall/answer` is sent with non-JSON body
- **THEN** the response is `400 Bad Request`
