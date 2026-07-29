## Context

This is the most consequential phase in the Y-batch (designated BLOCKING in the parent plan). It closes the cache-isolation gap that the existing `repo` column partially closed and the existing "Repo column migration purges un-tagged legacy rows" spec requirement half-codified. Five decisions were resolved during explore:

- **Issue 1 — Migration strategy:** strip the existing in-code migration block from `cache.Open()`; schema changes are forward-only; maintainer deletes `~/.tr/memory.db` manually on schema bumps. `branch TEXT NOT NULL` (no `DEFAULT ''`) is added to `concepts` and `questions` `CREATE TABLE`. Every caller must supply non-empty values; the Store refuses empty values.
- **Issue 2 — Empty-branch (detached HEAD) semantics:** option 2.A — detect-and-skip. Hooks' branch resolution (`git rev-parse --abbrev-ref HEAD`) stays as-is. When it returns `""` (detached HEAD state), `runPipeline` logs `[pipeline] skipping insert: detached HEAD (no branch)` and returns. No fallback resolution, no synthetic branch label, no SHA substitution.
- **Issue 3 — Branch filtering scope across question operations:** decision 3.A + Option D + M1. Pure branch isolation across all question operations; `tr status` advisory surfaces stale questions on other branches (Option D); MCP layer propagates `branch` identically to the CLI layer (M1) so the AI-agent surface doesn't leak cross-branch.
- **Issue 4/4-r.A — Sentinel:** (resolved in Y2's design) the `# total-recall managed` sentinel is kept; the `[total-recall]` stderr prefix is kept; CLI invocation strings inside hook/message bodies are renamed to `tr`. Not directly relevant to Y1 but referenced when the pipeline handles `env.Branch`.
- **Issue 5 — Y4 hook PATH strategy:** (resolved in Y4's design) reliance on `tr` being on PATH, drop the `os.Executable()` capture. Not directly relevant to Y1.
- **Issue 6 — Y3 hooks-dir resolution:** (resolved in Y3's design) resolve-once-pass-in via `git rev-parse --git-path hooks`. Not directly relevant to Y1.

Existing layer model (per `DOCS/ARCHITECTURE/INSTALL_LAYERS.md`): this change touches the **user cache** layer (schema), the **daemon** layer (HTTP API + MCP propagation), and the **binary** layer (`tr ask` resolver behavior). The **git hooks** layer is untouched — hooks already send `env.Branch` in the `HookEnvelope`; they just need the daemon to honor it).

No users exist. No live migration matters.

## Goals / Non-Goals

**Goals:**
- Concepts extracted on `repo-A` / `feature-X` are retrievable only via a `WHERE repo = ? AND branch = ?` query against `repo-A` / `feature-X`. Never `repo-A` / `main`. Never `repo-B`.
- Questions synthesized from `feature-X` concepts are queued, claimed, peeked, and listed under `repo-A` / `feature-X` only. Never cross-branch within a repo, never cross-repo.
- `tr ask` on `repo-A` / `feature-X` only ever receives questions synthesized from `feature-X` concepts. If the user switches to `main`, `tr ask` on `main` is unaffected by `feature-X` activity (and vice versa).
- The MCP surface (`recall_next`, `recall_status`, `recall://queue`, `recall://recent`) behaves identically to `tr ask` w.r.t. branch scoping — no contract drift between CLI and AI-agent surfaces.
- Detached-HEAD extractions are skipped silently with a log line; no synthetic `branch` values pollute the schema.
- A `tr status` advisory surfaces the gap created by strict isolation when questions remain undelivered on a non-active branch, so users know to switch back and answer them.
- No in-code schema migration runs. The migration block in `cache.Open()` (store.go:111-157, ~47 lines) is stripped. Fresh DBs always have the full schema.

**Non-Goals:**
- No automatic cross-branch question delivery. The post-checkout advisory brainstormed in Issue 3's Option A and the migrate-vs-answer UX in Option C are explicitly deferred to a future phase. Y1 only *detects* stale questions; it does not *act* on them.
- No new hook type (no post-checkout, no merge-commit).
- No GC for questions orphaned on deleted branches. A question queued under `branch=feature-X` lingers as undelivered if the user deletes `feature-X` without first answering. A future phase can add cleanup.
- No rebase-aware behavior. Interactive-rebase commits fire `pre-commit` (only for commits that actually occur, on whatever branch the rebase lands on); detached-HEAD is skipped per 2.A; rebases that conclude on `feature-X` result in `branch=feature-X` extractions — that's correct.
- No new tests for behavior already covered by Issue 3's spec scope.

## Decisions

### Decision 1 — `branch TEXT NOT NULL` with no `DEFAULT ''`

The new columns on `concepts` and `questions` are `branch TEXT NOT NULL` with **no DEFAULT clause**. The Go caller must always supply a non-empty branch string on every insert; absent a default, SQL-level implicit behavior can't mask a Go-level omission.

**Rationale (Issue 1):** a `DEFAULT ''` would silently admit `branch=""` rows even when the caller forgot to pass the value. Rejecting `''` at the SQLite layer (the column refuses NULL via NOT NULL, but accepts `''`) is *not* the same as refusing `''` at the application layer. The app-layer guard (early return with log) is the rejection mechanism; the NOT NULL constraint is the floor.

**Counter-argument considered:** keeping `DEFAULT ''` for SQL-ergonomics (ad-hoc tooling inserts don't need to supply it). Rejected (Issue 1): there is no ad-hoc tooling; the only writer is the Go pipeline.

### Decision 2 — Strip the existing migration block from `cache.Open()`

The ~47-line block at store.go:111-157 that does `addColumnIfMissing` for the `correct_index`, `answer_index`, `correct`, `feedback` historical migrations AND the `repo` column-add + `DELETE FROM` purge migrations is deleted entirely. `Open()` becomes: `CREATE TABLE IF NOT EXISTS` (with full final schema) + index creation + return. The `addColumnIfMissing` helper function (store.go:166) is deleted if Y1 is the only caller — verify via ripgrep before deletion.

**Rationale (Issue 1):** no users exist; no user-with-old-DB scenario is real; dead code is bad; existing accumulated data is easy to reproduce.

**Migration path:** maintainer (only user) deletes `~/.tr/memory.db` once before starting the daemon with the new binary. Fresh DB is created on next `Open()`.

**Alternatives considered and rejected:**
- Continue auto-purge pattern (Issue 1 Option 1) — would lose the maintainer's existing accumulated repo-scoped concepts unnecessarily; adds dead code.
- Add column, no purge, document manual deletion (Issue 1 Option 2) — leaves `branch=""` rows lingering;` manual-deletion docs add cognitive surface; conflict with the "user has no knowledge of migrations" principle.
- Hybrid preserve (Issue 1 Option 3) — generates a `branch=""` legacy pool that pollutes isolation or becomes inert — either outcome is bad.

### Decision 3 — Reject empty repo or branch at the application layer with early return + log

`Store.Save`, `Store.Recent`, `Store.NextQuestion`, `Store.SaveQuestion`, `Store.QueueDepth`, `Store.PeekNextQuestion`, `Store.RecentAnswered` each check at the top: if `repo == "" || branch == ""`, return early (no error) with a log line `[store] skipping <operation>: empty repo or branch`.

**Rationale (Issue 1):** the existing pattern at store.go:186-187 (`if len(concepts) == 0 { return nil }`) is the precedent — early no-op with no error. Error returns would force every caller to branch on the error; the log + return-nil pattern is one-liner on both sides. Matches the existing `runPipeline` early-return shape at server.go:138 (`payload.Diff == "" → log + return`).

**The absence of error returns is the entire point:** extracted concepts are non-critical; silently dropping them is correct behavior, not a degraded mode.

### Decision 4 — Detached HEAD: detect-and-skip in `runPipeline`, no hook-script edits

`internal/engine/server.go`'s `runPipeline` adds an early-return guard: after extracting `env.Branch` from the hook envelope, if it's empty, log `[pipeline] skipping insert: detached HEAD (no branch)` and return. Hook scripts are not modified; they keep calling `git rev-parse --abbrev-ref HEAD` which returns `""` in detached HEAD.

**Rationale (Issue 2):** every "resolve branch differently" fallback (using `git rev-parse HEAD` for a commit SHA as branch value, `git name-rev` for symbolic labels, `git rev-parse --abbrev-ref @{-1}` for the previous branch, a synthetic `"detached"` string) either pollutes the branch namespace with non-branch values or misattributes concepts to a branch the user wasn't really working in. Detached-HEAD extractions are rare; the next commit on a real branch will re-extract. Missing one is a non-event.

**Alternatives considered and rejected:**
- 2.B — Use `BRANCH="${BRANCH:-_DETACHED_}"` synthetic label in hook. Creates a permanent `_DETACHED_`-branch pool that's queryable only when re-detached. Adds hook-script edits + spec for the sentinel. Adds zero value.
- 2.C — Use `git rev-parse HEAD` (SHA) as branch value. Conceptually incoherent — branch column holds a SHA. Schema drift.
- 2.D/2.E — previously rejected as "right namespace, wrong attribution" or "polluting with detached-N labels".

### Decision 5 — Pure branch isolation across all question operations (3.A), including the orphaned-question trade-off

`Store.NextQuestion`, `Store.SaveQuestion`, `Store.QueueDepth`, `Store.PeekNextQuestion`, `Store.RecentAnswered` all take `branch` and filter `WHERE repo = ? AND branch = ?`. A question synthesized on `feature-X` and never claimed before the user switches branches lingers in the `feature-X` queue until the user re-runs `tr ask` on `feature-X`. Deleting `feature-X` without ever claiming leaves an orphan question in the DB. No GC in Y1.

**Rationale (Issue 3):** this is the direct consequence of "branch is a required and scoped value" applied uniformly. The "merge into main" or "answer now during checkout" UX follow-ups are explicitly deferred (Issue 3 brainstorm). The orphan question issue is acknowledged and deferred — not solved.

**Cost documented in design.md:** a user who extensively works on `feature-X`, switches to `main`, and runs `tr ask` will see "all caught up" if `main` has no pending questions even if `feature-X`'s queue has 5 questions. This is correct behavior under strict isolation.

**Mitigation:** Decision 7 below (Option D `tr status` advisory) closes the visibility gap without breaking isolation.

### Decision 6 — M1: propagate `branch` through every MCP surface

MCP gains `branch` parameter identical to the `repo` parameter in shape:
- `recallNextIn`, `recallAnswerIn`, `recallStatusIn` struct field additions: `Branch *string "json:\"branch,omitempty\"`.
- New helper `branchFromToolInput(r *string) string` mirroring `repoFromToolInput`.
- New helper `branchFromResourceURI(uri string) string` mirroring `repoFromResourceURI` (extracts `?branch=` from `recall://queue?repo=X&branch=Y`).
- `recall_next` handler calls `store.NextQuestion(ctx, repo, branch, "mcp")`.
- `recall_status` handler calls `store.QueueDepth(ctx, repo, branch)`.
- `queueResourceHandler` calls `store.QueueDepth` + `store.PeekNextQuestion` with both.
- `recentResourceHandler` calls `store.RecentAnswered` with both.
- Resource templates `recall://queue{?repo}` → `recall://queue{?repo}{&branch}`; `recall://recent{?repo}` → `recall://recent{?repo}{&branch}`.
- `recallWorkflowInstructions` prompt content appended: "Pass the current branch from `git rev-parse --abbrev-ref HEAD` as `branch` so questions are scoped to active work."

**Rationale (Issue 3):** deferring MCP branch propagation would create contract drift between the CLI and AI-agent surfaces — the exact disease Y1 is curing. The mechanical cost is ~20 lines + 2 helpers. It mirrors the `repo` decision verbatim; no new design questions.

**Alternative considered and rejected (Issue 3):** M3 defer to fast-follow phase and document the contract drift in spec. Rejected because the spec gap is itself a footgun and the work is small.

### Decision 7 — Option D: `tr status` advisory for stale questions

New endpoint `GET /recall/stale?repo=<path>` returns:
```json
{"repo": "/path/to/repo", "branches": {"feature-X": 3, "main": 0}}
```
The daemon scans `questions` for the repo's distinct non-empty branches with `delivered_at IS NULL` and counts them. (Implementation note: query shape `SELECT branch, COUNT(*) FROM questions WHERE delivered_at IS NULL AND repo = ? GROUP BY branch`.) Branches with count 0 are excluded from the result (or included as 0 — design choice; excluding is cleaner).

`runStatus()` in `cmd/tr/main.go` adds a `GET /recall/stale?repo=<resolved-repo>` call (using `hooks.FindRepoRoot()` to resolve `repo` for the status command). If the user isn't in a repo, the advisory block is skipped (no `repo` to query). The printed output format:
```
⚠ 3 recall questions pending on branch feature-X (last queued: <timestamp>)
   Switch back: git switch feature-X && tr ask
```

**Rationale (Issue 3):** the smallest UX safety net that doesn't break isolation. Detection uses existing `delivered_at IS NULL` semantics + Decision 5's per-branch scoping. Agent prompt for deferred migrate-vs-answer UX preserved in design.md (Open Questions) for future phase owners.

**Future anchor (deferred):** the next phase can introduce a `recall_migrate` MCP tool and a CLI command like `tr migrate --from feature-X --to main` that updates the `branch` column on pending questions. The Option D advisory is the foundation for that UX — it surfaces the gap that the future UX would resolve. Y1 ships the foundation (detection + surfacing); the future phase ships the action.

**Alternatives considered and rejected (Issue 3):**
- Option A — post-checkout hook that triggers `tr ask` before leaving the branch. Reject for Y1 scope: fourth hook type + new ask flag + new endpoint = a Y6 of its own.
- Option B — cross-branch fallback in `tr ask`. Reject: breaks strict isolation, the entire point of Y1.
- Option C — merge-commit migration of questions. Reject: only works for true merges; FF/squash/rebase bypass `MERGE_HEAD`.
- Option E — time-based promotion. Reject: breaks isolation and creates a "after N days questions merge to repo pool" unstable behavior.

### Decision 8 — Answer-path stays ID-keyed

`Store.AnswerQuestion`, `Store.GetQuestion`, `Store.SkipQuestion` (store.go:282, 299, 320) continue to take only `id`. They do NOT take `repo` or `branch`. The existing comments "ID-keyed (globally unique) — no repo parameter needed" stay.

**Rationale:** question IDs are SQLite autoincrement and thus globally unique. The answer endpoint is invoked by a `tr ask` session that already obtained a question ID via `/recall/next` — the ID encodes its origin branch. There's no need to re-validate repo/branch on the answer path; the existing pattern is correct.

## Risks / Trade-offs

- **Risk:** refactoring breaks `cmd/total-recall/cache_test.go` call sites that currently pass `""` as `repo` (lines 68, 88, 94, e.g. `s.Save(ctx, "", concepts)`). → **Mitigation:** those tests need real values (e.g. `s.Save(ctx, "/repo/x", "main", concepts)`). Item in tasks.md. Tests should also assert that empty values return early (no-op) with a log line — those are the new tests for the Decision 3 guard.
- **Risk:** MCP tests in `integration_test.go` need to update their MCP call sites to include `branch` — if missed, MCP tests pass `Branch = nil` → `branchFromToolInput` returns `""` → `Store.NextQuestion(ctx, repo, "", "mcp")` returns empty result + no question → test fails to find the question it expects. → **Mitigation:** tasks.md includes MCP test updates; mock data in tests must include both `repo` and `branch`; the integration test seeds `Store.Save` with real values; MCP `recall_next` must pass `branch` matching what was saved.
- **Risk:** the orphaned-question cleanup question is real but **deferred** — if `feature-X` accumulates 50 questions and the user deletes the branch, the DB has 50 orphaned rows forever. Y1 explicitly does not address this; future phase (Y?) might add an `tr db cleanup --branch X` or auto-purge questions on detected branch deletion via periodic scan. → **Trade-off documented:** users accept orphans over cross-branch leaks.
- **Risk:** `tr status` needs to resolve `repo` via `hooks.FindRepoRoot()`; if the user runs `tr status` outside a git repo, the advisory block silently skips (no error). → **Mitigation:** matches `resolveAskRepo`'s existing behavior; documented in tasks.
- **Risk:** A user invoking an MCP agent from a non-git directory (e.g. `cd /tmp && claude` in an MCP-literate editor) has the agent pass `Branch = nil`; the daemon's `branchFromToolInput` returns `""`, the Store methods return empty (Decision 3). The MCP agent gets `{"question": null}` for recall_next and `{"queue_depth": 0}` for recall_status. This mirrors the CLI's behavior outside a repo and is correct.
- **Trade-off:** strict branch isolation means a user who merges `feature-X` into `main` and answers the merged code's recall questions on `main` is NOT prompted on the same concepts — they were on `feature-X`. The user's own workflow choice determines whether recall questions follow work across the merge boundary. The deferred migration UX (Decision 7's future anchor) addresses this; Y1 doesn't.

## Migration Plan

1. Implement Y1 cache-tenant-isolation per tasks.md.
2. As the maintainer/sole user: before starting the daemon with the new binary for the first time, delete `~/.tr/memory.db` (`rm ~/.tr/memory.db` on Unix; `Remove-Item ~/.tr/memory.db` on PowerShell). The next `tr serve` creates a fresh DB with the new `branch` columns.
3. Restart the daemon (if currently running) using the new binary built from Y1 + Y2.
4. Hooks are unchanged (they already send branch in the envelope); the daemon's new code now honors it.

**Rollback:** revert the commit. The old DB schema doesn't have `branch` columns; the old binary works with the old DB. Since there's no migration code to undo, rollback is a pure git revert + restore `~/.tr/memory.db` from backup if you wiped it.

## Open Questions

- **Deferred: migrate-vs-answer UX for stale questions.** Issue 3's brainstorming identified a likely-future feature — on `tr status` advisory, the user can `tr migrate --from feature-X --to main` to update `branch` column on pending questions (effectively re-queueing them on `main`), OR `tr ask --branch feature-X` temporarily to answer them in-place. Both are deferred. The Decision 7 foundation makes either a fast-follow.
- **Deferred: orphaned-question cleanup.** A future `tr db cleanup` or daemon-side sweeper for orphaned questions (where the branch was deleted via `git branch -D`) is not in Y1.
- **Minor: should `tr status` also query the `main` branch's queue depth inline (when the user is on `main`), in addition to listing other branches' depths?** Out of scope for Y1; future polish.