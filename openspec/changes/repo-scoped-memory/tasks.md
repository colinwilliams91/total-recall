## 1. Schema + Store (internal/cache/store.go)

- [x] 1.1 Strip legacy `concepts.db` migration: remove `legacyDBFilename` constant and the `copyFile` migration block in `Open()`; remove the `copyFile` helper if now unused
- [x] 1.2 Add `repo TEXT NOT NULL DEFAULT ''` column to `concepts` and `questions` via the existing idempotent `addColumnIfMissing` migrations list
- [x] 1.3 Add covering indexes: `idx_concepts_repo_seen ON concepts(repo, seen_at DESC)` and `idx_questions_repo_q ON questions(repo, queued_at ASC) WHERE delivered_at IS NULL` (idempotent `CREATE INDEX IF NOT EXISTS`)
- [x] 1.4 Add one-time purge: after the `repo` column is added to each table, `DELETE FROM concepts; DELETE FROM questions;` gated on the column being newly added (track with a bool from `addColumnIfMissing`); log `[store] purged un-tagged legacy rows during repo-scoping migration`
- [x] 1.5 Update `Fingerprint` usage and `Save(ctx, repo, concepts)` to insert `repo` into the `concepts` row
- [x] 1.6 Update `Recent(ctx, repo, n)` to `WHERE repo = ?`
- [x] 1.7 Update `SaveQuestion(ctx, repo, question, choices, correctIndex)` to insert `repo`
- [x] 1.8 Update `NextQuestion(ctx, repo, claimedBy)` atomic dequeue to `WHERE delivered_at IS NULL AND repo = ?`
- [x] 1.9 Leave `GetQuestion`, `AnswerQuestion`, `SkipQuestion` ID-keyed (no `repo` param)
- [x] 1.10 Update `QueueDepth(ctx, repo)` to `WHERE delivered_at IS NULL AND repo = ?`
- [x] 1.11 Update `PeekNextQuestion(ctx, repo)` to `WHERE delivered_at IS NULL AND repo = ?`
- [x] 1.12 Update `RecentAnswered(ctx, repo, limit)` to `WHERE answered_at IS NOT NULL AND repo = ?`
- [x] 1.13 Add `TR_HOME` env override to `trDir()`: check `os.Getenv("TR_HOME")` first, use it if non-empty, else fall back to `~/.tr`
- [x] 1.14 Run `go build ./...` and `go vet ./...` — verify clean (expect compile errors in callers, fixed in later tasks)

## 2. Recall Engine (internal/recall/engine.go)

- [x] 2.1 Update `Engine.Synthesize(ctx, repo, difficulty, model)` signature to accept `repo` and pass to `store.Recent(ctx, repo, 20)`
- [x] 2.2 `GenerateFeedback` signature unchanged (ID-keyed, no repo)

## 3. Pipeline + HTTP Handlers (internal/engine/server.go)

- [x] 3.1 In `runPipeline`, pass `env.Repo` to `store.Save(ctx, env.Repo, fingerprints)`
- [x] 3.2 In `runPipeline`, pass `env.Repo` to `recallEngine.Synthesize(ctx, env.Repo, "", model)`
- [x] 3.3 In `runPipeline`, pass `env.Repo` to `store.SaveQuestion(ctx, env.Repo, q.Question, q.Choices, q.CorrectIndex)`
- [x] 3.4 `handleRecallNext`: read `repo` query param (`r.URL.Query().Get("repo")`), pass to `store.NextQuestion(ctx, repo, "shell")`; when repo is non-empty and result is nil, log the repo-move advisory
- [x] 3.5 `handleRecallAnswer`: read `repo` query param (accepted for symmetry; `AnswerQuestion`/`SkipQuestion` are ID-keyed so no behavior change)
- [x] 3.6 Run `go build ./...` and `go vet ./...` — verify clean

## 4. MCP Server (internal/engine/mcp.go)

