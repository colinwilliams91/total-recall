# recall-engine

## MODIFIED Requirements

### Requirement: Recall engine accepts an enriched `SynthesisContext` and synthesizes a single question per hook event
`Engine.Synthesize(ctx context.Context, repo, branch, model string, synth SynthesisContext, resolver Resolver) (*Question, error)` SHALL produce at most one `Question` per invocation. Difficulty selection is delegated to the `Resolver` interface (`internal/recall/difficulty`). `Synthesize` calls `resolver.Resolve(ctx, synth)` and consumes the returned `Resolution`: the `Difficulty` string drives the system turn (an empty difficulty substitutes `"intermediate"` as the safety-net default, as before), and the `FocusConcepts` list biases the synthesis user turn — focus concepts are surfaced ahead of the remaining concept rows so the generated question targets demonstrated weaknesses. Focus concepts that are not already present in `synth.Concepts` MUST NOT be fabricated into the feed (biasing only reorders/promotes concepts that exist in the context). If `repo == ""` or `branch == ""`, `Synthesize` returns `nil, nil` without calling `resolver.Resolve` (no resolution cost for empty contexts). If `len(synth.Concepts) == 0`, `Synthesize` returns `nil, nil` without calling the provider.

#### Scenario: Enriched concepts available in SynthesisContext
- **WHEN** `Synthesize` is called with `synth.Concepts` containing concept rows for `repo = "/path/X"` and `branch = "feature-X"` and a `Static` resolver resolving to `"hard"`
- **THEN** it calls the provider with a system turn composed from the loaded policy doc + format contract carrying the resolved difficulty directive, and a user turn listing each concept with weight/source/seen-at

#### Scenario: Empty SynthesisContext concepts refuses to synthesize
- **WHEN** `Synthesize` is called with `len(synth.Concepts) == 0` (e.g., first-ever commit on the branch produced no cached concepts)
- **THEN** `Synthesize` returns `nil, nil` without invoking `resolver.Resolve` and without making an AI call

#### Scenario: Empty repo or branch refuses to synthesize
- **WHEN** `Synthesize` is called with `repo = ""` or `branch = ""` (e.g., detached HEAD scenario upstream)
- **THEN** `Synthesize` returns `nil, nil` without invoking `resolver.Resolve` and without calling the provider; the pipeline skips silently

#### Scenario: Resolver produces the difficulty injected into the prompt
- **WHEN** `Synthesize` is called with a `Static` resolver resolving to `"hard"` and a `SynthesisContext` containing 3 concept rows
- **THEN** the system turn passed to `SynthesisRequest` contains `"hard"` as the difficulty directive

#### Scenario: Empty resolver result falls back to intermediate
- **WHEN** the configured resolver's `Resolve` returns an empty `Difficulty`
- **THEN** `Synthesize` substitutes `"intermediate"` and proceeds with the system prompt containing `"intermediate"` as the difficulty directive

#### Scenario: Focus concepts bias the user turn ordering
- **WHEN** the resolver's `Resolution` carries `FocusConcepts = ["circuit-breaker"]` and `synth.Concepts` contains `["retry", "circuit-breaker", "jitter"]`
- **THEN** the user turn lists `circuit-breaker` ahead of `retry` and `jitter` (biasing is reordering, not synthesis-input invention)

#### Scenario: Focus unknown to the current context is ignored
- **WHEN** `FocusConcepts` names a concept absent from `synth.Concepts`
- **THEN** the user turn is unchanged with respect to that concept (no placeholder rows injected)

#### Scenario: Resolver not consulted for empty repo or branch
- **WHEN** `Synthesize` is called with `repo = ""` or `branch = ""`
- **THEN** `Synthesize` returns `nil, nil` without invoking `resolver.Resolve`; the resolver is not on the critical path for the empty-context short-circuit

#### Scenario: Resolver not consulted for empty concept list
- **WHEN** `Synthesize` is called with `len(synth.Concepts) == 0`
- **THEN** `Synthesize` returns `nil, nil` without invoking `resolver.Resolve`; the resolver is not on the critical path for the empty-concepts short-circuit
