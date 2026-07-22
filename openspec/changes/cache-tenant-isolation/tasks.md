## 1. Schema migration strip + branch-column addition in store.go

- [ ] 1.1 In `internal/cache/store.go`, add `branch TEXT NOT NULL` (NO `DEFAULT ''`) to the `concepts` `CREATE TABLE` statement (line ~22-29). Add the same column to the `questions` `CREATE TABLE` statement (line ~33-49).
- [ ] 1.2 Replace index `idx_concepts_repo_seen ON concepts(repo, seen_at DESC)` with `idx_concepts_repo_branch_seen ON concepts(repo, branch, seen_at DESC)`.
- [ ] 1.3 Replace index `idx_questions_repo_q ON questions(repo, queued_at ASC) WHERE delivered_at IS NULL` with `idx_questions_repo_branch_q ON questions(repo, branch, queued_at ASC) WHERE delivered_at IS NULL`.
- [ ] 1.4 Strip the entire `existingMigrations` block in `Open()` (store.go:111-122) — the loop over `correct_index`, `answer_index`, `correct`, `feedback` migrations.
- [ ] 1.5 Strip the entire `repo` migration block in `Open()` (store.go:124-145) — the loop over `concepts`/`questions` doing `addColumnIfMissing` on `repo` + the conditional `DELETE FROM` purge. Strip the `purged` flag + log line at lines 143-145.
- [ ] 1.6 Strip the post-migration index-creation block at lines 147-157 — the indexes `idx_concepts_repo_seen` and `idx_questions_repo_q` (replacement indexes are added in step 1.2/1.3 directly in `Open()` after the `CREATE TABLE` statements).
- [ ] 1.7 Delete the `addColumnIfMissing` helper (store.go:166-175) IF and only IF no other code calls it after step 1.4-1.6. Use `grep -r 'addColumnIfMissing' --include='*.go'` to confirm zero callers remain. If callers exist outside Y1 scope, leave the helper (out of scope for Y1).
- [ ] 1.8 Verify `cache.Open()` is now: `trDir` → `sql.Open` → `SetMaxOpenConns(1)` → `CREATE TABLE IF NOT EXISTS concepts` → `CREATE TABLE IF NOT EXISTS questions` → `CREATE INDEX IF NOT EXISTS idx_concepts_repo_branch_seen` → `CREATE INDEX IF NOT EXISTS idx_questions_repo_branch_q` → return `&Store{db: db}, nil`. No migrations, no `addColumnIfMissing` calls, no `purged` log lines.

## 2. Store method signature changes (add branch, reject empty)

- [ ] 2.1 `Store.Save(ctx, repo, branch, concepts)` — add `branch string` param; insert uses `(?, ?, ?, ?, ?)` for `concept, source, weight, repo, branch`. Add guard at top: `if repo == "" || branch == "" { log.Printf("[store] skipping insert: empty repo or branch"); return nil }`.
- [ ] 2.2 `Store.Recent(ctx, repo, branch, n)` — add `branch string` param; query `WHERE repo = ? AND branch = ?`. Add guard: `if repo == "" || branch == "" { log.Printf("[store] skipping recent: empty repo or branch"); return nil, nil }`.
- [ ] 2.3 `Store.SaveQuestion(ctx, repo, branch, question, choices, correctIndex)` — add `branch string` param; insert `VALUES (?, ?, ?, ?, ?)` for `question, choices, correct_index, repo, branch`. Add guard: `if repo == "" || branch == "" { log.Printf("[store] skipping savequestion: empty repo or branch"); return nil }`.
- [ ] 2.4 `Store.NextQuestion(ctx, repo, branch, claimedBy)` — add `branch string` param; the `UPDATE...RETURNING` inner `SELECT` adds `AND branch = ?`. Add guard: `if repo == "" || branch == "" { return nil, nil }` (no log needed for read; matches existing `nil, nil` return on no rows).
- [ ] 2.5 `Store.QueueDepth(ctx, repo, branch)` — add `branch string` param; query `WHERE delivered_at IS NULL AND repo = ? AND branch = ?`. Add guard: `if repo == "" || branch == "" { return 0, nil }`.
- [ ] 2.6 `Store.PeekNextQuestion(ctx, repo, branch)` — add `branch string` param; query `WHERE delivered_at IS NULL AND repo = ? AND branch = ?`. Add guard: `if repo == "" || branch == "" { return nil, nil }`.
- [ ] 2.7 `Store.RecentAnswered(ctx, repo, branch, limit)` — add `branch string` param; query `WHERE answered_at IS NOT NULL AND repo = ? AND branch = ?`. Add guard: `if repo == "" || branch == "" { return nil, nil }`.
- [ ] 2.8 `Store.AnswerQuestion`, `Store.GetQuestion`, `Store.SkipQuestion` (store.go:282, 299, 320) — **NO CHANGE**. They remain ID-keyed. Existing comments "ID-keyed globally unique — no repo parameter needed" stay unchanged.

