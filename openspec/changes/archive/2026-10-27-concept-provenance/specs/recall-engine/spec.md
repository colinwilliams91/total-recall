## ADDED Requirements

### Requirement: Synthesis records provenance of the concepts it drew on
When a synthesis call yields a question that the pipeline persists via `SaveQuestion`, the recall engine SHALL also record the concept batch from `SynthesisContext` (the concepts fed to that synthesis, with their weights) as provenance rows in the same transaction as the question. Provenance SHALL be derived from engine inputs (the feed), not from a new AI contract — the concepts in `SynthesisContext` are the authoritative record of what drove the synthesis.

#### Scenario: Successful synthesis yields provenance
- **WHEN** `Synthesize` returns a question from a `SynthesisContext` carrying 3 concept rows and the pipeline persists it
- **THEN** 3 provenance rows exist for that question, one per fed concept, carrying each concept's weight at synthesis time

#### Scenario: Synthesis failure leaves no provenance
- **WHEN** a synthesis AI call fails and `Synthesize` returns `nil, nil`
- **THEN** no question row and no provenance rows are written; the pipeline simply skips

---

### Requirement: Synthesis concept feed remains working-surface-first
The concept feed built into `SynthesisContext` SHALL be drawn from sightings on the current repo and branch first (preserving the existing per-branch `Recent` semantics). Identity-layer history (aliases, user-level mastery signals from provenance joins) MAY refine ordering within that feed but SHALL NOT widen it across repos or branches. This makes the priority rule (`working-diff relevance is first-class; tutoring is second-class`) enforceable at the store layer rather than leaving it to the resolver.

#### Scenario: Feed is still scoped to the triggering repo and branch
- **WHEN** the pipeline builds `SynthesisContext` for repo X/branch Y on a commit
- **THEN** every concept in the feed has a sighting on repo X/branch Y, regardless of what the identity layer knows about other repos