- [x] 4.1 `recall_next`: add optional `Repo *string` to input struct; pass dereferenced value or `""` to `store.NextQuestion(ctx, repo, "mcp")`
- [x] 4.2 `recall_answer`: add optional `Repo *string` to input struct (accepted, ID-keyed ops unchanged)
- [x] 4.3 `recall_status`: add optional `Repo *string`; scope `store.QueueDepth(ctx, repo)` accordingly
- [x] 4.4 `recall://queue` resource: derive repo from URI query string or default `""`; scope `QueueDepth` and `PeekNextQuestion`
- [x] 4.5 `recall://recent` resource: derive repo from URI query string or default `""`; scope `RecentAnswered(ctx, repo, 10)`
- [x] 4.6 Update `recall_workflow` prompt text to mention passing the current repo to `recall_next` when known
- [x] 4.7 Run `go build ./...` and `go vet ./...` — verify clean

## 5. tr ask (cmd/total-recall/ask.go)

- [x] 5.1 Before polling, call `hooks.FindRepoRoot()` to resolve repo; on error, set `repo=""` and log `[ask] not inside a git repo — falling back to global recall queue` to stderr
- [x] 5.2 Thread `repo` through the `askModel` (add a `repo string` field set at construction)
- [x] 5.3 `pollCmd`: append `?repo=<url-encoded repo>` to `/recall/next` when repo is non-empty
- [x] 5.4 `postAnswer`: append `repo` query param to `/recall/answer?feedback=true`
- [x] 5.5 `postSkip`: append `repo` query param to `/recall/answer`
- [x] 5.6 Run `go build ./...` and `go vet ./...` — verify clean

## 6. Config TR_HOME (internal/config/config.go)

- [x] 6.1 Update `UserConfigPath()` to check `TR_HOME` first, return `$TR_HOME/config.yaml` when set, else `~/.tr/config.yaml`
- [x] 6.2 Update `UserConfigDir()` to check `TR_HOME` first, return `$TR_HOME` when set, else `~/.tr`
- [x] 6.3 Run `go build ./...` and `go vet ./...` — verify clean

## 7. Tests

- [x] 7.1 Update any existing cache tests to pass `repo` arguments and assert repo-scoped behavior
- [x] 7.2 Add a cache test asserting repo isolation: save concepts for repo X, save for repo Y, `Recent(repo=X)` returns only X's
- [x] 7.3 Add a cache test asserting `NextQuestion` repo isolation: queue Q for X, `NextQuestion(repo=Y)` returns nil
- [x] 7.4 Add a `TR_HOME` test: `t.Setenv("TR_HOME", t.TempDir())`, call `cache.Open()`, confirm it opens under tempdir not `~/.tr`
- [x] 7.5 Update integration tests (daemon handlers) to pass `repo` query params and assert scoping
- [x] 7.6 Run `go test ./...` — verify all pass

## 8. Documentation Sync

- [x] 8.1 Update `AGENTS.md` — Cache section: note repo column + `TR_HOME` override; remove `concepts.db` migration mention
- [x] 8.2 Update `ROADMAP.md` — note repo-scoped memory + `TR_HOME` test isolation
- [x] 8.3 Update `DOCS/ARCHITECTURE/DELIVERY.md` and any architecture doc referencing the global queue — reflect repo scoping
- [x] 8.4 Update `DOCS/TESTING/E2E.md` — add repo-scoped test checks and `TR_HOME` isolation usage
- [x] 8.5 Sync canonical `openspec/specs/{concept-cache,question-store,question-delivery,mcp-server,tr-ask}/spec.md` with the change deltas; add `openspec/specs/tr-home-override/spec.md`
- [x] 8.6 Remove the drifted `cache.db` reference in `openspec/specs/concept-cache/spec.md` (already says `memory.db` in the change delta)

## 9. Final Verification

- [x] 9.1 Run `go build ./... && go vet ./... && go test ./...` — all clean
- [x] 9.2 Manual smoke: set `TR_HOME` to a temp dir, start daemon, commit in two repos, confirm `tr ask` in each only surfaces that repo's questions
