## Context

Total Recall's memory store is a single SQLite file (`~/.tr/memory.db`) shared across every repository a developer works in. The hook ingestion side already receives repo identity (`env.Repo`, the absolute path from `git rev-parse --show-toplevel`), but discards it after a log line. The dequeue side (`tr ask`, MCP tools) never sends a repo at all. This creates a cross-repo leak: a question synthesized from repo X's concepts can be served to a developer working in repo Y.

```
INGESTION (has repo)                    CONSUMPTION (lacks repo)
─────────────────────                   ────────────────────────
git hook in repo X                      tr ask in repo Y
  env.Repo = "/path/to/X"                 (no repo sent)
  ▼                                       ▼
POST /hooks/pre-commit                  GET /recall/next
  ▼                                       ▼
runPipeline                             handleRecallNext
  store.Save(fp)      ← no repo           store.NextQuestion("shell")
  recall.Synthesize() ← Recent(20) global   WHERE delivered_at IS NULL
  store.SaveQuestion(q) ← no repo           ORDER BY queued_at ASC  ← GLOBAL
  ▼                                       ▼
question queued globally                oldest Q (from X) served to Y
```

The fix closes both ends: tag rows with `repo` at ingestion, filter by `repo` at consumption.

## Design

### 1. Schema: repo column + indexes

```sql
ALTER TABLE concepts  ADD COLUMN repo TEXT NOT NULL DEFAULT '';
ALTER TABLE questions ADD COLUMN repo TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_concepts_repo_seen  ON concepts(repo, seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_questions_repo_q    ON questions(repo, queued_at ASC) WHERE delivered_at IS NULL;
```

The partial index on `questions(repo, queued_at ASC) WHERE delivered_at IS NULL` is exactly the access path `NextQuestion` uses — repo-scoped, oldest-first, unclaimed only. At current scale (hundreds of rows) these are free; they're correct for any future scale this tool will see.

### 2. Migration: purge legacy, strip concepts.db

`store.Open()` currently has a one-time migration guard that copies `concepts.db` → `memory.db`. Per the preface, legacy support is dropped:

1. Remove the `legacyDBFilename` constant and the `copyFile` migration block.
2. On first open with the new binary, if the `repo` column does not exist on either table, add it via the existing idempotent `addColumnIfMissing` helper, then `DELETE FROM concepts; DELETE FROM questions;` to purge un-tagged legacy rows.
3. Log `[store] purged un-tagged legacy rows during repo-scoping migration`.

The purge is a one-time event gated on the column-add (which itself is idempotent), so it runs exactly once per upgraded database.

### 3. Store method signatures

Every query method gains a `repo string` parameter. Empty `repo` matches the global pool (rows with `repo = ''`), which is the fallback for `tr ask` outside a git repo.

```go
func (s *Store) Save(ctx context.Context, repo string, concepts []Fingerprint) error
func (s *Store) Recent(ctx context.Context, repo string, n int) ([]ConceptRow, error)
func (s *Store) SaveQuestion(ctx context.Context, repo, question string, choices []string, correctIndex int) error
func (s *Store) NextQuestion(ctx context.Context, repo, claimedBy string) (*StoredQuestion, error)
func (s *Store) GetQuestion(ctx context.Context, id int64) (*StoredQuestion, error)   // unchanged — ID is globally unique
func (s *Store) AnswerQuestion(ctx context.Context, id int64, ...) error               // unchanged — ID-keyed
func (s *Store) SkipQuestion(ctx context.Context, id int64) error                      // unchanged — ID-keyed
func (s *Store) QueueDepth(ctx context.Context, repo string) (int, error)
func (s *Store) PeekNextQuestion(ctx context.Context, repo string) (*StoredQuestion, error)
func (s *Store) RecentAnswered(ctx context.Context, repo string, limit int) ([]StoredQuestion, error)
```

`GetQuestion`, `AnswerQuestion`, and `SkipQuestion` remain ID-keyed (the AUTOINCREMENT PK is globally unique across repos), so they don't need a `repo` param. The repo scoping matters for *selection* (which question to surface), not for *acting on* a question the caller already holds by ID.

### 4. NextQuestion — repo-scoped atomic dequeue

