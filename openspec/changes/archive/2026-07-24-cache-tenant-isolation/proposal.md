## Why

The concept cache today stores concepts tagged with `repo` only; recall questions can already surface concept fingerprints from any branch within a repo (the half-finished repo isolation work in `internal/cache/store.go` and `internal/engine/server.go`). Users working on `feature-X` can be quizzed on concepts from `main` and vice versa — which violates tenant isolation at the branch level. Additionally, the existing inline migration code in `cache.Open()` is dead-on-first-run cruft (no userbase exists) that adds complexity for a graceful-degradation scenario that doesn't degrade gracefully. This phase finishes the branch axis, strips the migration block, and propagates branch filtering across every question-operation surface (cache, HTTP API, MCP layer, `tr status` advisory).

## What Changes

- Add a `branch TEXT NOT NULL` column to `concepts` and `questions` tables in `internal/cache/store.go`'s `CREATE TABLE` statements. No `DEFAULT ''` — the Go caller must always supply a value; this is enforceable at the column level (the absence of a default forces explicit values on every insert).
- Add a `branch` parameter to `Store.Save`, `Store.Recent`, `Store.SaveQuestion`, `Store.NextQuestion`, `Store.QueueDepth`, `Store.PeekNextQuestion`, `Store.RecentAnswered`. Each query becomes `WHERE repo = ? AND branch = ?`. No special-case "empty values match anything" semantics — empty repo or branch is refused at the app layer.
- Strip the existing `addColumnIfMissing` migration block from `cache.Open()` (store.go:111-157): the historical migrations for `correct_index`, `answer_index`, `correct`, `feedback`, and the `repo` column purges. Fresh databases always have the full schema; existing databases (sole maintainer's local `~/.tr/memory.db`) require a one-time manual `rm` since the schema will not match (Option (a) decision from Issue 1).
- Update the index `idx_concepts_repo_seen` to `idx_concepts_repo_branch_seen ON concepts(repo, branch, seen_at DESC)` and `idx_questions_repo_q` to `idx_questions_repo_branch_q ON questions(repo, branch, queued_at ASC) WHERE delivered_at IS NULL`.
- **BREAKING**: in `Store.Save`, refuse inserts when `repo == "" || branch == ""` (early no-op with log line `[store] skipping insert: empty repo or branch`); in `Store.NextQuestion`, `Store.Recent`, `Store.QueueDepth`, `Store.PeekNextQuestion`, `Store.RecentAnswered`, refuse queries when either is empty (return empty result + log line). This removes the "global pool" semantics that existed in previous spec text for `repo = ""`.
- In `internal/engine/server.go`'s `runPipeline`, propagate `env.Branch` to all `Store` calls (currently the envelope carries branch but the pipeline drops it). Add an early-return guard: if `env.Branch == ""`, log `[pipeline] skipping insert: detached HEAD (no branch)` and return — no concept extraction, no SaveQuestion (matches Issue 2's detect-and-skip decision 2.A).
- Add `branch` to the HTTP query layer: `GET /recall/next` accepts `?branch=` alongside `?repo=`; `POST /recall/answer` is ID-keyed and unchanged (it stays repo/branch-agnostic — the existing comment in store.go:282 explicitly notes "ID-keyed globally unique").
- Add a new helper `resolveAskBranch()` in `cmd/tr/ask.go` (paired with the existing `resolveAskRepo()`). Both return `(string, error)`. `tr ask` exits early with the not-in-repo advisory if either fails — no HTTP request sent. Remove the existing `resolveAskRepo` fallback to `""`; remove the "global pool" comment.
- In `internal/engine/mcp.go`, propagate `branch` through the MCP surface: add `Branch *string` param to `recall_next`, `recall_status` tool input structs; add `branchFromToolInput` helper. Update `queueResourceHandler` and `recentResourceHandler` to extract `?branch=` from the resource URI via a new `branchFromResourceURI` helper. Update the `recall://queue` and `recall://recent` resource templates from `recall://queue{?repo}` to `recall://queue{?repo}{&branch}`. Update `recallWorkflowInstructions` prompt text to instruct agents to pass the current branch from `git rev-parse --abbrev-ref HEAD`. This is the `M1` MCP branch propagation decision (Issue 3).
- Add `GET /recall/stale` endpoint that returns per-branch undelivered-question counts for a repo (response shape: `{"repo":"/path","branches":{"feature-X":3,"main":0}}`). Add a "stale questions" advisory block to `runStatus()` in `cmd/tr/main.go` (the `tr status` command) that queries this endpoint and prints warnings for any branch with `count > 0`. Issue 3's Option D decision.
- Maintain the existing CHANGE-time behavior that `POST /recall/answer` and the `recall_answer` MCP tool remain ID-keyed (no repo/branch filtering) — confirmed correct in Issue 3 since question IDs are globally unique.

**Non-changes (deliberate):**

