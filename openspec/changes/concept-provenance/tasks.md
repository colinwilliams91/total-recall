# Tasks: Concept Provenance

## 1. Schema reshape (store layer)

- [ ] 1.1 Add `concept_registry` (slug PK `[a-z0-9_]+`, prose, first_seen) and `concept_aliases` (alias, target) alongside the existing tables in `internal/cache/store.go` schema init; verify `go build ./...` and a `cache_test.go` case opening a fresh store shows all three new tables present via `sqlite_master`
- [ ] 1.2 Reshape `concepts` into a sightings table: add `slug TEXT NOT NULL REFERENCES concept_registry(slug)`; keep `repo`/`branch`/`source`/`weight`/`seen_at` semantics untouched; update `MIGRATION.md` with the delete-`memory.db` teardown instruction; verify `setupCache(t)` tests still pass after deleting `TR_HOME/memory.db`
- [ ] 1.3 Add `question_concepts` (question_id FK cascade, slug FK, weight snapshot) with an index on `question_id`; verify a `cache_test.go` round-trip inserting and reading provenance rows

## 2. Identity resolution at the write boundary

- [ ] 2.1 Implement validation + resolution in `Store.Save`: reject slugs that fail `^[a-z0-9_]+$` per-element (drop element, logged warning, batch continues); resolve exact slug, mint registry row on first sighting, reuse (first_seen untouched) on subsequent; verify with table-driven `cache_test.go` cases for valid/malformed/missing-slug/first-vs-repeat sighting
- [ ] 2.2 Preserve and prove scope-refusal ordering: `Save` with empty repo or branch logs the skip and performs NO registry insert (identity is never minted from scope-failed writes); add the test case asserting registry row count is unchanged
- [ ] 2.3 Implement alias resolution on the `Save` path (alias → target before registry check; alias targeting a nonexistent registry slug is rejected at alias-write time); verify via `cache_test.go`: sighting under an alias lands on the target identity, and a mint-time slug colliding with an existing alias resolves to the alias target (the edge case flagged as open question #3, mandatory test case)

## 3. Extraction contract (slug minting)

- [ ] 3.1 Add `Slug string` to `ConceptFingerprint` and the extraction prompt's JSON contract (_slug alongside concept/source/weight, with stable-general-name instruction); verify `pipeline` unit tests parse the extended shape and drop slug-less elements with a warning
- [ ] 3.2 Update the extraction prompt asset and any inline fallback template to the new contract; verify with a provider-stub diff fixture end-to-end through `ExtractConcepts` → `Store.Save` that the registry gains a slug and a sighting links to it (first vertical-slice e2e validation)

## 4. Provenance capture (synthesis → storage)

- [ ] 4.1 Extend `SaveQuestion` to accept the fed concept batch (slug + weight pairs) and insert `question_concepts` rows in the SAME transaction as the question/choices/queued-event; verify `cache_test.go`: rejected `SaveQuestion` (e.g., empty question) leaves zero partial provenance rows
- [ ] 4.2 Thread the concept batch from `runPipeline`'s `SynthesisContext` through the engine into `SaveQuestion`; verify an integration test (`startTestDaemon` + seeded store) shows a synthesized question carrying N provenance rows matching the N fed concepts, and a failed synthesis (stubbed AI error) carries none

## 5. Reads surface identity

- [ ] 5.1 Extend `ConceptRow`/`Recent` reads to carry the resolved slug next to prose; verify existing `Recent` scoping tests pass unchanged plus a new case proving slug round-trips
- [ ] 5.2 Confirm feed semantics unchanged at the recall layer: `SynthesisContext` concepts still come exclusively from branch-scoped sightings (working-surface-first); add a regression test asserting no out-of-surface concept enters the feed even when the registry contains sightings from other repos

## 6. Alias CLI surface (deliberately late)

- [ ] 6.1 Add daemon route + store method for alias add with validation (slug-shaped alias, target must exist in registry) returning an explicit error on a dangling target; verify integration test rejects `alias add` with nonexistent target and accepts a valid one, and that `alias` rows enable resolution in a subsequent `Save` call
- [ ] 6.2 Add `torec alias add|list|remove` as thin cobra commands hitting the daemon route (mirror `asset`/`config` client patterns); verify `cmd/torec` tests cover flag parsing, daemon error surfacing (dangling target, invalid slug shape), and `alias list` output ordering

## 7. Docs and final verification

- [ ] 7.1 Update AGENTS.md cache section (new tables, two-layer scoping note, manual-migration reminder) and confirm `docs/ARCHITECTURE/` store docs match the two-layer model; also correct AGENTS.md's Manual e2e section, which became stale when adaptive-difficulty landed `adaptive-ab.{sh,ps1}` (listed in `scripts/e2e/README.md`); verify both modified doc sets render and reference the finalized table names
- [ ] 7.2 Full verification pass: `go build ./... && go vet ./... && go test ./...` green, `golangci-lint run` green, and a manual memory.db delete → fresh `serve` → commit → `ask` walkthrough showing the full flow (slugs minted, question + provenance persisted, alias repair merges history in listing)
- [ ] 7.3 Run `scripts/e2e/adaptive-ab.sh ./bin/torec` (manual, real-AI provider required; complements 7.2's stub-level pass) and verify identity + provenance behavior under repeated identical commits: after any arm's 10 same-payload POSTs, `concept_registry` holds exactly one row per distinct concept (`first_seen` untouched across sightings) while sightings append, and every captured question in `$TR_HOME/arms/<arm>.txt` is backed by `question_concepts` rows; spot-check the high-cluster/all-code arms' near-miss phrasings for slug drift (e.g., `go_interfaces` vs `golang_interfaces`) in the registry, recording any drift as alias candidates
