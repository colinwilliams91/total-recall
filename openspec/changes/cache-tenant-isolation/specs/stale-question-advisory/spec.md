## ADDED Requirements

### Requirement: Endpoint GET /recall/stale reports per-branch undelivered question counts
The daemon SHALL expose `GET /recall/stale?repo=<path>` as a new route registered in `RegisterRoutes()`. It SHALL respond `200 OK` with a JSON body `{"repo":"<path>","branches":{"<branch>":<count>, ...}}` where each branch key is a distinct `branch` value in the `questions` table with `delivered_at IS NULL AND repo = ?`, and each value is the count of undelivered questions for that branch.

The `repo` query parameter is REQUIRED. If absent or empty, the handler SHALL respond `400 Bad Request` with `{"error":"repo query parameter is required"}`.

Branches with zero undelivered questions SHALL be omitted from the result (only branches with `count > 0` appear). When no questions are undelivered for the repo, the response is `{"repo":"<path>","branches":{}}`.

The query SHALL be `SELECT branch, COUNT(*) FROM questions WHERE delivered_at IS NULL AND repo = ? GROUP BY branch` (or equivalent). Only questions with non-empty `branch` columns contribute — `branch = ""` rows (which should not exist per the Question Store spec, but are defensively excluded) are not counted.

#### Scenario: Three branches with pending questions
- **WHEN** `GET /recall/stale?repo=/path/X` is called and `/path/X` has 3 undelivered questions on `feature-X`, 0 on `main`, and 2 on `bugfix-Y`
- **THEN** the response is `{"repo":"/path/X","branches":{"feature-X":3,"bugfix-Y":2}}` (`main` is omitted because its count is 0)

#### Scenario: No pending questions on any branch
- **WHEN** `GET /recall/stale?repo=/path/X` is called and no questions for `/path/X` have `delivered_at IS NULL`
- **THEN** the response is `{"repo":"/path/X","branches":{}}`

#### Scenario: Missing repo parameter
- **WHEN** `GET /recall/stale` is called with no `repo` query parameter
- **THEN** the handler responds `400 Bad Request` with `{"error":"repo query parameter is required"}`

#### Scenario: Repo with no questions at all
- **WHEN** `GET /recall/stale?repo=/never/used` is called and no questions exist for that repo
- **THEN** the response is `200 OK` with `{"repo":"/never/used","branches":{}}`

---

### Requirement: `tr status` queries /recall/stale and prints a per-branch advisory
`cmd/.../main.go`'s `runStatus()` function SHALL call `hooks.FindRepoRoot()` to resolve the current repo. If not in a git repo, the stale-questions advisory block SHALL be skipped silently (no error, no output). If in a git repo, it SHALL call `GET /recall/stale?repo=<resolved-repo>` and, on success (200 OK), inspect the `branches` object in the response. For each branch with `count > 0`, it SHALL print:

```
⚠  <count> recall question(s) pending on branch <branch>
   Switch back: git switch <branch> && tr ask
```

If the `branches` object is empty, no stale-questions advisory SHALL be printed. If the daemon is not running (status check already exits non-zero for that case), the advisory block is also skipped — the stale-questions advisory is additive to the existing daemon-down exit behavior.

The advisory block SHALL print AFTER the existing daemon-status and config-show blocks, so the overall output of `tr status` becomes: daemon-up confirmation → fully-resolved config → stale-questions advisory.

#### Scenario: User on main, questions pending on feature-X
- **WHEN** `tr status` is run inside `/path/X` on branch `main`, and `/path/X` has 3 undelivered questions on `feature-X`
- **THEN** the output includes a line `⚠  3 recall question(s) pending on branch feature-X` followed by the suggested switch-back command

#### Scenario: No pending questions anywhere
- **WHEN** `tr status` is run inside `/path/X` on branch `main`, and no questions for `/path/X` have `delivered_at IS NULL`
- **THEN** no stale-questions advisory is printed

#### Scenario: tr status outside a git repo
- **WHEN** `tr status` is run from a directory that is not inside a git repository
- **THEN** the stale-questions advisory block is skipped silently; the daemon-health and config-show blocks still print

#### Scenario: Daemon down
- **WHEN** `tr status` is run and the health check at `/health` fails (daemon not running)
- **THEN** `runStatus` exits non-zero as before; the stale-questions advisory is not reached (no point in querying `/recall/stale` if the daemon is down)

---

### Requirement: Future anchor - migrate-vs-answer UX is deferred
This requirement exists to document a future-phase enhancement that builds on the stale-question advisory. It is NOT to be implemented in the `cache-tenant-isolation` phase.

A future phase MAY introduce a `tr migrate --from <branch> --to <branch>` command that updates the `branch` column on pending (unclaimed) questions in the cache, effectively re-queueing them under a different branch (typically from a feature branch into `main` after a merge). It MAY also introduce a `tr ask --branch <branch>` flag that allows in-place answering of a different branch's pending questions without switching branches first.

A future phase MAY also introduce a `post-checkout` Git hook type that fires on `git switch`/`git checkout`, queries `/recall/stale` for the branch being left, and either (a) prints an advisory warning the user that questions remain pending, or (b) inline-triggers `tr ask --branch=<left-branch>` to deliver them before the switch completes.

The Phase Y1 `cache-tenant-isolation` lays the detection foundation (the `/recall/stale` endpoint + the `tr status` advisory); future phases own the actual delivery/migration actions.

#### Scenario: Deferral is documented
- **WHEN** a future phase author scans this spec for `migrate-vs-answer`
- **THEN** they find this requirement block and understand the deferred scope (CLI migrate command, `tr ask --branch`, post-checkout hook) — none of which are to be built in Phase Y1.