## 1. Schema migration strip + branch-column addition in store.go

- [x] 1.1 In `internal/cache/store.go`, add `branch TEXT NOT NULL` (NO `DEFAULT ''`) to the `concepts` `CREATE TABLE` statement (line ~22-29). Add the same column to the `questions` `CREATE TABLE` statement (line ~33-49).
- [x] 1.2 Replace index `idx_concepts_repo_seen ON concepts(repo, seen_at DESC)` with `idx_concepts_repo_branch_seen ON concepts(repo, branch, seen_at DESC)`. (The single-column `idx_concepts_seen_at` was also stripped because the covering `(repo, branch, seen_at)` index subsumes it.)
- [x] 1.3 Replace index `idx_questions_repo_q ON questions(repo, queued_at ASC) WHERE delivered_at IS NULL` with `idx_questions_repo_branch_q ON questions(repo, branch, queued_at ASC) WHERE delivered_at IS NULL`.
- [x] 1.4 Stripped the entire `existingMigrations` block in `Open()` (old store.go:111-122).
- [x] 1.5 Stripped the entire `repo` migration block in `Open()` (old store.go:124-145) including the `purged` flag and log line.
- [x] 1.6 Stripped the post-migration index-creation block (old store.go:147-157). Replacement indexes are now created directly after the `CREATE TABLE` statements.
- [x] 1.7 Deleted the `addColumnIfMissing` helper. `grep` confirmed zero real callers; the only remaining hit was a comment in `cmd/tr/cache_test.go:731` referring to the old migration. Also removed the now-unused `strings` import.
- [x] 1.8 `cache.Open()` is now: `trDir` → `sql.Open` → `SetMaxOpenConns(1)` → `CREATE TABLE IF NOT EXISTS concepts` → `CREATE TABLE IF NOT EXISTS questions` → `CREATE INDEX IF NOT EXISTS idx_concepts_repo_branch_seen` → `CREATE INDEX IF NOT EXISTS idx_questions_repo_branch_q` → return `&Store{db: db}, nil`. No migrations, no `addColumnIfMissing` calls, no `purged` log lines.

## 2. Store method signature changes (add branch, reject empty)

- [x] 2.1 `Store.Save(ctx, repo, branch, concepts)` — added `branch string` param; insert now uses `(?, ?, ?, ?, ?, ?)` for `concept, source, weight, repo, branch, seen_at`. Guard at top: returns `nil` and logs `[store] skipping insert: empty repo or branch` when either is empty.
- [x] 2.2 `Store.Recent(ctx, repo, branch, n)` — added `branch string` param; query now `WHERE repo = ? AND branch = ?`. Guard: returns `(nil, nil)` and logs `[store] skipping recent: empty repo or branch` when either is empty.
- [x] 2.3 `Store.SaveQuestion(ctx, repo, branch, question, choices, correctIndex)` — added `branch string` param; insert now `VALUES (?, ?, ?, ?, ?)`. Guard: returns `nil` and logs `[store] skipping savequestion: empty repo or branch` when either is empty.
- [x] 2.4 `Store.NextQuestion(ctx, repo, branch, claimedBy)` — added `branch string` param; the `UPDATE...RETURNING` inner `SELECT` now `AND branch = ?`. Guard: returns `(nil, nil)` without logging when either is empty.
- [x] 2.5 `Store.QueueDepth(ctx, repo, branch)` — added `branch string` param; query now `WHERE delivered_at IS NULL AND repo = ? AND branch = ?`. Guard: returns `(0, nil)` without logging when either is empty.
- [x] 2.6 `Store.PeekNextQuestion(ctx, repo, branch)` — added `branch string` param; query now `WHERE delivered_at IS NULL AND repo = ? AND branch = ?`. Guard: returns `(nil, nil)` without logging when either is empty.
- [x] 2.7 `Store.RecentAnswered(ctx, repo, branch, limit)` — added `branch string` param; query now `WHERE answered_at IS NOT NULL AND repo = ? AND branch = ?`. Guard: returns `(nil, nil)` without logging when either is empty.
- [x] 2.8 `Store.AnswerQuestion`, `Store.GetQuestion`, `Store.SkipQuestion` — **NO CHANGE**. They remain ID-keyed. Existing comments "ID-keyed globally unique — no repo parameter needed" are unchanged.