## 3. Recall engine signature change

- [ ] 3.1 `internal/recall/engine.go`'s `Engine.Synthesize` signature: add `branch string` parameter between `repo` and `difficulty` — `Synthesize(ctx, repo, branch, difficulty, model)`. Pass `branch` to `e.store.Recent(ctx, repo, branch, 20)`.
- [ ] 3.2 Add a guard at the top of `Synthesize`: `if repo == "" || branch == "" { return nil, nil }`. This matches the store-guard behavior and doesn't waste an AI call when upstream sent empty values.

## 4. Daemon handler changes (server.go)

- [ ] 4.1 `runPipeline` (server.go:127) — after extracting `env.Payload.Diff`, add a guard: `if env.Branch == "" { log.Printf("[pipeline] skipping insert: detached HEAD (no branch)"); return }`. Place after the existing `payload.Diff == ""` early-return (line 138).
- [ ] 4.2 `runPipeline` — change `s.store.Save(ctx, env.Repo, fingerprints)` (line 160) to `s.store.Save(ctx, env.Repo, env.Branch, fingerprints)`.
- [ ] 4.3 `runPipeline` — change `s.recallEngine.Synthesize(ctx, env.Repo, "", s.cfg.AI.Model)` (line 167) to `s.recallEngine.Synthesize(ctx, env.Repo, env.Branch, "", s.cfg.AI.Model)` — the `""` is the existing difficulty placeholder, `env.Branch` goes between `env.Repo` and the existing `""`.
- [ ] 4.4 `runPipeline` — change `s.store.SaveQuestion(ctx, env.Repo, q.Question, ...)` (line 176) to `s.store.SaveQuestion(ctx, env.Repo, env.Branch, q.Question, ...)`.
- [ ] 4.5 `handleRecallNext` (server.go:195) — read `branch := r.URL.Query().Get("branch")` alongside `repo := r.URL.Query().Get("repo")`. If `repo == "" || branch == ""`, respond `204 No Content` directly without calling `store.NextQuestion`. Otherwise call `s.store.NextQuestion(r.Context(), repo, branch, "shell")`.
- [ ] 4.6 `handleRecallAnswer` (server.go:216) — **NO CHANGE** to the answer/eval logic; answer path is ID-keyed. The `repo` query param continues to be accepted for symmetry (and `branch` MAY be accepted the same way — add `branch` to the captured query params for symmetry with `repo`, but it is unused).

## 5. New endpoint /recall/stale (server.go)

- [ ] 5.1 Add `s.mux.HandleFunc("GET /recall/stale", s.handleRecallStale)` to `RegisterRoutes()` (server.go:96-104).
- [ ] 5.2 Add `handleRecallStale` method: read `repo := r.URL.Query().Get("repo")`; if `repo == ""`, respond `400 Bad Request` with `{"error":"repo query parameter is required"}`. Otherwise execute `SELECT branch, COUNT(*) FROM questions WHERE delivered_at IS NULL AND repo = ? AND branch != '' GROUP BY branch` (the `branch != ''` is defensive — `branch = ""` rows should never exist but are excluded). Build the response map and respond `200 OK` with `{"repo":"<path>","branches":{...}}`.

## 6. MCP layer changes (mcp.go)

