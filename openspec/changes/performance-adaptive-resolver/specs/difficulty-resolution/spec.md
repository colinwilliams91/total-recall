# difficulty-resolution

## MODIFIED Requirements

### Requirement: `Resolver` interface exposes difficulty selection as a separable concern from synthesis prompt construction
The `internal/recall/difficulty` package exposes a `Resolver` interface with a single method `Resolve(ctx context.Context, synth recall.SynthesisContext) Resolution`. The returned `Resolution` SHALL carry `Difficulty` — one of `"easy"`, `"intermediate"`, `"hard"`, or `""` (the empty string indicating the resolver elected no preference and the caller should substitute a default) — and `FocusConcepts`, an optional list of concept identifiers biased for synthesis content targeting (weak-concept repetition; empty when the resolver has no content steer). `Synthesize` and `SynthesisRequest` consume only the `Resolution` struct; they do not inspect the heuristics or signals that produced it.

#### Scenario: Resolver returns a concrete difficulty
- **WHEN** any `Resolver` implementation's `Resolve` is called with a valid `SynthesisContext`
- **THEN** the returned `Resolution.Difficulty` is `"easy"`, `"intermediate"`, `"hard"`, or `""` — no other value is permitted

#### Scenario: Resolver is stateless across calls
- **WHEN** the same `Resolver` instance is called twice in a row with the same `SynthesisContext`
- **THEN** both calls return the same `Resolution` values; the resolver does not mutate per-call state

### Requirement: `Adaptive` resolver applies a first-match heuristic table over `SynthesisContext` signals
`Adaptive{}` SHALL implement `Resolver`. `Resolve(ctx, synth)` MUST evaluate the following heuristics in order; the first match wins and produces the returned difficulty:

1. **High-cluster signal** — `avg(synth.Concepts[*].Weight) > 0.7 AND len(synth.Concepts) >= 3` → returns `"hard"`
2. **Dispersed signal** — `avg(synth.Concepts[*].Weight) < 0.3 AND len(synth.Concepts) >= 5` → returns `"easy"`
3. **All-code source signal** — `len(synth.Concepts) >= 5 AND all(synth.Concepts[*].Source == "code")` → escalates the fallback by one level; returns `"hard"` (the fallback `"intermediate"` escalated one level)
4. **AI-delegation signal** — `len(synth.CommitMsg) < 50 AND len(synth.DiffSnippet) > 400` → returns `"hard"` (last commit-shape rule; agent-heavy workflows — the product's primary user class — trip this constantly, so it terminates rather than leads the table)
5. **Fallback** — no heuristic matched → returns `"intermediate"`

The thresholds are package-private constants in `internal/recall/difficulty/adaptive.go`: `adaptiveHighClusterWeight = 0.7`, `adaptiveDispersedWeight = 0.3`, `adaptiveMinHighClusterConcepts = 3`, `adaptiveMinDispersedConcepts = 5`, `adaptiveDelegationMsgMaxChars = 50`, `adaptiveDelegationSnippetMinChars = 400`, `adaptiveMinAllCodeSourceConcepts = 5`.

#### Scenario: AI-delegation no longer outranks the concept-structure signals
- **WHEN** `synth` carries both the AI-delegation shape (short message + large snippet) and 5 concepts at avg weight 0.2
- **THEN** the resolved difficulty is `"easy"` (the dispersed rule is evaluated first; delegation is the final commit-shape rule)

#### Scenario: High-cluster signal fires (no AI-delegation)
- **WHEN** `synth.CommitMsg` is 200 chars (no AI-delegation signal), `synth.Concepts` has 3 entries with weights `[0.9, 0.8, 0.7]` (avg = 0.8)
- **THEN** `Adaptive{}.Resolve` returns `"hard"`

#### Scenario: Dispersed signal fires
- **WHEN** `synth.Concepts` has 5 entries with weights all `0.2` and no AI-delegation signal
- **THEN** `Adaptive{}.Resolve` returns `"easy"`

#### Scenario: All-code source signal escalates fallback
- **WHEN** `synth.Concepts` has 5 entries all with `Source = "code"`, avg weight `0.5` (no high-cluster, no dispersed, no delegation)
- **THEN** `Adaptive{}.Resolve` returns `"hard"` (fallback `"intermediate"` escalated one level)

#### Scenario: AI-delegation still escalates when nothing else fires
- **WHEN** `synth.CommitMsg = "fix:"` (5 chars) and `synth.DiffSnippet` is 500 chars, with no concept-structure signal
- **THEN** `Adaptive{}.Resolve` returns `"hard"` — the rule is demoted, not removed

#### Scenario: Fallback with no signal match
- **WHEN** `synth.Concepts` has 3 entries with avg weight `0.5` (no high-cluster, too few for dispersed, mixed sources), and no delegation shape
- **THEN** `Adaptive{}.Resolve` returns `"intermediate"`

#### Scenario: Empty SynthesisContext does not panic
- **WHEN** `synth` is the zero value (`Concepts == nil`, `CommitMsg == ""`, `DiffSnippet == ""`)
- **THEN** `Adaptive{}.Resolve` returns `"intermediate"` (the fallback) without panicking on the empty slice or empty string
