## Context

The `adaptive-difficulty` change shipped a commit-shape-driven difficulty resolver: per synthesis call it evaluates a first-match heuristic table (AI-delegation, high-cluster, dispersed, all-code, fallback) over the `SynthesisContext` and injects the concrete level into the system prompt. The design doc of that change explicitly deferred the learner-facing goals — a `Performance` resolver over answer history and spaced-repetition — and flagged the "adaptive always escalates" bias. This change is that deferral landing.

The user's framing (which this change adopts as its governing requirement):

1. Primary user class is agent-heavy developers. Any commit-shape signal that detects "this diff looks AI-generated" is *second-class*: it describes the workflow, not the learner, and for the primary user class it fires on every commit — degenerating resolution into "always hard".
2. The two real adaptation axes are content (repeat concepts the user shows weakness in) and difficulty (success-rate escalation, e.g. ≥ 92% recent correctness earns harder questions).

Data foundations observed in the tree:

- `question_events` records `queued`/`delivered`/`answered`/`skipped` rows with `occurred_at`; `selections` joins to `choices.is_correct` — per-question correctness is already derivable today (no new writes needed for the tier rule).
- Per-*concept* outcomes are NOT derivable yet: `questions` carry no concept linkage. The in-flight `concept-provenance` change (planned, unimplemented) introduces `question_concepts(question_id, concept_slug, weight)` written at synthesis time plus the `concept_registry` identity layer. That join is the prerequisite for per-concept weakness.

## Goals / Non-Goals

**Goals:**
- A `Performance` resolver that composes learner evidence over the commit-signal baseline: tier the difficulty from the windowed correctness ratio; return focus concepts for weakness-targeted content.
- Demote commit-shape signals: reorder the `Adaptive` table so AI-delegation is the last rule; learner evidence (where present) is the only factor that can move past commit shape.
- Widening the `Resolver` contract minimally: `Resolve` returns `Resolution{Difficulty, FocusConcepts}` — one struct, not a second method.
- Persist the resolved difficulty at queue time (`'queued'` event payload), completing the prior change's deferred task 6.3 — the audit spine the next resolver iteration will need.

**Non-Goals:**
- Spaced-repetition *scheduling* (when to re-ask) — this change implements *what* to emphasize; the resurfacing scheduler (a `SpacedRepetition` resolver with intervals/intervals-of-forgetting) remains future work.
- New lifecycle event types — no CHECK-vocabulary changes needed; the difficulty rides the existing `'queued'` row's payload column.
- Separating the resolver from `Synthesize`'s call path or introducing async resolution.
- A second AI call to grade concepts or judge learner ability — alllearner stats are SQL aggregates over data already recorded.
- Changing the three-level difficulty vocabulary or the config keys (`recall.difficulty` values remain `easy`|`intermediate`|`hard`|`adaptive`).

## Decisions

### Decision: `Performance` composes with (not replaces) the commit-signal baseline

