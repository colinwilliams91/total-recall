## MODIFIED Requirements

### Requirement: GET /recall/next returns the next pending question or 204, scoped to repo and branch, with the correctness key WITHHELD
`GET /recall/next` SHALL accept `repo` and `branch` query parameters (the absolute repository path from `git rev-parse --show-toplevel` and the branch name from `git rev-parse --abbrev-ref HEAD`). It SHALL call `store.NextQuestion(ctx, repo, branch, "shell")` where `repo` and `branch` are the query param values. Both parameters MUST be non-empty; if either is empty the handler SHALL respond `204 No Content` without calling the store (the client is outside a git worktree or in detached HEAD). When a question is available, it SHALL respond `200 OK` with `{"id":N,"question_type":"multiple_choice","question":"...","choices":["..."]}`. `correct_index` (and per-type equivalents) SHALL NOT be included — the correctness key crosses the wire only in the submit-response, never at delivery. This fixes the leak where a client could inspect delivery JSON to know which choice is correct before submitting. The `question_type` field enables future multi-select and free-text clients to dispatch rendering without a separate schema probe.

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
The prior `POST /recall/answer` endpoint is renamed to `POST /recall/select`. `POST /recall/select` SHALL accept a JSON body dispatched on `question_type`:

- `multiple_choice`: `{"id":N,"selected_index":N,"skip":bool}` (single-select)
- `multi_select` (when it lands): `{"id":N,"selected_indices":[N,...],"skip":bool}`
- `free_text` (when it lands): `{"id":N,"selected_text":"...","skip":bool}`

The handler SHALL:
1. If `skip == true`: call `store.SkipQuestion(ctx, id)`, respond `{"ok":true,"skipped":true}`.
2. Otherwise (single-select MC for now): call `store.GetQuestion(ctx, id)`. If nil, respond `404`. If `selected_index` is nil or out of range, respond `400`.
3. Compute `correct = (selected_index == question.correct_index)`, `answerText = choices[selected_index]`, `correctText = choices[correct_index]` (where `question.correct_index` is the derived position of the `choices.is_correct = 1` row). Persist the selection via `store.SubmitSelection(ctx, id, []int64{choiceID at selected_index})`.
4. If `?feedback=true` query param is present: call `recallEngine.GenerateFeedback(...)`.
5. Respond `200 OK` with `{"ok":true,"correct":<bool>,"correct_index":<int>,"correct_answer":"<text>","feedback":"<text>"|null}` — both `correct_index` (the engine's key identifier) and `correct_answer` (the engine's key text) are sent (send-both) to support stateless MCP consumers generating feedback without cross-call state.

#### Scenario: Terminal select - correct, feedback returned
- **WHEN** `POST /recall/select?feedback=true` is called with `{"id": 1, "selected_index": 0}` and selected_index 0 is correct
- **THEN** the server evaluates `correct = true`, calls `GenerateFeedback`, persists the selection via `SubmitSelection`, and responds `{"ok":true,"correct":true,"correct_index":0,"correct_answer":"...","feedback":"<AI text>"}`

#### Scenario: Malformed body
- **WHEN** `POST /recall/select` is sent with non-JSON body
- **THEN** the response is `400 Bad Request`

#### Scenario: Lexical rule enforcement — "answer" banned on user side
- **WHEN** a request body contains an `answer_index` field (the prior user-side name)
- **THEN** the field is ignored or rejected; the user-side field name is `selected_index` exclusively

---

### Requirement: POST /recall/select skip path
When the request body contains `"skip": true`, the handler SHALL call `store.SkipQuestion(ctx, id)` and respond `{"ok":true,"skipped":true}`. No evaluation, no AI call, no `selections` row insert. ID-keyed — no `repo` or `branch` filtering applies. The skip lives in `questions.status = 'skipped'` (atomic with a `'skipped'` event in `question_events`), not in the prior `answer = 'skip'` magic-string sentinel.

#### Scenario: Skip submitted
- **WHEN** `POST /recall/select {"id":3,"skip":true}` is sent
- **THEN** the response is `200 OK {"ok":true,"skipped":true}` and the row has `status = 'skipped'` (no `selections` rows, one `'skipped'` event row)

---

### Requirement: POST /recall/select error handling
The handler SHALL respond `400 Bad Request` with `{"error":"selected_index out of range"}` when `selected_index` is negative or >= `len(choices)`. It SHALL respond `404 Not Found` when no question with the given ID exists. No DB write SHALL occur on either error path — no `selections` row, no `'answered'` event, no `status` transition.

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

## REMOVED Requirements

### Requirement: POST /recall/answer evaluates correctness and optionally generates feedback (ID-keyed, no repo/branch filtering)
**Reason**: Renamed to `POST /recall/select` per the lexical rule: "answer" is banned on the user side of the wire (verb ambiguity and collision with the engine's key concept). Body field `answer_index` is banned and replaced with `selected_index`. Submit-response gains `correct_answer` (text) alongside `correct_index` (int) for stateless MCP consumers (send-both). The skip path's prior `answer = 'skip'` sentinel becomes `status = 'skipped'` at the storage layer.
**Migration**: Hard break — no deprecation window (0 live users). Clients of `POST /recall/answer` reject the new path; the daemon returns `404` for `/recall/answer` after the rename. TUI, hooks, and MCP all update atomically in this change. See MODIFIED `POST /recall/select` requirement above.

### Requirement: POST /recall/answer skip path
**Reason**: The skip path moves to `POST /recall/select` with the same `skip: true` body field, but the response shape changes from `{"ok":true}` to `{"ok":true,"skipped":true}` and the storage encoding changes from `answer = 'skip'` magic string to `status = 'skipped'` enum column atomic with a `'skipped'` event row.
**Migration**: See REMOVED `POST /recall/answer` above; the skip path lives under `/recall/select` now.

### Requirement: POST /recall/answer error handling
**Reason**: The error response field name changes from `"answer_index out of range"` to `"selected_index out of range"` to match the renamed user-side field. The error paths (400 for out-of-range, 404 for unknown ID) are unchanged in semantics.
**Migration**: See MODIFIED `POST /recall/select error handling` above; clients checking the error string for `"answer_index out of range"` must update to `"selected_index out of range"`.