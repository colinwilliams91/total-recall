## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: Repo column migration purges un-tagged legacy rows
**Reason**: In-code schema migrations have been removed entirely (Decision 2 in `cache-tenant-isolation/design.md`). The `cache.Open()` migration block (`addColumnIfMissing` for `repo` and the subsequent `DELETE FROM concepts` purge) is deleted. Fresh databases are created with the full schema including the `repo` and `branch` columns; existing databases belonging to the sole maintainer are deleted manually before starting the new daemon.
**Migration**: The maintainer (only user) runs `rm ~/.tr/memory.db` once. The next `tr serve` creates a fresh DB with the new schema. No automated upgrade path exists or is intended.