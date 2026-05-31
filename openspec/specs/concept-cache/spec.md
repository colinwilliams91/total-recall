## Requirements

### Requirement: Concept cache is a SQLite database at ~/.tr/cache.db
The cache SHALL be stored in a single SQLite file at `~/.tr/cache.db`. `cache.Open()` SHALL create `~/.tr/` if it does not exist and run schema migration on every open (idempotent `CREATE TABLE IF NOT EXISTS`).

#### Scenario: First daemon start (no cache file)
- **WHEN** `cache.Open()` is called and `~/.tr/cache.db` does not exist
- **THEN** the file is created, the schema is applied, and `Open()` returns a non-nil `*Store`

#### Scenario: Subsequent daemon start (cache file exists)
- **WHEN** `cache.Open()` is called and `~/.tr/cache.db` already exists with data
- **THEN** `Open()` returns a non-nil `*Store` without modifying existing rows

---

### Requirement: Concepts are saved per-repo, per-branch
`Store.Save` SHALL insert concept fingerprints with `repo` and `branch` metadata so that `Store.Recent` can scope results to the active repo.

#### Scenario: Save after extraction
- **WHEN** `ExtractConcepts` returns `[]ConceptFingerprint` for a pre-commit hook from repo `/home/user/myproject` on branch `feature/retry-logic`
- **THEN** `Store.Save` inserts each fingerprint with the correct `repo` and `branch` values

---

### Requirement: Recent query returns concepts ordered by recency
`Store.Recent(ctx, repo, limit)` SHALL return up to `limit` concept fingerprints for `repo`, ordered by `created_at DESC` (most recent first).

#### Scenario: Recent concepts retrieved for synthesis
- **WHEN** the recall engine calls `Store.Recent` with `limit = 20`
- **THEN** the store returns the 20 most recent concept fingerprints for that repo

---

### Requirement: Store is closed on daemon graceful shutdown
`Store.Close()` SHALL be called as part of the daemon's graceful shutdown sequence, after in-flight goroutines are drained. This ensures no writes are lost.

#### Scenario: Clean shutdown
- **WHEN** the daemon receives SIGTERM and graceful shutdown completes
- **THEN** `store.Close()` is called and the SQLite connection is cleanly closed

---

### Requirement: modernc.org/sqlite is the only new external dependency
The cache implementation SHALL use `modernc.org/sqlite` (CGO-free pure-Go SQLite). No other new external dependencies are introduced in Phase 3 for the cache layer.

#### Scenario: go.mod after Phase 3
- **WHEN** Phase 3 is complete
- **THEN** `go.mod` contains `modernc.org/sqlite` and no other new direct dependencies beyond what Phase 2 introduced
