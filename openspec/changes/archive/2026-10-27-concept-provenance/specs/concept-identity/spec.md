## Purpose

Give each concept a single stable, user-scoped identity so concept-level tutoring (mastery, dedupe, spaced repetition) can operate across repos and branches: a slug registry that the extraction AI mints at write time, exact-match identity semantics, a bounded alias pressure valve, and loud validation at the write boundary.

## ADDED Requirements

### Requirement: Concept identity is user-scoped and global across repos and branches
The concept identity layer SHALL maintain one canonical identity per distinct concept for the entire store, keyed by a canonical slug. Identity is NOT scoped per-repo or per-branch: a concept first observed on one repo/branch SHALL be the same identity when observed on another. This departs from the per-repo/per-branch scoping of the fingerprint log; the single-user nature of the store is what makes global identity correct (the DB belongs to one user's brain).

#### Scenario: Same concept seen on different repos shares one identity
- **WHEN** a sighting with canonical slug `go_interfaces` is recorded for repo A/branch main, and later a sighting with the same slug is recorded for repo B/branch feat-db
- **THEN** both sightings reference the same registry identity; the registry contains one row for `go_interfaces` with the first-seen timestamp unchanged

---

### Requirement: Canonical slug is minted by the extraction AI at write time
Concept identity SHALL be minted at extraction time: the extraction AI call emits a `slug` field alongside the existing `concept` prose, and the store persists the sighting linked to the registry entry for that slug at `Store.Save` time. No read-time slug derivation, no fuzzy matching, and no second AI call SHALL be used. Slug minting rides the existing extraction call — it adds contract surface, not latency (the call is already an async background task; the hook response is unaffected).

#### Scenario: Extraction emits a slug and the store resolves it
- **WHEN** `ExtractConcepts` returns a fingerprint with slug `go_interfaces` and prose "interface satisfaction via implicit methods", and `Store.Save` persists it for repo X/branch Y
- **THEN** a sighting row exists linked to the registry entry with slug `go_interfaces`, and the prose lives on the sighting, not the identity

#### Scenario: Slug minting does not block the hook
- **WHEN** the extraction AI emits slugs as part of the same call that already emits concepts
- **THEN** the hook's 202 Accepted response latency is unchanged; no additional provider request is made for identity

---

### Requirement: Identity matching is exact after canonicalization
Identity resolution SHALL be exact-match on the canonical slug. No similarity heuristics, no model-assisted merging, and no platform-dependent matching SHALL be introduced. Where phrasing variance genuinely collides, it SHALL be resolved through explicit aliases (see requirement below), never through fuzzy lookup at read time.

#### Scenario: Identical slugs dedupe; different slugs do not merge
- **WHEN** the store resolves identity for slug `sql_joins`
- **THEN** only registry entries whose slug is exactly `sql_joins` match; a sighting recorded as `joining_tables_in_sql` is a different identity until an alias row or user action says otherwise

---

### Requirement: Malformed slugs are rejected loudly at the write boundary
The system SHALL validate at the write boundary that every persisted slug matches a canonical shape (`[a-z0-9_]+`). A slug that leaks prose-shaped text (spaces, mixed case, punctuation) SHALL be rejected with a logged warning and the affected sighting SHALL be skipped, rather than silently persisting a fragmented identity. Validation failures degrade the sighting, never the commit pipeline.

#### Scenario: Prose-shaped slug rejected
- **WHEN** the extraction AI emits slug `"Handle HTTP Middleware Chaining"` for a concept
- **THEN** the sighting is not persisted; a warning naming the offending slug is logged and the rest of the batch is saved normally

#### Scenario: Missing slug degrades like extraction failure
- **WHEN** the extraction AI omits the `slug` field for an element
- **THEN** that element is skipped with a warning; the pipeline continues without crashing

---

### Requirement: Aliases are an explicit pressure valve, not a matching system
An alias table SHALL map an explicit alternate slug onto an existing registry slug. Alias resolution SHALL occur during identity resolution at write time (a sighting whose slug is an alias lands on the target identity). Aliases SHALL be exact-match lookups — adding an alias never introduces fuzziness. The alias source SHALL be validated at write time: an alias target MUST already exist in the registry for its repo scope. The authoring surface for aliases MAY begin as a manual/user-command flow; automated alias generation is out of scope for this change (see design open item — the alias *writer* experience needs dedicated planning).

#### Scenario: Alias resolution at write time
- **WHEN** alias `joining_tables_in_sql` → `sql_joins` exists and a sighting arrives with slug `joining_tables_in_sql`
- **THEN** the sighting is persisted against the `sql_joins` registry identity; no new registry row is created for the alias

#### Scenario: Alias targeting a nonexistent identity is rejected
- **WHEN** an alias row is created targeting a slug absent from the registry
- **THEN** the alias creation is rejected with an explicit error; no dangling alias rows exist

---

### Requirement: Selection feeds the working surface first; mastery is second-class
Current working-surface relevance SHALL be the first-class input to concept selection for synthesis: recent sightings on the user's current repo and branch. Concept-level outcome history (mastery signals enabled by identity) SHALL only modulate selection within that surface — it SHALL NOT promote out-of-surface concepts ahead of the user's current work. A tutor mechanism that ranks "most overdue concept" globally ahead of concept feed boundaries violates this requirement.

#### Scenario: Overdue concept on another repo does not outrank current branch work
- **WHEN** concept `go_interfaces` has poor performance history but has no recent sightings on the current repo/branch, while fresh high-weight concepts exist for the current repo/branch
- **THEN** the synthesis concept feed is drawn from the current repo/branch sightings; `go_interfaces` is not injected ahead of them on the basis of mastery state
