# recall-engine

## MODIFIED Requirements

### Requirement: `Engine.New` constructs the `Resolver` from the configured `RecallConfig.Difficulty`
`recall.New(provider, store, cfg)` SHALL construct the `Resolver` field on the `Engine` struct once per process. When `cfg.Recall.Difficulty` is `"adaptive"` (or empty, preserving the prior default semantics), the resolver is a `Performance` resolver chaining over `Static{value: "adaptive"}` — the commit-signal baseline resolves first, and learner evidence (windowed answered history read through a stats provider backed by the store) modulates the tier and ranks the focus set. For a concrete configured value the chain remains `Static{value}` resolving verbatim, with no learner override of explicitly pinched difficulty. A nil store yields a Performance resolver whose stats reads behave as cold start (baseline only) — construction never fails for lack of a stats source.

#### Scenario: Adaptive config constructs the Performance chain over the Static baseline
- **WHEN** `recall.New` is called with `cfg.Recall.Difficulty == "adaptive"`
- **THEN** the `Engine` resolver is `Performance` chaining over `Static{value: "adaptive"}`; subsequent `Synthesize` calls resolve through the adaptive heuristics first, then the learner tier and focus ranking over windowed answered history

#### Scenario: Hard config constructs a verbatim Static resolver
- **WHEN** `recall.New` is called with `cfg.Recall.Difficulty == "hard"`
- **THEN** the `Engine` has a `Static{value: "hard"}` resolver; subsequent `Synthesize` calls invoke `Static.Resolve` → `"hard"` (no `Adaptive` consultation, no learner override)

#### Scenario: Empty config constructs the Performance chain over the Static baseline
- **WHEN** `recall.New` is called with `cfg.Recall.Difficulty == ""`
- **THEN** the `Engine` chains `Performance` over `Static{value: "adaptive"}` (preserving the prior `defaultDifficulty = "intermediate"` fallback semantics via the resolver chain)

#### Scenario: Nil store still constructs a working chain
- **WHEN** `recall.New` is called with a nil store and `difficulty: "adaptive"`
- **THEN** the `Performance` resolver treats every stats read as cold start and resolves from the commit-signal baseline only

---

### Requirement: Recall engine accepts an enriched `SynthesisContext` and synthesizes a single question per hook event
`Engine.Synthesize(ctx context.Context, repo, branch, model string, synth SynthesisContext, resolver Resolver) (*Question, Resolution, error)` SHALL produce at most one `Question` per invocation and return the resolver's `Resolution` for the pipeline's queued-event audit payload. The `difficulty string` parameter present in the prior Change A signature is removed; difficulty selection is delegated to the `Resolver` interface (`internal/recall/difficulty`). `Synthesize` calls `resolver.Resolve(ctx, synth)` and consumes the returned `Resolution`: the `Difficulty` string drives the system turn (an empty difficulty substitutes `"intermediate"` as the safety-net default, as before), and the `FocusConcepts` list biases the synthesis user turn — matched focus concepts surface first under their own lead section ("Concepts the developer is currently working through") ahead of the remaining concept rows so the generated question targets demonstrated weaknesses. Focus concepts that are not already present in `synth.Concepts` MUST NOT be fabricated into the feed (biasing only reorders/promotes concepts that exist in the context); a focus set matching no rows produces no lead section. `SynthesisContext` SHALL carry the `Repo` and `Branch` of the synthesis so scope-aware resolvers can read per-surface learner stats. If `repo == ""` or `branch == ""`, `Synthesize` returns `(nil, zero Resolution, nil)` without calling `resolver.Resolve` (no resolution cost for empty contexts). If `len(synth.Concepts) == 0`, `Synthesize` returns `(nil, zero Resolution, nil)` without calling the provider. Synthesis AI-call failures return `(nil, zero Resolution, nil)` as before.

#### Scenario: Enriched concepts available in SynthesisContext
- **WHEN** `Synthesize` is called with `synth.Concepts` containing concept rows for `repo = "/path/X"` and `branch = "feature-X"` and a `Static` resolver resolving to `"hard"`
- **THEN** it calls the provider with a system turn composed from the loaded policy doc + format contract carrying the resolved difficulty directive, and a user turn listing each concept with weight/source/seen-at

#### Scenario: Empty SynthesisContext concepts refuses to synthesize
- **WHEN** `Synthesize` is called with `len(synth.Concepts) == 0` (e.g., first-ever commit on the branch produced no cached concepts)
- **THEN** `Synthesize` returns `(nil, zero Resolution, nil)` without invoking `resolver.Resolve` and without making an AI call

#### Scenario: Empty repo or branch refuses to synthesize
- **WHEN** `Synthesize` is called with `repo = ""` or `branch = ""` (e.g., detached HEAD scenario upstream)
- **THEN** `Synthesize` returns `(nil, zero Resolution, nil)` without invoking `resolver.Resolve` and without calling the provider; the pipeline skips silently

#### Scenario: Resolver produces the difficulty injected into the prompt
- **WHEN** `Synthesize` is called with a `Static` resolver resolving to `"hard"` and a `SynthesisContext` containing 3 concept rows
- **THEN** the system turn passed to `SynthesisRequest` contains `"hard"` as the difficulty directive

#### Scenario: Empty resolver result falls back to intermediate
- **WHEN** the configured resolver's `Resolve` returns an empty `Difficulty`
- **THEN** `Synthesize` substitutes `"intermediate"` and proceeds with the system prompt containing `"intermediate"` as the difficulty directive

#### Scenario: Focus concepts bias the user turn ordering
- **WHEN** the resolver's `Resolution` carries `FocusConcepts = ["circuit-breaker"]` and `synth.Concepts` contains `["retry", "circuit-breaker", "jitter"]`
- **THEN** the user turn leads with a "Concepts the developer is currently working through" section listing `circuit-breaker`, and the working-with list contains only `retry` and `jitter` — the focused row moves to the lead section rather than being duplicated (biasing is reordering, not synthesis-input invention)

#### Scenario: Focus unknown to the current context is ignored
- **WHEN** `FocusConcepts` names a concept absent from `synth.Concepts`
- **THEN** the user turn is unchanged with respect to that concept (no placeholder rows injected); a focus set matching no rows produces no lead section at all

#### Scenario: Resolver not consulted for empty repo or branch
- **WHEN** `Synthesize` is called with `repo = ""` or `branch = ""`
- **THEN** `Synthesize` returns `(nil, zero Resolution, nil)` without invoking `resolver.Resolve`; the resolver is not on the critical path for the empty-context short-circuit

#### Scenario: Resolver not consulted for empty concept list
- **WHEN** `Synthesize` is called with `len(synth.Concepts) == 0`
- **THEN** `Synthesize` returns `(nil, zero Resolution, nil)` without invoking `resolver.Resolve`; the resolver is not on the critical path for the empty-concepts short-circuit
