## MODIFIED Requirements

### Requirement: Concept cache is a SQLite database at ~/.tr/memory.db
The cache SHALL be stored in a single SQLite file at `~/.tr/memory.db`. `cache.Open()` SHALL create `~/.tr/` (or `$TR_HOME` when `TR_HOME` is set) if it does not exist and run schema migration on every open (idempotent `CREATE TABLE IF NOT EXISTS`). The legacy `concepts.db` filename is no longer supported — `Open()` SHALL NOT attempt any `concepts.db` → `memory.db` migration.

#### Scenario: First daemon start (no memory.db)
- **WHEN** `cache.Open()` is called and `~/.tr/memory.db` does not exist
- **THEN** the file is created, the schema is applied, and `Open()` returns a non-nil `*Store`

#### Scenario: Subsequent daemon start (memory.db exists)
- **WHEN** `cache.Open()` is called and `~/.tr/memory.db` already exists with data
- **THEN** `Open()` returns a non-nil `*Store` without modifying existing rows

#### Scenario: TR_HOME redirects the cache directory
- **WHEN** `TR_HOME=/tmp/tr-test` is set and `cache.Open()` is called
- **THEN** the store opens `/tmp/tr-test/memory.db`, creating `/tmp/tr-test/` if needed, and does not touch `~/.tr/`

---

### Requirement: Concepts are saved per-repo
`Store.Save(ctx, repo, concepts)` SHALL insert concept fingerprints with the `repo` column set to the provided repository path (the absolute path from `git rev-parse --show-toplevel`, supplied by the hook envelope). An empty `repo` string SHALL be stored as-is, representing the global pool.

#### Scenario: Save after extraction from a known repo
- **WHEN** `ExtractConcepts` returns `[]ConceptFingerprint` for a pre-commit hook from repo `/home/user/myproject`
- **THEN** `Store.Save` inserts each fingerprint with `repo = "/home/user/myproject"`

#### Scenario: Save with empty repo (global pool)
- **WHEN** `Store.Save` is called with `repo = ""`
- **THEN** each fingerprint is inserted with `repo = ""` and is retrievable by a global (`repo = ""`) `Recent` query

---

### Requirement: Recent query returns concepts scoped to a repo
`Store.Recent(ctx, repo, limit)` SHALL return up to `limit` concept fingerprints where `repo = ?`, ordered by `seen_at DESC` (most recent first). When `repo = ""`, it SHALL return the global pool.

#### Scenario: Recent concepts retrieved for synthesis
- **WHEN** the recall engine calls `Store.Recent` with `repo = "/home/user/myproject"` and `limit = 20`
- **THEN** the store returns the 20 most recent concept fingerprints for that repo only

#### Scenario: Global recent when repo is empty
- **WHEN** `Store.Recent` is called with `repo = ""`
- **THEN** the store returns the most recent concepts across the global pool (`repo = ""`)

---

### Requirement: Repo column migration purges un-tagged legacy rows
When `store.Open()` adds the `repo` column to an existing `concepts` table (idempotent `ALTER TABLE ADD COLUMN`), it SHALL subsequently `DELETE FROM concepts` to purge rows that have no repo tag. This purge SHALL run exactly once — gated on the column-add, which is itself idempotent. The purge SHALL be logged as `[store] purged un-tagged legacy rows during repo-scoping migration`.

#### Scenario: Existing database upgraded
- **WHEN** `Open()` is called on a `memory.db` that has `concepts` rows but no `repo` column
- **THEN** the `repo TEXT NOT NULL DEFAULT ''` column is added, all existing rows are deleted, and the purge is logged

#### Scenario: Already-migrated database
- **WHEN** `Open()` is called on a database that already has the `repo` column
- **THEN** no purge occurs and no purge log is emitted

---

### Requirement: TR_HOME environment variable overrides the default home directory
When the `TR_HOME` environment variable is set to a non-empty path, `cache.trDir()` SHALL use `$TR_HOME` as the Total Recall data directory instead of `~/.tr`. This redirects `memory.db` (and, via the parallel config change, `config.yaml`) to an arbitrary location. When `TR_HOME` is unset, behavior is unchanged (`~/.tr`).

#### Scenario: Test isolation via TR_HOME
- **WHEN** a test sets `TR_HOME=/tmp/test-tr-123` and starts a daemon
- **THEN** the daemon opens `/tmp/test-tr-123/memory.db` and never reads or writes `~/.tr/memory.db`

---

### Requirement: Store is closed on daemon graceful shutdown
`Store.Close()` SHALL be called as part of the daemon's graceful shutdown sequence, after in-flight goroutines are drained. This ensures no writes are lost.

#### Scenario: Clean shutdown
- **WHEN** the daemon receives SIGTERM and graceful shutdown completes
- **THEN** `store.Close()` is called and the SQLite connection is cleanly closed

---

### Requirement: modernc.org/sqlite is the only SQLite dependency
The cache implementation SHALL use `modernc.org/sqlite` (CGO-free pure-Go SQLite). No `mattn/go-sqlite3` or other CGo SQLite dependency SHALL be introduced.

#### Scenario: go.mod check
- **WHEN** the cache layer is inspected
- **THEN** `go.mod` contains `modernc.org/sqlite` and no CGo SQLite dependency