- [ ] 6.1 Add `Branch *string "json:\"branch,omitempty\"` field to `recallNextIn`, `recallAnswerIn`, `recallStatusIn` structs (mcp.go:51-53, 80-85, 131-133).
- [ ] 6.2 Add `branchFromToolInput(r *string) string` helper mirroring `repoFromToolInput` (mcp.go:20-25).
- [ ] 6.3 Add `branchFromResourceURI(uri string) string` helper mirroring `repoFromResourceURI` (mcp.go:29-35). Extracts `?branch=` from URIs.
- [ ] 6.4 `recall_next` handler (mcp.go:54-77): call `branch := branchFromToolInput(in.Branch)`; pass `store.NextQuestion(ctx, repo, branch, "mcp")`. If `repo == "" || branch == ""`, the store returns `nil, nil` and the tool returns `{"question":null}` automatically.
- [ ] 6.5 `recall_status` handler (mcp.go:134-153): call `branch := branchFromToolInput(in.Branch)`; pass `store.QueueDepth(ctx, repo, branch)`. If either is empty, `QueueDepth` returns `0` automatically.
- [ ] 6.6 `recall_answer` handler (mcp.go:86-128): NO CHANGE to the evaluation logic; `Branch` field is accepted in the input struct for symmetry (already added in 6.1) but is unused on the answer path (ID-keyed).
- [ ] 6.7 `queueResourceHandler` (mcp.go:222-256): add `branch := branchFromResourceURI(req.Params.URI)`; pass to `store.QueueDepth` and `store.PeekNextQuestion`. If `repo == "" || branch == ""`, the store methods return zero/nil and the response shows `{"depth":0,"next":null}` automatically.
- [ ] 6.8 `recentResourceHandler` (mcp.go:261-294): add `branch := branchFromResourceURI(req.Params.URI)`; pass to `store.RecentAnswered`. If either is empty, store returns nil and the response shows `[]`.
- [ ] 6.9 Resource templates updated: change `URITemplate: "recall://queue{?repo}"` to `URITemplate: "recall://queue{?repo}{&branch}"` (mcp.go:167). Same for `recent` (mcp.go:183). The plain (non-template) `recall://queue` and `recall://recent` resources (mcp.go:156, 174) keep their existing URIs as canonical identity for subscription — no URI-template change needed.
- [ ] 6.10 `recallWorkflowInstructions` prompt content (mcp.go:16): append a sentence: "Pass the current branch from `git rev-parse --abbrev-ref HEAD` as `branch` so questions are scoped to active work." Also explicitly note: "When unable to determine the branch (detached HEAD), the daemon returns no question — do not retry without a branch context."

## 7. tr ask resolver + URL params (ask.go)

- [ ] 7.1 Add `resolveAskBranch()` helper in `ask.go` (paired with `resolveAskRepo`): runs `exec.Command("git", "rev-parse", "--abbrev-ref HEAD").Output()`, returns `(string, error)`. On error or empty output, returns `("", error)`.
- [ ] 7.2 Change `resolveAskRepo()` to return `(string, error)` instead of `string`. Remove the "fall back to global pool" behavior at ask.go:28-29. On error, return `("", err)`.
- [ ] 7.3 In `askCmd().RunE`, call both `repo, repoErr := resolveAskRepo()` and `branch, branchErr := resolveAskBranch()`. If either errors, print an advisory to stderr (`[ask] not inside a git repo — skipping this recall window` OR `[ask] detached HEAD — no branch context; skipping this recall window`) and `return nil` — no HTTP request. Otherwise pass `repo` and `branch` to `newAskModel(timeout, repo, branch)`.
- [ ] 7.4 `askModel` struct: add `branch string` field alongside `repo string` (ask.go:122). `newAskModel` signature: `newAskModel(timeout time.Duration, repo, branch string)`.
- [ ] 7.5 `pollCmd` (ask.go:145-171): add `?branch=<urlenc(branch)>` to the `/recall/next` URL. If `repo != ""` (always true post-7.2), include both `?repo=...&branch=...`.
- [ ] 7.6 `postAnswer` (ask.go:277-304): include `&branch=<urlenc(branch)>` in the `/recall/answer` URL for symmetry (the answer endpoint ignores it — ID-keyed).
- [ ] 7.7 `postSkip` (ask.go:306-320): same — include `?branch=...` on the URL for symmetry.

## 8. tr status advisory (main.go)

- [ ] 8.1 In `runStatus()` (main.go:469-498), after the existing daemon-up confirmation + config-show blocks, add a stale-questions advisory block. First, resolve `repo` via `hooks.FindRepoRoot()`. If error (not in a git repo), skip the block silently.
- [ ] 8.2 If in a git repo, call `GET /recall/stale?repo=<resolved-repo>` with a 1-second timeout (matching the existing health-check timeout). If the request fails or daemon is unreachable, skip silently (the upstream daemon-down exit already fired at the top of `runStatus`).
- [ ] 8.3 On 200 OK, unmarshal `{"repo":"...","branches":{"branch":N, ...}}`. For each branch with `count > 0`, print `⚠  <count> recall question(s) pending on branch <branch>` and `   Switch back: git switch <branch> && tr ask`. If `branches` is empty, print nothing.
- [ ] 8.4 Order is preserved: daemon-health block → config-show block → stale-questions block. The advisory block prints AFTER everything else.

