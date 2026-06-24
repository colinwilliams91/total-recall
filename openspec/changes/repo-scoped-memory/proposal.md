## Why

Total Recall persists all concepts and questions in a single global `~/.tr/memory.db` with no repo identity attached to any row. The hook envelope carries `env.Repo` (the repo path from `git rev-parse --show-toplevel`), but that value is consumed by a single log line (`server.go:116`) and discarded — it is never persisted on concepts or questions, and the dequeue side (`tr ask`, MCP `recall_next`) never sends a repo at all.

The result: a developer working in repo Y who runs `tr ask` dequeues the oldest pending question, which may have been synthesized from repo X's concepts. In production this means recall questions are not scoped to the repository the developer is currently working within — the stated product goal. The single shared DB is also a friction point for testing, where a clean slate requires stopping the daemon and manually clearing the file.

## What Changes

- `concepts` and `questions` tables in `memory.db` gain a `repo TEXT NOT NULL DEFAULT ''` column plus covering indexes; existing legacy rows are purged on migration (the app is pre-production with a single user, so backward compatibility for old data is explicitly not a goal)
- `internal/cache/store.go` — every query method (`Save`, `Recent`, `SaveQuestion`, `NextQuestion`, `GetQuestion`, `AnswerQuestion`, `SkipQuestion`, `QueueDepth`, `PeekNextQuestion`, `RecentAnswered`) gains a `repo` parameter and scopes its SQL with `WHERE repo = ?`; legacy `concepts.db` migration logic is stripped (only `memory.db` is supported going forward)
- `internal/engine/server.go` `runPipeline` tags saved concepts and synthesized questions with `env.Repo`; `handleRecallNext` and `handleRecallAnswer` read an optional `repo` query param and pass it through to the store
- `internal/engine/mcp.go` — `recall_next`, `recall_answer`, and `recall_status` tools gain an optional `repo` input field; resource handlers (`recall://queue`, `recall://recent`) accept a repo hint and scope accordingly
- `cmd/total-recall/ask.go` — `tr ask` resolves the repo root via `hooks.FindRepoRoot()` and sends it as `?repo=<path>` to `/recall/next` and `/recall/answer`; when run outside a git repo, it falls back to global dequeue (`repo=""`) with a log advisory
- `TR_HOME` env override added to `internal/cache/store.go` `trDir()` and `internal/config` path resolution — when set, all `~/.tr` paths resolve under `$TR_HOME` instead of the user home; defaults to `~/.tr` when unset (fully backward-compatible); primary purpose is test/CI isolation so a test daemon can point at `t.TempDir()` and never touch the real `~/.tr`
- Logging: when a dequeue returns no rows for a non-empty repo key, the daemon logs an advisory hinting the repo may have been moved (repo key is the absolute path; moving a repo changes the key)

## Capabilities

### Modified Capabilities

- `concept-cache`: `concepts` table gains `repo` column; `Save` and `Recent` accept `repo`; legacy `concepts.db` migration stripped; `TR_HOME` override honored by `trDir()`
- `question-store`: `questions` table gains `repo` column; `SaveQuestion`, `NextQuestion`, `QueueDepth`, `PeekNextQuestion`, `RecentAnswered` accept `repo`; legacy data purged on migration
- `question-delivery`: `GET /recall/next` and `POST /recall/answer` accept optional `repo` query param; pass-through to store
- `mcp-server`: `recall_next`, `recall_answer`, `recall_status` tools gain optional `repo` input; `recall://queue` and `recall://recent` resources scope by repo
- `tr-ask`: `tr ask` resolves repo root and sends `?repo=`; falls back to global dequeue outside a git repo with a log advisory

### New Capabilities

- `tr-home-override`: `TR_HOME` environment variable redirects all `~/.tr` path resolution (config + memory.db) to an arbitrary directory; enables test/CI isolation

## Impact

- `internal/cache/store.go` — schema (repo column + indexes), all method signatures gain `repo`, legacy migration stripped, `trDir()` honors `TR_HOME`, purge on migration
- `internal/engine/server.go` — `runPipeline` tags with `env.Repo`; `handleRecallNext`/`handleRecallAnswer` read `repo` query param; repo-mismatch logging
- `internal/engine/mcp.go` — tool input structs gain optional `Repo *string`; resource handlers scope by repo
- `internal/recall/engine.go` — `Synthesize` accepts `repo` and passes to `store.Recent`
- `cmd/total-recall/ask.go` — resolve repo root, send `?repo=`, fallback + advisory
- `internal/config/config.go` — `UserConfigPath`/`UserConfigDir` honor `TR_HOME`
- `cmd/total-recall/*_test.go` — update cache/integration tests for repo-scoped behavior; add `TR_HOME`-based isolation tests
- `openspec/specs/{concept-cache,question-store,question-delivery,mcp-server,tr-ask}/spec.md` — updated to reflect repo scoping
- `openspec/specs/tr-home-override/spec.md` — new
- `AGENTS.md`, `ROADMAP.md`, `DOCS/ARCHITECTURE/`, `DOCS/TESTING/E2E.md` — synced with repo-scoped model and `TR_HOME`

## Key Design Decisions

- **Relational multi-tenant filtering (tenant = repo path), not per-repo DB files.** A `repo` column + `WHERE repo = ?` is the boring, correct relational pattern. At 229 concepts / 54 questions the covering indexes are free. Per-repo DB files would force a `StoreManager` re-architecture and break future cross-repo queries (spaced repetition). SQLite serves these indexed lookups in microseconds — decades from needing anything more.
- **Repo key = absolute filesystem path (`env.Repo`).** Available today with zero effort (the hook already sends it). Filesystem-layout leaking into the DB is a non-issue for a single-user, developer-machine-only tool. A moved repo changes the key; we log an advisory when a repo-keyed dequeue finds nothing, hinting at a possible move.
- **Empty string `repo` is the global pool.** `tr ask` outside a git repo (or an MCP client that omits `repo`) falls back to `repo=""` dequeue. Graceful degradation, not an error.
- **Legacy data purged on migration.** The app is pre-production with one user; carrying un-tagged rows forward as a "global" pool would perpetuate the leak. A clean `DELETE` on first open with the new schema is simpler and honest.
- **`TR_HOME` for test isolation, not production.** Production runs one daemon (port 7331 is hardcoded, single-daemon is the architecture) serving all repos through the repo column. `TR_HOME` lets tests/CI redirect `~/.tr` to a throwaway dir so they never touch real state. The two solutions compose: repo column fixes the production leak; `TR_HOME` fixes test friction.
- **MCP `repo` arg is optional.** Existing MCP clients that omit it get global dequeue (backward-compatible behavior). New clients that can supply workspace repo context get scoped results.
- **No Redis, no caching layer.** The workload is single-user, single-machine, write-on-commit/read-on-ask measured in requests per day. The 80KB SQLite file is already faster than a network round-trip to any cache. Adding Redis would be anti-performant here.

## Non-Goals

- Per-repo DB file partitioning (physical isolation) — the relational column approach is simpler and sufficient
- Backward compatibility for pre-existing `memory.db` rows — legacy data is purged on migration
- Supporting the legacy `concepts.db` filename — migration logic is stripped; only `memory.db` is supported
- Daemon autostart or multi-port/multi-daemon architectures
- Spaced repetition / cross-repo answer history aggregation (future phase)
- CI/CD deployment of Total Recall (the tool targets developer machines, not pipelines)