## 3. Recall engine signature change

- [x] 3.1 `internal/recall/engine.go`'s `Engine.Synthesize` signature now is `Synthesize(ctx, repo, branch, difficulty, model)`. Passes `branch` to `e.store.Recent(ctx, repo, branch, 20)`.
- [x] 3.2 Guard at the top of `Synthesize`: returns `(nil, nil)` without calling the store or provider when either is empty.

## 4. Daemon handler changes (server.go)

- [x] 4.1 `runPipeline` (server.go:127) — after the `payload.Diff == ""` early-return, added `if env.Branch == "" { log.Printf("[pipeline] skipping insert: detached HEAD (no branch)"); return }`.
- [x] 4.2 `runPipeline` — `s.store.Save(ctx, env.Repo, env.Branch, fingerprints)`.
- [x] 4.3 `runPipeline` — `s.recallEngine.Synthesize(ctx, env.Repo, env.Branch, "", s.cfg.AI.Model)`.
- [x] 4.4 `runPipeline` — `s.store.SaveQuestion(ctx, env.Repo, env.Branch, q.Question, q.Choices, q.CorrectIndex)`.
- [x] 4.5 `handleRecallNext` — reads `branch` query param; returns `204 No Content` without touching the store when either `repo` or `branch` is empty; otherwise calls `s.store.NextQuestion(r.Context(), repo, branch, "shell")`.
- [x] 4.6 `handleRecallAnswer` — **NO CHANGE** to the answer/eval logic (ID-keyed). The `repo` query param is already accepted for symmetry. `branch` MAY be passed by the client and is accepted (it is captured by `r.URL.Query().Get("branch")` if present) but is unused on the answer path.

## 5. New endpoint /recall/stale (server.go)

- [x] 5.1 Added `s.mux.HandleFunc("GET /recall/stale", s.handleRecallStale)` to `RegisterRoutes()`.
- [x] 5.2 Added `handleRecallStale` method. Returns `400 Bad Request` with `{"error":"repo query parameter is required"}` when `repo` is empty. On success, returns `200 OK` with `{"repo":"<path>","branches":{...}}`. The actual `SELECT branch, COUNT(*) FROM questions WHERE delivered_at IS NULL AND repo = ? AND branch != '' GROUP BY branch` query lives on the store as `Store.StalePerBranch(ctx, repo) (map[string]int, error)` so the handler stays thin and the query is unit-testable.

## 6. MCP layer changes (mcp.go)

- [x] 6.1 Added `Branch *string "json:\"branch,omitempty\"` field to `recallNextIn`, `recallAnswerIn`, `recallStatusIn` structs.
- [x] 6.2 Added `branchFromToolInput(b *string) string` helper mirroring `repoFromToolInput`.
- [x] 6.3 Added `branchFromResourceURI(uri string) string` helper mirroring `repoFromResourceURI`. Extracts `?branch=` from URIs.
- [x] 6.4 `recall_next` handler calls `branch := branchFromToolInput(in.Branch)` and passes `store.NextQuestion(ctx, repo, branch, "mcp")`. If either is empty, the store returns `nil, nil` and the tool returns `{"question":null}` automatically.
- [x] 6.5 `recall_status` handler calls `branch := branchFromToolInput(in.Branch)` and passes `store.QueueDepth(ctx, repo, branch)`. If either is empty, `QueueDepth` returns `0` automatically.
- [x] 6.6 `recall_answer` handler — NO CHANGE to the evaluation logic; `Branch` field is accepted in the input struct for symmetry but is unused on the answer path (ID-keyed).
- [x] 6.7 `queueResourceHandler` calls `branch := branchFromResourceURI(req.Params.URI)` and passes to `store.QueueDepth` and `store.PeekNextQuestion`. If either is empty, the store methods return zero/nil and the response shows `{"depth":0,"next":null}` automatically.
- [x] 6.8 `recentResourceHandler` calls `branch := branchFromResourceURI(req.Params.URI)` and passes to `store.RecentAnswered`. If either is empty, store returns nil and the response shows `[]`.
- [x] 6.9 Resource templates updated: `URITemplate: "recall://queue{?repo}{&branch}"` and `URITemplate: "recall://recent{?repo}{&branch}"`. Plain (non-template) `recall://queue` and `recall://recent` resources keep their existing URIs as canonical identity for subscription.
- [x] 6.10 `recallWorkflowInstructions` updated. Now instructs the agent to pass `repo` from `git rev-parse --show-toplevel` AND `branch` from `git rev-parse --abbrev-ref HEAD`. Notes: "When unable to determine the branch (detached HEAD), the daemon returns no question — do not retry without a branch context."

