## MODIFIED Requirements

### Requirement: Recall engine synthesizes a single question per hook event using a `Resolver` for difficulty selection
`Engine.Synthesize(ctx context.Context, repo, branch, model string, synth SynthesisContext, resolver Resolver) (*Question, error)` SHALL produce at most one `Question` per invocation. The `difficulty string` parameter present in the prior Change A signature is removed; difficulty selection is delegated to the `Resolver` interface (`internal/recall/difficulty`). `Synthesize` calls `resolver.Resolve(ctx, synth)` to obtain the difficulty string before constructing the system prompt; if the resolver returns `""`, `Synthesize` substitutes `"intermediate"` as the safety-net default (preserving the prior `defaultDifficulty = "intermediate"` behavior without elevating the constant to a public API). If `repo == ""` or `branch == ""`, `Synthesize` returns `nil, nil` without calling `resolver.Resolve` (no resolution cost for empty contexts). If `len(synth.Concepts) == 0`, `Synthesize` returns `nil, nil` without calling the provider.

#### Scenario: Resolver produces the difficulty injected into the prompt
- **WHEN** `Synthesize` is called with a `Static` resolver configured to return `"hard"` and a `SynthesisContext` containing 3 concept rows for `repo = "/path/X"` and `branch = "feature-X"`
- **THEN** the system turn passed to `SynthesisRequest` contains `"hard"` as the difficulty directive

#### Scenario: Empty resolver result falls back to intermediate
- **WHEN** the configured resolver's `Resolve` returns `""` for a given `SynthesisContext`
- **THEN** `Synthesize` substitutes `"intermediate"` and proceeds with the system prompt containing `"intermediate"` as the difficulty directive

#### Scenario: Resolver not consulted for empty repo or branch
- **WHEN** `Synthesize` is called with `repo = ""` or `branch = ""`
- **THEN** `Synthesize` returns `nil, nil` without invoking `resolver.Resolve`; the resolver is not on the critical path for the empty-context short-circuit

#### Scenario: Resolver not consulted for empty concept list
- **WHEN** `Synthesize` is called with `len(synth.Concepts) == 0`
- **THEN** `Synthesize` returns `nil, nil` without invoking `resolver.Resolve`; the resolver is not on the critical path for the empty-concepts short-circuit

---

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