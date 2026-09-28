# Performance-Adaptive Resolver

## Why

The adaptive difficulty resolver shipped in the `adaptive-difficulty` change is commit-shape-driven: it inspects the diff/commit signals and picks easy/intermediate/hard per question. Its strongest rule — the AI-delegation signal (short commit message + large diff) — fires *first* and wins outright, but that signal is exactly what the product's primary user class (agent-heavy developers, who delegate most commit work) produces on every commit. For our primary users the resolver degenerates into "always hard": constant, not adaptive, and fatiguing (the risk the prior design doc already flagged). Meanwhile, the data that actually reflects the learner — per-question outcomes recorded as `selections` joined to `choices.is_correct`, timestamped by `question_events` — is recorded but read by nothing.

This change makes the learner the primary factor, per the user's stated vision:

- **Content-level repetition**: procedurally adapt recall question *contents* — concepts the user shows weakness in (missed questions on a topic) are re-asked, via what the synthesis feed emphasizes.
- **Difficulty escalation**: adapt the difficulty level based on the user's success rate — e.g. a recent correctness ratio ≥ 92% earns more advanced/deeper questions; a struggling learner is eased rather than hammered.
- **Signal class demotion**: commit-shape heuristics (AI-delegation keys among them) become a *second-class* baseline — they bootstrap resolution when no learner history exists and otherwise only modulate; they never outrank learner evidence, and the AI-delegation signal in particular loses its first-match winner status.

## What Changes

**New resolver `Performance` (wraps the heuristic baseline):** a `LearnerPerformance` resolver sits above the commit-signal `Adaptive` baseline. It reads aggregate answer outcomes (correct vs incorrect per question, attributed to concepts via provenance links) and produces the resolution:

- **Difficulty tier:** windowed correctness ratio over the user's recent answered questions. Ratio ≥ 0.92 → escalate one level above the baseline; ratio below a struggling floor (e.g. < 0.50) → de-escalate one level; in between → the commit-signal baseline unchanged. Insufficient history (fewer than N answered) → pure commit-signal baseline (today's behavior preserved as the cold start).
- **Weakness targeting:** concepts with indications of weakness (recent misses, repeated misses on the same concept) drive part of the resolution output — the resolver returns a set of focus concepts that the engine feeds preferentially to synthesis, so the next question *targets* what the user is demonstrably weak on.
- **Provenance prerequisite:** per-concept outcomes require a question→concept link and concept identity. This change *consumes* the `question_concepts` join and the `concept_registry` identity layer from the `concept-provenance` change — it does not invent its own schema. Sequenced after that change; see design for the fallback if it has not landed.
- **Audit continuity:** the resolved difficulty now persists on the `'queued'` question event payload (the deferred follow-up captured as task 6.3 of `adaptive-difficulty`) so operator debugging and future resolvers can correlate level↔outcome.

**Demoting the commit-shape signals:**

- The first-match heuristic table is re-ordered: learner history > concept-structure signals (high-cluster, dispersed) > all-code escalation > AI-delegation. AI-delegation escalates to `hard` only as the *final* commit-shape rule, and never applies when learner history says otherwise.
- The AI-delegation heuristic alone no longer resolves to `hard` before the concept-structure rules get their turn.

**Resolution contract widens (breaking, internal):** the `Resolver` interface evolves from returning a bare difficulty `string` to a typed `Resolution{Difficulty string, FocusConcepts []string}`. `Static` and `Synthesize` adopt the new shape; every existing configured value (`easy`/`intermediate`/`hard`/`adaptive`) continues to resolve as it does today.

## Capabilities

### New Capabilities
- `learner-performance`: the `Performance` resolution — mastery aggregation over `question_events` + `question_concepts` provenance (per-concept results), the correctness-ratio tier rules (windows, thresholds 0.92/floor, escalation ladder), weakness-driven focus-concept selection for the synthesis feed, and the cold-start delegation to the commit-signal baseline.

### Modified Capabilities
- `difficulty-resolution`: the `Resolver` contract widens to return `Resolution` (difficulty + focus concepts); the heuristic first-match table is reordered so AI-delegation is the last commit-shape rule rather than the first-match winner.
- `recall-engine`: `Synthesize` consumes the `Resolution` — the difficulty lands in the system turn as today, and the focus concepts bias the synthesis user-turn feed (weak concepts surfacing first) so question *contents* target demonstrated weaknesses.

## Impact

- **Code:** `internal/recall/difficulty` (new `performance.go` resolver + mastery/stats types; `.Resolve` return type widens; `adaptive.go` rule reordering + demotion of the AI-delegation rule). `internal/recall/engine.go` (`Synthesize` consumes `Resolution`; user-turn feed biasing hook). `internal/engine/server.go` (passes learner stats store handle or resolver context through — no engine surface change). `internal/cache/store.go` (read-only mastery aggregation queries joining `questions`/`question_events`/`question_concepts`; `'queued'` event payload now carries the resolved difficulty). Tests collocate: `internal/recall/difficulty/performance_test.go` plus engine/server integration tests in `cmd/torec/`.
- **Dependencies:** consumes `concept-provenance` (the `question_concepts` join + `concept_registry` slugs). If that change has not landed when this one implements, a scoped provenance-along-the-way fallback applies (record the synthesis concept batch on the queued event payload instead — see design, Open Questions).
- **APIs:** no external API change; `Resolver`'s method signature is an internal seam — sole in-tree callers (`Static`, engine, tests) update atomically.
- **Specs:** adds `openspec/specs/learner-performance/spec.md`; updates `openspec/specs/difficulty-resolution/spec.md` and `openspec/specs/recall-engine/spec.md`.