The new resolver wraps the existing `Adaptive` heuristics rather than replacing them: with insufficient learner history, it *is* the baseline (cold start = today's behavior, minus the AI-delegation promotion); with sufficient history, the learner tier shifts the baseline's result. This keeps the commit-shape signals exactly where the user placed them — second-class — while preserving them as the bootstrapping prior.

**Alternatives considered:**
- *Replace the heuristic table entirely once history exists.* Rejected: the commit-signal layer carries per-commit context (the shape of this diff) that learner aggregates deliberately abstract away; a hybrid keeps both signals honest — commits shape the baseline, the learner moves it.
- *Store fused `Performance` logic inside `Adaptive`.* Rejected: two different evidence domains (commit shape vs recorded outcomes), different data dependencies (the store handle), and different test surfaces — the seam between them is the design.

### Decision: one wide `Resolve` result, not a new interface method

`Resolver.Resolve` returns `Resolution{Difficulty string, FocusConcepts []string}` instead of a bare string. One method, one widening; `Static` gains an empty-focus default, `Adaptive` wraps its level into the struct. Callers in `Synthesize` destructure once.

**Alternatives considered:**
- *A second interface method `FocusConcepts(ctx, synth)`.* Rejected: two methods invites per-call contract drift (a resolver implementing one but not the other) and two resolution costs where callers always need both.
- *Side-channel provider for focus concepts.* Rejected: focus selection is a resolution responsibility (learner-driven); a separate injection path would duplicate the staleness/ordering rules.

### Decision: correctness tiers are exactly one level around the commit-signal baseline

The windowed ratio (default window: last 20 answered per repo+branch) applies at most one tier move: ≥ 0.92 escalates, < 0.50 de-escalates, else baseline holds. The 0.92 threshold is the user-specified example, adopted verbatim as the initial constant; the floor is the proposing-side trade (struggling learners get eased rather than hammered).

**Alternatives considered:**
- *Absolute tiers ignoring commit shape when history exists.* Rejected for v1: it discards live per-commit context entirely; the composition keeps both evidence types and is easier to reason about. A future change can revisit "learner-only mode".
- *Per-concept difficulty tiers.* Rejected for this change: per-concept outcome granularity is too sparse for a stable tier (a concept seen twice swings wildly); it informs the *focus set*, not the tier, until data volume justifies it.

### Decision: weakness targeting reorders the synthesis feed, it does not create a separate question pipeline

The `Resolution.FocusConcepts` list reorders/promotes concept rows in the synthesis user turn (the policy prompt already says "keep the question directly related to one of the provided concepts" — feeding the weak concept first exploits that). Focus concepts ranking: recent misses first (miss count desc, then most-recent first), each concept must be linked to at least one answered question in the window, zero-outcome concepts never qualify, all-recently-correct yields an empty focus set. Cap: 3 focus concepts. Cross-repo weak concepts may only fill leftover focus slots (the working surface stays first-class — same priority constraint `concept-provenance` records).

**Alternatives considered:**
- *A dedicated "review queue" of synthesized weak-concept questions.* Rejected: doubles the synthesis path and the delivery contract surfaces questions whose repo/branch differs from the user's current work — the provenance change's selection-priority rule explicitly forbids promoting out-of-surface concepts ahead of the working surface.
- *Prompt surgery ("focus on concept X").* Rejected: the difficulty/version of the user turn stays observable and testable through reordering; bespoke directive sentences would escape the policy-asset contract that `SynthesisRequest` owns.

### Decision: focus concepts re-rank (never invent) synthesis inputs

`Synthesize` reorders `synth.Concepts` so carried focus concepts surface first; a focus concept absent from the current context is dropped silently. The synthesis feed remains 100% derivable from the user's own data.

**Alternatives considered:**
- *Fetch the weak concept's ConceptRow from the store and splice it in.* Rejected for this change: it changes the concept feed's meaning (a sighting the user is not currently working on) and needs a store round-trip per focus target. The provenance change's priority ordering already settles surface-first feeding; child work can revisit once that lands.

### Decision: per-concept outcomes come from the provenance join (`concept-provenance`), with a recorded fallback

The performance stats query joins `'answered'` `question_events` → selections/choices → `question_concepts` (slug-grained, from the provenance change). Sequencing: `adaptive-difficulty` archives first; `concept-provenance` lands before or with this change. If neither is available at implementation time, the minimal fallback is: record the synthesis concept batch on the `'queued'` event payload (the deferred task 6.3 style: `{"resolved_difficulty": ..., "concepts": [...]}`) and aggregate weakness from that payload — strictly worse linkage (batch-level, not per-question-target), so it degrades weakness ranking, not the difficulty tier rule (which needs only per-question correctness, available today).

**Alternatives considered:**
- *Invent a `question_concepts`-equivalent table here.* Rejected: duplicates the provenance change's design; two identity models would fight.
- *Infer weakness from question text similarity.* Rejected: fuzzy matching is exactly what `concept-provenance` exists to avoid.

### Decision: mastery stats are read-only store queries, cached per pipeline call

The `Performance` resolver needs a stats provider; the composition root (engine construction) wires a read-only stats interface backed by the store (which already owns `question_events`). One aggregate query per synthesis call — the same order of cost as the existing `Recent()` fetch, no caching layer, no hot-reload machinery.

**Alternatives considered:**
- *Pass the whole `*cache.Store` into the resolver.* Rejected: the resolver would gain write access it must never need; the narrow stats interface is the seam future resolvers (spaced repetition) extend.
- *Persist an aggregate mastery table.* Rejected: denormalizes outcomes; the event spine is queryable as-is and the pull-on-demand model avoids invalidation bugs.

## Risks / Trade-offs

- **[Trade-off] Answer scarcity makes the tier rule conservative early.** The 20-question window means the tier rules engage only after a real answer habit forms; until then resolution is commit-shape-driven (cold start preserved). Accepted: thresholds before the window fills would chase noise.
- **[Risk] Skipped questions skew the focus ranking.** A concept the user skips repeatedly accumulates no outcome; it will not rank as weak. Accepted for this change (skip ≠ known weakness); a future change can weight a forming skip-streak.
- **[Dependency risk] `concept-provenance` defines the concept linkage.** If it changes shape (e.g. `question_concepts` gains columns), the mastery query follows. Recorded as an explicit ordering dependency; the batch-payload fallback protects the tier rule from total blockage.
- **[Risk] Focus re-ranking is subtle in the prompt.** The synthesis model may not honor ordering emphasis for a well-weighted context. Mitigation: the user-turn formatting gives focused concepts their own lead section ("Concepts the developer is currently working through") — observable in the synthesis capture tests.
- **[Trade-off] Window/threshold constants are best-guesses** (20 / 0.92 / 0.50 / 5-history / 3-focus). Package-private in this change; the A/B spike harness from `adaptive-difficulty` (task 5.1) generalizes to capture per-tier outcome distributions before exposing any knob as config.

## Migration Plan

1. Archive `adaptive-difficulty` (its `difficulty-resolution` and `recall-engine` deltas become the main specs this change modifies).
2. Land `concept-provenance` (or accept the recorded fallback path in Open Questions).
3. Implement `Resolution` contract widening + `Adaptive` reorder in `internal/recall/difficulty` (mechanical, all in-tree callers update atomically).
4. Implement mastery aggregation (store read query) and the `Performance` resolver.
5. Wire composition at the engine (`Engine.New` wraps the configured chain with `Performance` on top), bias the synthesis user turn, persist resolved difficulty on the queued event payload.
6. Tests: resolver unit tests (tiers, cold start, focus ranking), engine integration (focus bias observable in the user turn), config sanity (static difficulty modes unchanged).
7. No new schema of its own; existing tables + `question_concepts` only. No migration beyond whatever `concept-provenance` defines.

## Open Questions

- **Sequencing:** this change assumes `concept-provenance` lands first (its `question_concepts` + `concept_registry` provide the concept-level outcomes). If the user wants this change first, the fallback is recording the synthesis concept batch on the `'queued'` event payload and computing *batch-level* weakness until provenance arrives — workable, weaker, explicitly temporary.
- **Window/threshold tuning:** 20 answers / 0.92 ceiling / 0.50 floor / 5-answer minimum / 3-focus cap are constants, not config (mirroring the prior change's stance). A/B spike data should drive the first tuning pass.
- **Does the tier rule read repo+branch scope or user-global?** Records show concepts are per-repo/per-branch; the learner is global. The change tiers on the per-repo/per-branch window (the store's scoping rule), but a user-global variant is a plausible follow-up — user experience input welcome.
- **Should `skipped` questions count toward a future "how often are they disengaging" signal?** Not in this change (data lives under `'skipped'` events); noted for the spaced-repetition iteration.
