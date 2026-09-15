## MODIFIED Requirements

### Requirement: GET /recall/next returns the next pending question or 204, scoped to repo and branch, with the correctness key WITHHELD
`GET /recall/next` SHALL accept `repo` and `branch` query parameters (the absolute repository path from `git rev-parse --show-toplevel` and the branch name from `git rev-parse --abbrev-ref HEAD`). It SHALL call `store.NextQuestion(ctx, repo, branch, "shell")` where `repo` and `branch` are the query param values. Both parameters MUST be non-empty; if either is empty the handler SHALL respond `204 No Content` without calling the store (the client is outside a git worktree or in detached HEAD). When a question is available, it SHALL respond `200 OK` with `{"id":N,"question_type":"multiple_choice","question":"...","choices":["..."]}`. The correctness key (`correct_index` and `correct_answer`) SHALL NOT be included — it is withheld until the user submits via `POST /recall/select`. The `question_type` field is the discriminator that future multi-select clients dispatch on.

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