- No new hook type is introduced (no post-checkout hook, no merge-commit detection) for cross-branch question delivery. Issue 3's brainstorming produced a future-phase concept (post-checkout hook advisory + optional migrate-vs-answer UX) that Y1 explicitly defers.
- No data migration runs at code time. Existing `~/.tr/memory.db` is incompatible with the new schema; maintainer deletes it manually. (Issue 1 Option 1.A decision.)
- No `branch = ""` rows are ever stored. Detached-HEAD extraction events are skipped silently. (Issue 2 decision 2.A.)

## Capabilities

### New Capabilities

- `stale-question-advisory`: informs the user via `tr status` when recall questions remain undelivered on branches other than the active one. Surfaces the gap created by strict branch isolation without attempting automatic cross-branch delivery.

### Modified Capabilities

- `concept-cache`: per-repo requirements become per-repo+per-branch. Existing "Concepts are saved per-repo" is amended to require both `repo` and `branch`. The "Repo column migration purges un-tagged legacy rows" requirement is REMOVED (the migration code is being deleted). Existing "Recent query returns concepts scoped to a repo" becomes "scoped to a repo and branch". New requirements: "Concepts require non-empty repo and branch"; "No in-code schema migration runs."
- `question-store`: every query method that takes `repo` gains a `branch` parameter; semantics change from "scoped to repo" to "scoped to repo and branch". `NextQuestion`, `SaveQuestion`, `QueueDepth`, `PeekNextQuestion`, `RecentAnswered` all narrow their `WHERE` clauses. `AnswerQuestion`, `GetQuestion`, `SkipQuestion` remain ID-keyed (no change). The "global pool" semantics for empty `repo` are removed.
- `tr-ask`: `resolveAskRepo` no longer falls back to `""`/"global pool" — it returns an error and `tr ask` exits early. A new helper `resolveAskBranch` mirrors it. Both `repo` and `branch` are sent on `/recall/next` and `/recall/answer` requests (except `answer` which is ID-keyed).
- `question-delivery`: the `runPipeline` in `internal/engine/server.go` updates `Store.Save`, `recall.Synthesize`, and `Store.SaveQuestion` call sites to pass `env.Branch`. New requirement: pipeline skips concept extraction when `env.Branch` is empty (detached HEAD).
- `recall-engine`: `recall.Engine.Synthesize` gains a `branch` parameter. It passes `branch` to `Store.Recent`.
- `mcp-server`: `recall_next`, `recall_status`, `recall://queue`, `recall://recent` gain a `branch` parameter (input struct field for tools; `?branch=` in URI for resources). `recall_answer` is unchanged (ID-keyed). `recallWorkflowInstructions` prompt is updated to instruct agents to pass the current branch.

## Impact

- **Code edited:**
  - `internal/cache/store.go` — `CREATE TABLE` statements (schema, no migration); 7 Store method signatures (add `branch` param + `WHERE` clause change); strip ~47 lines of migration code from `Open()`.
  - `internal/engine/server.go` — `runPipeline` branch propagation + detached-HEAD early-return; `handleRecallNext` reads `?branch=` query param; add `handleRecallStale` handler for `/recall/stale`; `RegisterRoutes` adds `GET /recall/stale`.
  - `internal/engine/mcp.go` — `recallNextIn`, `recallAnswerIn`, `recallStatusIn` gain `Branch *string`; new `branchFromToolInput` and `branchFromResourceURI` helpers; resource templates renamed to include `{&branch}`; `queueResourceHandler`, `recentResourceHandler` pass `branch` to Store; `recallWorkflowInstructions` text updated.
  - `internal/recall/engine.go` — `Synthesize` gains `branch` param, passes to `Store.Recent`.
  - `cmd/tr/ask.go` — `resolveAskBranch()` helper; `resolveAskRepo()` returns error instead of `""` fallback; `pollCmd` adds `?branch=` to URL; `postAnswer` adds `?branch=` (cosmetic only since answer is ID-keyed).
  - `cmd/tr/main.go` — `runStatus()` adds `GET /recall/stale` call and printer for the advisory block.
- **Tests updated:**
  - `cmd/tr/cache_test.go` — every `Store.Save` and `Store.Recent` call site gets the two new args (no `""` cases allowed — tests use real repo/branch values like `"/repo/x"`, `"feature-x"`).
  - `cmd/tr/integration_test.go` — hook POST bodies already include repo/branch; assert they flow through to the cache. Add `/recall/stale` endpoint test. Update MCP endpoint tests to pass `branch`.
  - `cmd/tr/recall_test.go` — `Synthesize` calls get the two new args.
  - `cmd/tr/ask_test.go` — `resolveAskBranch` happy-path test, `resolveAskBranch` error test, `resolveAskRepo` error (was-fallback) test.
- **Layers affected:** the **user cache** layer (`~/.tr/memory.db` schema) changes — maintainer deletes the existing DB once. The **daemon** layer (HTTP API, MCP) gains the `branch` parameter and `/recall/stale` endpoint. The **binary** layer (`tr ask`) gains the `resolveAskBranch` helper. The **user config** and **repo config** layers are untouched. The **git hooks** layer is untouched (hooks already send branch via `git rev-parse --abbrev-ref HEAD`).
- **Dependencies:** none added or removed.