## 9. Test updates

- [ ] 9.1 `cmd/tr/cache_test.go`: every `s.Save(ctx, ...)` and `s.Recent(ctx, ...)` call site gets an extra `branch` arg. Replace calls that use `""` (lines 68, 72, 88, 94, 100) with real values like `"main"`. The repo-isolation test (line 116-135) uses `"/repo/x"` vs `"/repo/y"`; add an analogous branch-isolation test: save to `repo="/r", branch="feature-X"` then `Recent("/r", "main", 10)` should return empty.
- [ ] 9.2 Add explicit test for the Decision 3 guard: `s.Save(ctx, "", "main", concepts)` returns `nil` and no rows inserted; `s.Recent(ctx, "/r", "", 10)` returns `(nil, nil)`.
- [ ] 9.3 `cmd/tr/integration_test.go`: hook POST bodies already include `repo` AND `branch` (the existing hook scripts already send `branch`); verify the integration test asserts that concepts saved via the hook pipeline land with the correct `repo` + `branch` columns. If existing assertions only check `repo`, extend to check `branch` too via `store.Recent(ctx, repo, branch, 10)` (which will be empty if the branch wasn't propagated). Add new test for `GET /recall/stale` endpoint coverage (seed via `store.SaveQuestion`, expect the endpoint to return the seeded count per branch).
- [ ] 9.4 `cmd/tr/recall_test.go`: every `Synthesize(ctx, repo, ...)` call gets `branch` arg added.
- [ ] 9.5 `cmd/tr/ask_test.go`: add tests for `resolveAskRepo` error path (returns `("", err)` when outside a repo — was previously the `""` fallback); `resolveAskBranch` happy path (inside a repo on a named branch — returns `("feature-X", nil)`); `resolveAskBranch` error path (detached HEAD — returns `("", err)`). These are unit tests at the function level, not full TUI tests.
- [ ] 9.6 `cmd/tr/integration_test.go` MCP test section: existing MCP-fetch tests must pass `Repo` AND `Branch` in the `recallNextIn` struct. If they pass only `Repo`, the store reads `Branch = ""` and returns no question — test will fail "no question returned." Update all MCP test call sites to include both. Verify resource-template URIs `recall://queue?repo=...&branch=...` are exercised by the integration test.

## 10. Spec/docs hygiene + verification

- [ ] 10.1 `DOCS/ARCHITECTURE/INSTALL_LAYERS.md` lines 157-167: remove the "Open issue: cache-layer tenant isolation spec drift" block. The drift is being resolved by this change; the block is stale.
- [ ] 10.2 `DOCS/ARCHITECTURE/INSTALL_LAYERS.md` line 159 mentions "requires per-repo, per-branch scoping" — that's no longer an Open Issue; remove the Open Issue header and any remaining reference to the spec drift.
- [ ] 10.3 `AGENTS.md` "Cache" row mentions "Spec drift: openspec/specs/concept-cache/spec.md requires per-repo, per-branch scoping; current schema has no repo/branch columns and Store.Recent takes no repo arg." — that whole note is stale; remove it. Replace with a single line: "Concepts and questions are scoped per-repo AND per-branch (no global pool). Empty `repo` or `branch` is refused at the store layer." Verify the rest of the AGENTS.md Architecture section is consistent.
- [ ] 10.4 Run `go build ./... && go vet ./... && go test ./...` — all must pass.
- [ ] 10.5 Manual one-time maintainer action: `rm ~/.tr/memory.db` (or `Remove-Item ~/.tr/memory.db` on Windows). The next `tr serve` will create a fresh DB with the new schema. Confirm via `tr status` showing no error on daemon启动.
- [ ] 10.6 Sanity-check: the `openspec validate cache-tenant-isolation` command passes; the spec deltas at `openspec/changes/cache-tenant-isolation/specs/{concept-cache,question-store,question-delivery,recall-engine,mcp-server,tr-ask,stale-question-advisory}/spec.md` parse and validate against the schema.