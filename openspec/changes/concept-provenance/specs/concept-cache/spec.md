## ADDED Requirements

### Requirement: Question provenance join records the concepts a question drew on
`Store.SaveQuestion` SHALL insert, in the same transaction as the question and its choices, one `question_concepts` row per concept fed to synthesis, linking the question to the registry identity of each concept (`question_id`, registry slug reference, and the sighting weight at synthesis time). Provenance is written for questions in any status; it is the durable record of "what fed this question" and is the basis for future concept-level outcome grading.

#### Scenario: Provenance captured atomically with the question
- **WHEN** `Synthesize` succeeds and `SaveQuestion` persists a question whose `SynthesisContext` carried 3 concepts
- **THEN** a question row, its choice rows, a `queued` event, and 3 `question_concepts` rows are all committed in one transaction; a rejected `SaveQuestion` leaves no partial provenance rows

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