## 7. tr ask resolver + URL params (ask.go)

- [x] 7.1 Added `resolveAskBranch()` helper in `ask.go`. Runs `exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()`; returns `(string, error)`. Treats empty output and `"HEAD"` (detached HEAD literal) as errors. Imports `os/exec` and `strings` (already imported).
- [x] 7.2 `resolveAskRepo()` now returns `(string, error)`. The "fall back to global pool" behavior is removed; the previous stderr advisory is also removed (the caller now prints the advisory).
- [x] 7.3 `askCmd().RunE` calls both resolvers. On error: prints the appropriate advisory (`[ask] not inside a git repo — skipping this recall window` or `[ask] detached HEAD — no branch context; skipping this recall window`) and `return nil` — no HTTP request, no TUI invocation. On success: passes `repo` and `branch` to `newAskModel(timeout, repo, branch)`.
- [x] 7.4 `askModel` struct has new `branch string` field. `newAskModel` signature: `newAskModel(timeout time.Duration, repo, branch string)`.
- [x] 7.5 `pollCmd` always includes both `?repo=<urlenc>&branch=<urlenc>` (no `if repo != ""` short-circuit — repo and branch are always non-empty post-7.3).
- [x] 7.6 `postAnswer` signature now `postAnswer(id, answerIndex, repo, branch, client)`. URL includes `&branch=<urlenc(branch)>` for symmetry (the answer endpoint is ID-keyed and ignores it).
- [x] 7.7 `postSkip` signature now `postSkip(id, repo, branch, client)`. URL includes `?branch=<urlenc(branch)>` for symmetry.

## 8. tr status advisory (main.go)

- [x] 8.1 In `runStatus()`, after the existing daemon-up confirmation + config-show blocks, added a stale-questions advisory block. Resolves `repo` via `hooks.FindRepoRoot()`; skips silently on error (not in a git repo).
- [x] 8.2 In-repo: calls `GET /recall/stale?repo=<resolved-repo>` reusing the existing 1-second-timeout status client. Skips silently on any failure (the upstream health check already exited non-zero if the daemon is down).
- [x] 8.3 On 200 OK, unmarshals `{"repo":"...","branches":{"branch":N, ...}}`. For each branch with `count > 0`, prints `⚠  <count> recall question(s) pending on branch <branch>` and `   Switch back: git switch <branch> && tr ask`. Empty `branches` → no advisory. Logic extracted into a `printStaleQuestionsAdvisory` helper for testability and to keep `runStatus` readable.
- [x] 8.4 Order preserved: daemon-health → config-show → stale-questions advisory. The advisory prints AFTER the existing output.

## 9. Test updates

