## MODIFIED Requirements

### Requirement: Recall engine accepts an enriched `SynthesisContext` and synthesizes a single question per hook event
`Engine.Synthesize(ctx context.Context, repo, branch, model string, synth SynthesisContext, resolver Resolver) (*Question, error)` SHALL produce at most one `Question` per invocation. The `difficulty string` parameter present in the prior Change A signature is removed; difficulty selection is delegated to the `Resolver` interface (`internal/recall/difficulty`). `Synthesize` calls `resolver.Resolve(ctx, synth)` to obtain the difficulty string before constructing the system prompt; if the resolver returns `""`, `Synthesize` substitutes `"intermediate"` as the safety-net default (preserving the prior `defaultDifficulty = "intermediate"` behavior without elevating the constant to a public API). If `repo == ""` or `branch == ""`, `Synthesize` returns `nil, nil` without calling `resolver.Resolve` (no resolution cost for empty contexts). If `len(synth.Concepts) == 0`, `Synthesize` returns `nil, nil` without calling the provider.

#### Scenario: Enriched concepts available in SynthesisContext
- **WHEN** `Synthesize` is called with `repo = "/path/X"`, `branch = "feature-X"`, `synth.Concepts` containing 3 `ConceptRow` entries, and a `Static` resolver configured to return `"hard"`
- **THEN** it calls the provider with a system turn containing `"hard"` as the difficulty directive and a user turn listing each concept with weight/source/seen-at, and returns a `*Question`

#### Scenario: Empty SynthesisContext concepts refuses to synthesize
- **WHEN** `Synthesize` is called with `len(synth.Concepts) == 0` (e.g., first-ever commit on the branch produced no cached concepts)
- **THEN** `Synthesize` returns `nil, nil` without invoking `resolver.Resolve` and without making an AI call

#### Scenario: Empty repo or branch refuses to synthesize
- **WHEN** `Synthesize` is called with `repo = ""` or `branch = ""` (e.g., detached HEAD scenario upstream)
- **THEN** `Synthesize` returns `nil, nil` without invoking `resolver.Resolve` and without calling the provider; the pipeline skips silently

#### Scenario: Resolver produces the difficulty injected into the prompt
- **WHEN** the configured `Resolver`'s `Resolve` returns a concrete level for the given `SynthesisContext`
- **THEN** the system turn passed to `SynthesisRequest` contains that level as the difficulty directive

#### Scenario: Empty resolver result falls back to intermediate
- **WHEN** the configured resolver's `Resolve` returns `""` for a given `SynthesisContext`
- **THEN** `Synthesize` substitutes `"intermediate"` and proceeds with the system prompt containing `"intermediate"` as the difficulty directive

---

## ADDED Requirements

### Requirement: `Engine.New` constructs the `Resolver` from the configured `RecallConfig.Difficulty`
`recall.New(provider, store, cfg)` SHALL construct the `Resolver` field on the `Engine` struct once per process. When `cfg.Recall.Difficulty != ""`, the resolver is `Static{value: cfg.Recall.Difficulty}` (which delegates to `Adaptive` when `value == "adaptive"`). When `cfg.Recall.Difficulty == ""`, the resolver is `Static{value: "adaptive"}` (preserves the prior `defaultDifficulty` semantics; the `Adaptive` resolver produces the fallback `"intermediate"` when no signals fire).

#### Scenario: Adaptive config constructs an adaptive-routed Static resolver
- **WHEN** `recall.New` is called with `cfg.Recall.Difficulty == "adaptive"`
- **THEN** the `Engine` has a `Static{value: "adaptive"}` resolver; subsequent `Synthesize` calls invoke `Static.Resolve` → `Adaptive.Resolve` → concrete difficulty

#### Scenario: Hard config constructs a verbatim Static resolver
- **WHEN** `recall.New` is called with `cfg.Recall.Difficulty == "hard"`
- **THEN** the `Engine` has a `Static{value: "hard"}` resolver; subsequent `Synthesize` calls invoke `Static.Resolve` → `"hard"` (no `Adaptive` consultation)

#### Scenario: Empty config constructs an adaptive-routed Static resolver
- **WHEN** `recall.New` is called with `cfg.Recall.Difficulty == ""`
- **THEN** the `Engine` has a `Static{value: "adaptive"}` resolver; the `Adaptive` resolver produces the fallback `"intermediate"` for empty contexts (preserving the prior `defaultDifficulty = "intermediate"` behavior via the resolver chain rather than via a `Synthesize`-level constant)

---

### Requirement: `defaultDifficulty` constant is removed; the `Resolver` owns the default
`internal/recall/engine.go` SHALL NOT export or carry a `defaultDifficulty` constant. The default difficulty selection is fully owned by the `Resolver` chain: `Static` returns the configured value or delegates to `Adaptive`; `Adaptive` falls back to `"intermediate"` when no heuristic matches. The `Synthesize` method retains a single `if difficulty == "" { difficulty = "intermediate" }` defensive substitution as a zero-value safety net for future resolver implementations that return `""`; this is not a public default, just a defensive guard.

#### Scenario: No defaultDifficulty constant in the engine package
- **WHEN** a developer greps `internal/recall/engine.go` for `defaultDifficulty`
- **THEN** the identifier is absent (the constant was deleted in this change)

#### Scenario: Safety-net fallback activates only on empty resolver result
- **WHEN** the resolver returns a non-empty difficulty string (`"easy"`, `"intermediate"`, `"hard"`, or any other non-empty value)
- **THEN** `Synthesize` uses that string verbatim; the `"intermediate"` substitution does not fire

#### Scenario: Safety-net fallback activates on empty resolver result
- **WHEN** the resolver returns `""`
- **THEN** `Synthesize` substitutes `"intermediate"` and the prompt contains `"intermediate"` as the difficulty directive