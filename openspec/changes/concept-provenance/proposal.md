# Concept Provenance

## Why

Answer outcomes (`question_events` + `selections` + `choices.is_correct`) can grade *questions* today, but the unit a tutor actually needs to grade — the *concept* — is anonymous: `concepts` is an append-only fingerprint log with no stable identity ("Go interfaces" may appear as 40 rows across commits), and nothing links a question to the concepts it was synthesized from. This blocks every history-aware tutoring capability in one step: question-topic dedupe ("don't re-ask this"), per-concept mastery ("which ideas has he missed twice?"), and spaced-repetition scheduling — all of which the `adaptive-difficulty` change explicitly deferred to future history-aware resolvers. Pre-production is the right time to fix the schema: no migration path exists (`CREATE TABLE IF NOT EXISTS` + manual teardown), so schema debt taken now is paid at the cost of deleting `memory.db`, an amount of pain we currently price at zero.

## What Changes

**Two-layer identity model (the core structural change):**

- **User-scoped identity layer (new table `concept_registry`):** a global concept registry keyed by canonical slug, one per user (single-user product: the DB belongs to one user's brain). Scoping rules for this layer are user-level — a concept first seen on `repo-a/feat-x` is the *same* concept on `repo-b/main`. Slug minting happens at extraction time: the extraction AI call (already in the hot pipeline, already a background task) gains a contract extension — it now emits `(slug, prose, weight, source)` where `slug` is a machine-readable canonical identity (`[a-z0-9_]+`) and `prose` remains the free-form fingerprint text. No second AI call, no read-time fuzz.
- **Repo/branch-scoped sightings (modified `concepts` table):** the existing fingerprint log keeps its per-repo/per-branch scoping untouched, gaining an FK to the registry (slug FK). The two-layer split keeps the branch-scoping promise intact where it is load-bearing (sightings, log) and moves mastery to the level memory actually lives (user).
- **Exact-match identity:** matching is exact after canonicalization. No fuzzy similarity, no AI-merge pass. A `concept_aliases` table is included as a mutable pressure valve — an explicit `alias → target` lookup, *not* a merge engine — with a scoped deep-dive required before implementation (see open item below).

**Provenance join (new table `question_concepts`):** written at synthesis time, linking each persisted question to the concepts it drew on: `question_concepts(question_id, concept_slug, weight)`. Written by the engine from the synthesis inputs (loose provenance: "synthesized from this batch of recent sightings"), not from a new AI contract — the concepts fed to `SynthesisContext` are already known and authoritative about what drove the synthesis.

**Selection priority ordering (spec-level constraint):** the concept feed for synthesis is always the user's current working surface first (current repo/branch, recent sightings). Mastery *modulates selection within that surface*; it never promotes out-of-repo concepts ahead of it. Working-diff relevance is first-class, tutor scheduling is second-class.

**BREAKING:** schema reshape of `memory.db`. Pre-production: users delete `$TR_HOME/memory.db` (or `~/.tr/memory.db`) after this build lands, consistent with the no-migration-path convention in `internal/cache/MIGRATION.md`.

## Capabilities

### New Capabilities
- `concept-identity`: the user-scoped identity layer — `concept_registry` keyed by canonical slug, slug minted by the extraction AI in the existing extraction call, exact-match identity semantics, the `concept_aliases` pressure-valve table, and write-boundary validation (slug-shaped strings only; malformed slugs rejected loudly at the pipeline layer, never silently fragmented).

### Modified Capabilities
- `concept-cache`: `concepts` stays repo/branch-scoped per existing requirements but gains the registry FK (sightings layer); schema reshape into two-layer model; brand-new `question_concepts` provenance join written atomically with `SaveQuestion`; scope-refusal requirements preserved for the sighting layer.
- `concept-extraction`: extraction AI contract extended to emit canonical slugs alongside prose; malformed or missing slugs are a graceful-degradation case consistent with the existing extraction-failure requirement.
- `recall-engine`: `SynthesisContext` enrichment and provenance capture — the engine records the concept batch it synthesized from into `question_concepts` alongside the question and its choices (same transaction as `SaveQuestion`). Feeding/priority requirements: current working surface first, mastery as second-class modifier only.

## Impact

- **Code:** `internal/cache/store.go` (schema reshape: `concept_registry`, `concepts` FK, `concept_aliases`, `question_concepts`; `Save` writes through registry; transactional provenance insert), `internal/pipeline/extraction.go` + `ExtractionRequest` prompt (`ConceptFingerprint` gains `Slug`), `internal/recall/engine.go` (records provenance at synthesis), `internal/engine/server.go` (pipeline passes registry-aware concepts), `internal/cache/MIGRATION.md`.
- **Tests:** `cmd/torec/cache_test.go` (identity dedupe, alias resolution, schema reshape), `cmd/torec/main_test.go` (extraction contract), pipeline and recall-level tests for provenance capture, integration tests in `integration_test.go`.
- **Prompt assets:** `assets/prompts/concept-extraction.md` (or the extraction prompt section) gains the slug-emit contract.
- **Dependencies:** None new. No schema migration path — manual teardown per convention.
- **Sequencing:** independent of `adaptive-difficulty` (stateless heuristics, no shared writes); landable in either order. Two explicitly deferred follow-ups tracked here for coordination:
  1. **Observational audit** *(after `adaptive-difficulty` lands)*: dump resolved difficulty + used concepts into `question_events.payload` — cheap, write-path-only, and partially redundant with this change's `question_concepts` once it lands.
  2. **`concept_aliases` deep-dive** *(before/within this change's implementation)*: current framing is honest wishful thinking — the table needs real planning (who writes aliases: user command vs. automated dedup pass; how alias resolution composes with lookup without becoming a fuzzy system; validation of alias targets) before it ships.