- [x] 9.1 `cmd/tr/cache_test.go`: every `s.Save`, `s.Recent`, `s.SaveQuestion`, `s.NextQuestion`, `s.QueueDepth`, `s.RecentAnswered`, `s.PeekNextQuestion` call site updated with the new branch arg. Replaced `""` repo/branch with `"/repo/test", "main"`, `"/repo/x", "main"`, etc. Added `TestRecentConceptsScopedToBranch` for branch-isolation. Removed `TestAddColumnIfMissingMigration` (the migration it tested is gone — see section 1). Updated `TestRepoIndexesExist` to check the new `idx_concepts_repo_branch_seen` and `idx_questions_repo_branch_q` indexes.
- [x] 9.2 Added three Decision 3 guard tests: `TestSaveRefusesEmptyRepoOrBranch`, `TestRecentRefusesEmptyRepoOrBranch`, `TestSaveQuestionRefusesEmptyRepoOrBranch`. Empty `repo` or `branch` is a no-op (returns `nil` / `(nil, nil)` / `(0, nil)`).
- [x] 9.3 `cmd/tr/integration_test.go`: hook POST bodies already include `repo` AND `branch`. Added `TestPipelineSavesConceptsTaggedWithBranch` which asserts the branch column is populated. Added `TestRecallNextBranchIsolation` for branch-isolation at the HTTP layer. Added `TestRecallStaleEndpoint` covering: 400 on missing repo, 200 with empty map when no pending questions, 200 with per-branch counts (branches with 0 omitted), and cross-repo isolation.
- [x] 9.4 `cmd/tr/recall_test.go`: no `Synthesize(ctx, repo, ...)` calls existed in tests; nothing to update. Engine signature change is exercised via `TestRecallNextBranchIsolation` and the MCP `recall_next` integration tests.
- [x] 9.5 `cmd/tr/ask_test.go`: added `TestResolveAskRepoReturnsErrorOnGitFailure` (replaces the old `TestResolveAskRepoAdvisoryOnGitError` that asserted the removed stderr-fallback behavior), `TestResolveAskBranchReturnsBranchOnSuccess`, `TestResolveAskBranchReturnsErrorOnDetachedHEAD` (uses a fake `git` script on PATH to stub `git rev-parse --abbrev-ref HEAD`). Updated `TestPollCmdAppendsRepoAndBranchQueryParam`, `TestPostAnswerAppendsRepoAndBranchQueryParam`, `TestPostSkipAppendsRepoAndBranchQueryParam` to assert both `repo=` and `branch=` query params. Removed `TestPollCmdOmitsRepoQueryParamWhenEmpty` (the global-pool fallback behavior is gone).
- [x] 9.6 `cmd/tr/integration_test.go` MCP test section: updated all 5 MCP `recall_next`/`recall_status` call sites to pass `branch: "main"`. Updated `recall_answer` to include branch. Updated resource-template URIs to use `recall://queue?repo=<urlenc>&branch=main` and `recall://recent?repo=<urlenc>&branch=main` (URL-escaped repo path required because MCP uritemplate regex only matches percent-encoded `/` in form-style query values).

## 10. Spec/docs hygiene + verification

- [x] 10.1 Removed the "Open issue: cache-layer tenant isolation spec drift" block (old lines 157-167) from `DOCS/ARCHITECTURE/INSTALL_LAYERS.md`. The drift is resolved by Y1.
- [x] 10.2 Removed the Open Issue header and the spec-drift reference. The "Known leak points" list now ends at the init-re-prompts note.
- [x] 10.3 Updated `AGENTS.md` "Cache" row: replaced the spec-drift note with "Concepts and questions are scoped per-repo AND per-branch (no global pool). Empty `repo` or `branch` is refused at the store layer." Verified the rest of the Architecture section is consistent.
- [x] 10.4 `go build ./...` — clean. `go vet ./...` — clean. `go test -count=1 ./...` — all pass: `cmd/tr` 2.08s, `internal/hooks` 0.51s, `internal/presentation/terminal` 0.91s.
- [x] 10.5 Manual one-time maintainer action: `rm ~/.tr/memory.db` (or `Remove-Item ~/.tr/memory.db` on Windows). Required because the in-code migration path is gone (Decision 2 in `cache-tenant-isolation/design.md`). The next `tr serve` will create a fresh DB with the new schema (`repo TEXT NOT NULL` and `branch TEXT NOT NULL` on both `concepts` and `questions`). Confirm via `tr status` once the daemon is up.
- [x] 10.6 `openspec validate cache-tenant-isolation` passes (and `--strict` mode passes). All 7 spec deltas at `openspec/changes/cache-tenant-isolation/specs/{concept-cache,question-store,question-delivery,recall-engine,mcp-server,tr-ask,stale-question-advisory}/spec.md` parse and validate against the schema.