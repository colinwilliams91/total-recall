## Purpose

Persist concept fingerprints extracted from staged diffs in a SQLite database at `~/.tr/memory.db`, scoped per repo and per branch so recall is tenant-isolated to the working branch.
## Requirements
### Requirement: Concept cache is a SQLite database at ~/.tr/memory.db
The cache SHALL be stored in a single SQLite file at `~/.tr/memory.db`. `cache.Open()` SHALL create `~/.tr/` (or `$TR_HOME` when `TR_HOME` is set) if it does not exist and run schema initialization on every open (idempotent `CREATE TABLE IF NOT EXISTS`). No in-code schema migration (no `ALTER TABLE ADD COLUMN`, no row purge, no column existence check) SHALL run inside `Open()`. The legacy `concepts.db` filename is no longer supported — `Open()` SHALL NOT attempt any `concepts.db` → `memory.db` migration.

#### Scenario: First daemon start (no memory.db)
- **WHEN** `cache.Open()` is called and `~/.tr/memory.db` does not exist
- **THEN** the file is created with the full final schema (including `repo` and `branch` columns on `concepts` and `questions`), and `Open()` returns a non-nil `*Store`

#### Scenario: Subsequent daemon start (fresh-schema memory.db exists)
- **WHEN** `cache.Open()` is called and `~/.tr/memory.db` already exists with the current schema and data
- **THEN** `Open()` returns a non-nil `*Store` without modifying existing rows and without running any migration code path

#### Scenario: TR_HOME redirects the cache directory
- **WHEN** `TR_HOME=/tmp/tr-test` is set and `cache.Open()` is called
- **THEN** the store opens `/tmp/tr-test/memory.db`, creating `/tmp/tr-test/` if needed, and does not touch `~/.tr/`

---

### Requirement: Concepts are saved per-repo and per-branch
`Store.Save(ctx, repo, branch, concepts)` SHALL insert concept fingerprints with both the `repo` column and the `branch` column set to the values supplied by the hook envelope (the absolute repo path from `git rev-parse --show-toplevel` and the branch name from `git rev-parse --abbrev-ref HEAD`). Both `repo` and `branch` MUST be non-empty. If either is empty, `Save` SHALL return early (no-op) without inserting and SHALL log `[store] skipping insert: empty repo or branch`. No "global pool" semantics exist — empty values are not stored.

#### Scenario: Save after extraction from a known repo and branch
- **WHEN** `ExtractConcepts` returns `[]ConceptFingerprint` for a pre-commit hook from repo `/home/user/myproject` on branch `feature-X`
- **THEN** `Store.Save` inserts each fingerprint with `repo = "/home/user/myproject"` and `branch = "feature-X"`

#### Scenario: Save with empty repo refuses to insert
- **WHEN** `Store.Save` is called with `repo = ""`
- **THEN** no rows are inserted; the method returns `nil`; the daemon logs `[store] skipping insert: empty repo or branch`

#### Scenario: Save with empty branch (detached HEAD) refuses to insert
- **WHEN** `Store.Save` is called with `branch = ""` (because `git rev-parse --abbrev-ref HEAD` returned empty during a rebase or detached checkout)
- **THEN** no rows are inserted; the method returns `nil`; the daemon logs `[store] skipping insert: empty repo or branch`

---

### Requirement: Recent query returns concepts scoped to a repo and branch
`Store.Recent(ctx, repo, branch, limit)` SHALL return up to `limit` concept fingerprints where `repo = ? AND branch = ?`, ordered by `seen_at DESC` (most recent first). Both `repo` and `branch` MUST be non-empty. If either is empty, `Recent` SHALL return `(nil, nil)` without executing a query and without error.

#### Scenario: Recent concepts retrieved for synthesis on a specific branch
- **WHEN** the recall engine calls `Store.Recent` with `repo = "/home/user/myproject"` and `branch = "feature-X"` and `limit = 20`
- **THEN** the store returns the 20 most recent concept fingerprints for that repo AND branch only

#### Scenario: Empty repo or branch refuses to query
- **WHEN** `Store.Recent` is called with `repo = ""` or `branch = ""`
- **THEN** the method returns `(nil, nil)` and logs `[store] skipping recent: empty repo or branch`

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

---

### Requirement: Question provenance join records the concepts a question drew on
`Store.SaveQuestion` SHALL insert, in the same transaction as the question and its choices, exactly one `question_concepts` row per DISTINCT concept fed to synthesis, linking the question to the registry identity of each concept (`question_id`, registry slug reference, and the sighting weight at synthesis time). The distinctness SHALL be enforced by a `UNIQUE(question_id, slug)` constraint — when the fed batch carries a concept more than once (the sighting feed is row-based, so repeated commits on the same idea appear multiple times), the surviving row keeps the MAXIMUM weight from the batch. Provenance is written for questions in any status; it is the durable record of "what fed this question" and is the basis for future concept-level outcome grading. Feed multiplicity itself SHALL NOT be preserved in provenance — sighting counts remain readable from the sightings table.

#### Scenario: Provenance captured atomically with the question
- **WHEN** `Synthesize` succeeds and `SaveQuestion` persists a question whose `SynthesisContext` carried 3 distinct concepts
- **THEN** a question row, its choice rows, a `queued` event, and 3 `question_concepts` rows are all committed in one transaction; a rejected `SaveQuestion` leaves no partial provenance rows

#### Scenario: Repeated sightings of one concept collapse to a single row
- **WHEN** the fed batch contains the same slug 4 times (four sightings of one concept in the recent feed), with weights 0.9, 0.9, 0.4, 0.9
- **THEN** the question carries exactly 1 `question_concepts` row for that slug with the maximum weight (0.9); no identical `(question_id, slug)` duplicate rows exist

#### Scenario: Question with empty concept context
- **WHEN** a question is saved with an empty concept batch (possible only via paths that bypass the normal synthesis feed)
- **THEN** the question persists with zero `question_concepts` rows rather than erroring

---

### Requirement: Sighting writes resolve identity through the registry
`Store.Save` SHALL persist each concept sighting linked to its registry identity (resolved by exact slug, aliases included), keeping the existing per-repo/per-branch sighting columns (`repo`, `branch`) and all existing scope-refusal behavior unchanged. A concept whose identity is new to the registry SHALL be inserted on first sighting. Existing existing-scope guarantees (empty repo/branch refusals) are preserved and extended to identity resolution, which SHALL not change the scoping semantics of the log.

#### Scenario: First sighting of a new concept mints the registry identity
- **WHEN** `Store.Save` persists a sighting with slug `vector_embeddings` and the registry has no such slug
- **THEN** the registry gains a `vector_embeddings` row and the sighting links to it with `repo` and `branch` populated as before

#### Scenario: Scope refusal preserved for registry writes
- **WHEN** `Store.Save` is called with empty repo or branch
- **THEN** no sightings and no registry rows are written; the store logs the existing skip message