```sql
UPDATE questions
SET delivered_at = datetime('now'), claimed_by = ?
WHERE id = (
  SELECT id FROM questions
  WHERE delivered_at IS NULL AND repo = ?
  ORDER BY queued_at ASC
  LIMIT 1
)
RETURNING id, question, choices, correct_index, queued_at
```

The `AND repo = ?` is the entire change to the atomic dequeue. SQLite's serialized writes preserve the exactly-once guarantee per repo.

### 5. Pipeline: tag with env.Repo

```
sequenceDiagram
    participant Hook as git hook (repo X)
    participant Daemon as engine.Server
    participant Store as cache.Store
    participant AI as ai.Provider

    Hook->>Daemon: POST /hooks/pre-commit {repo:"/path/X", diff:"..."}
    Daemon->>Daemon: handleHook — log repo, spawn runPipeline(env)
    Daemon->>AI: ExtractConcepts(diff)
    AI-->>Daemon: []ConceptFingerprint
    Daemon->>Store: Save(ctx, env.Repo, fingerprints)
    Daemon->>AI: Synthesize(ctx, env.Repo, model)
    Note over AI: Synthesize calls Store.Recent(ctx, env.Repo, 20)
    AI-->>Daemon: *Question
    Daemon->>Store: SaveQuestion(ctx, env.Repo, q.Question, q.Choices, q.CorrectIndex)
    Daemon->>Daemon: NotifyResourceUpdated("recall://queue")
```

`recall.Engine.Synthesize` gains a `repo` parameter forwarded to `store.Recent`.

### 6. Consumption: tr ask resolves repo

```
tr ask
  ├─ term.IsTerminal? no → exit 0
  ├─ hooks.FindRepoRoot()
  │    ├─ ok → repo = "/path/to/current/repo"
  │    └─ err (not a git repo) → repo = "" (global fallback)
  │         └─ log: "[ask] not inside a git repo — falling back to global recall queue"
  ├─ poll: GET /recall/next?repo=<url-encoded repo>
  └─ answer: POST /recall/answer?repo=<...>&feedback=true
```

When `repo=""` and the dequeue returns nothing, `tr ask` already shows the caught-up message — no new UX needed. The daemon additionally logs the repo-mismatch advisory when a non-empty repo key yields no rows (hinting at a possible repo move).

### 7. MCP: optional repo arg

```go
type recallNextIn struct {
    Repo *string `json:"repo,omitempty"`
}
```

- `repo` omitted or nil → global dequeue (backward-compatible with existing MCP clients)
- `repo` provided → scoped dequeue
- `recall_status` and resources scope `QueueDepth`/`PeekNextQuestion`/`RecentAnswered` by the supplied repo (or global when absent)

### 8. TR_HOME env override

```go
func trDir() (string, error) {
    if env := os.Getenv("TR_HOME"); env != "" {
        if err := os.MkdirAll(env, 0o700); err != nil {
            return "", fmt.Errorf("creating TR_HOME directory: %w", err)
        }
        return env, nil
    }
    home, err := os.UserHomeDir()
    // ... existing ~/.tr logic
}
```

Same override added to `config.UserConfigPath()` and `config.UserConfigDir()`. A single `TR_HOME` redirects both config and memory.db, keeping them co-located (consistent with the `~/.tr` layout). Tests set `t.Setenv("TR_HOME", t.TempDir())` to get full isolation.

### 9. Repo-key move detection (logging only)

When `NextQuestion(repo="/old/path")` returns `nil, nil` and `QueueDepth(repo="")` is also 0 for that repo, the daemon logs:

```
[recall] no pending questions for repo="/old/path" — if you recently moved this repo, its key has changed
```

This is advisory only; no corrective action is taken automatically. Concepts from the old path remain in the DB under the old key and will age out naturally as recall focuses on the new path's concepts.

## Risks

- **[Risk] Repo path changes on move/clone** — the absolute path is the key, so moving a repo orphans its history under the old path. Mitigation: advisory logging (§9); pre-production single-user means the blast radius is "re-learn concepts in the new location," not data loss. A future `tr rekey` command could remap if needed.
- **[Risk] Signature churn across the codebase** — every `Store` method gains a `repo` param, touching the engine, recall, MCP, and tests. Mitigation: mechanical change; the compiler enforces completeness.
- **[Risk] MCP clients don't supply repo** — existing clients get global dequeue, which is the current behavior. Not a regression; scoped behavior is opt-in via the new optional field.
