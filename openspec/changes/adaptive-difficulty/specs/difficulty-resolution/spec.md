# difficulty-resolution

## ADDED Requirements

### Requirement: `Resolver` interface exposes difficulty selection as a separable concern from synthesis prompt construction
The `internal/recall/difficulty` package exposes a `Resolver` interface with a single method `Resolve(ctx context.Context, synth recall.SynthesisContext) string`. The returned string SHALL be one of `"easy"`, `"intermediate"`, `"hard"`, or `""` (the empty string indicating the resolver elected no preference and the caller should substitute a default). `Synthesize` and `SynthesisRequest` consume only this string; they do not inspect the heuristics or signals that produced it.

#### Scenario: Resolver returns a concrete difficulty
- **WHEN** any `Resolver` implementation's `Resolve` is called with a valid `SynthesisContext`
- **THEN** the returned string is `"easy"`, `"intermediate"`, `"hard"`, or `""` — no other value is permitted

#### Scenario: Resolver is stateless across calls
- **WHEN** the same `Resolver` instance is called twice in a row with the same `SynthesisContext`
- **THEN** both calls return the same value; the resolver does not mutate per-call state

---

### Requirement: `Static` resolver returns the configured value verbatim, delegating to `Adaptive` only when the value is `"adaptive"`
`Static{value string}` SHALL implement `Resolver`. `Resolve(ctx, synth)` MUST return `s.value` verbatim when `s.value != "adaptive"` and MUST return `Adaptive{}.Resolve(ctx, synth)` otherwise. Empty string SHALL be preserved as-is (the `Synthesize` caller substitutes `"intermediate"` as a safety net); unknown strings (`"foo"`, typos) MUST be returned verbatim and propagate to the prompt — this is a defense-in-depth concern, mitigated by config validation in the loader.

#### Scenario: Static with concrete difficulty returns verbatim
- **WHEN** `Static{value: "hard"}.Resolve(ctx, synth)` is called with any `synth`
- **THEN** the return value is `"hard"` regardless of `synth` content

#### Scenario: Static with adaptive delegates to Adaptive
- **WHEN** `Static{value: "adaptive"}.Resolve(ctx, synth)` is called with `synth` carrying high-cluster signals (3 concepts w/ avg weight 0.9)
- **THEN** the return value matches what `Adaptive{}.Resolve(ctx, synth)` would return for the same `synth` (`"hard"`, per the high-cluster heuristic)

#### Scenario: Static with empty string returns empty
- **WHEN** `Static{value: ""}.Resolve(ctx, synth)` is called with any `synth`
- **THEN** the return value is `""`; the caller (`Synthesize`) substitutes `"intermediate"` before invoking `SynthesisRequest`

---

### Requirement: `Adaptive` resolver applies a first-match heuristic table over `SynthesisContext` signals
`Adaptive{}` SHALL implement `Resolver`. `Resolve(ctx, synth)` MUST evaluate the following heuristics in order; the first match wins and produces the returned difficulty:

1. **AI-delegation signal** — `len(synth.CommitMsg) < 50 AND len(synth.DiffSnippet) > 400` → returns `"hard"`
2. **High-cluster signal** — `avg(synth.Concepts[*].Weight) > 0.7 AND len(synth.Concepts) >= 3` → returns `"hard"`
3. **Dispersed signal** — `avg(synth.Concepts[*].Weight) < 0.3 AND len(synth.Concepts) >= 5` → returns `"easy"`
4. **All-code source signal** — `len(synth.Concepts) >= 5 AND all(synth.Concepts[*].Source == "code")` → escalates the fallback by one level; returns `"hard"` (the fallback `"intermediate"` escalated one level)
5. **Fallback** — no heuristic matched → returns `"intermediate"`

The thresholds are package-private constants in `internal/recall/difficulty/adaptive.go`: `adaptiveHighClusterWeight = 0.7`, `adaptiveDispersedWeight = 0.3`, `adaptiveMinHighClusterConcepts = 3`, `adaptiveMinDispersedConcepts = 5`, `adaptiveDelegationMsgMaxChars = 50`, `adaptiveDelegationSnippetMinChars = 400`, `adaptiveMinAllCodeSourceConcepts = 5`.

#### Scenario: AI-delegation signal fires
- **WHEN** `synth.CommitMsg = "fix:"` (5 chars) and `synth.DiffSnippet` is 500 chars
- **THEN** `Adaptive{}.Resolve` returns `"hard"` regardless of `synth.Concepts` content

#### Scenario: High-cluster signal fires (no AI-delegation)
- **WHEN** `synth.CommitMsg` is 200 chars (no AI-delegation signal), `synth.Concepts` has 3 entries with weights `[0.9, 0.8, 0.7]` (avg = 0.8)
- **THEN** `Adaptive{}.Resolve` returns `"hard"`

#### Scenario: High-cluster threshold not met (too few concepts)
- **WHEN** `synth.Concepts` has 2 entries with weights `[0.9, 0.9]` (avg = 0.9 but `len < 3`)
- **THEN** the high-cluster heuristic does not match; evaluation continues to the next rule

#### Scenario: Dispersed signal fires
- **WHEN** `synth.Concepts` has 5 entries with weights all `0.2` (avg = 0.2) and no AI-delegation signal
- **THEN** `Adaptive{}.Resolve` returns `"easy"`

#### Scenario: All-code source signal escalates fallback
- **WHEN** `synth.Concepts` has 5 entries all with `Source = "code"`, avg weight `0.5` (no high-cluster, no dispersed, no AI-delegation)
- **THEN** `Adaptive{}.Resolve` returns `"hard"` (fallback `"intermediate"` escalated one level)

#### Scenario: Fallback with no signal match
- **WHEN** `synth.Concepts` has 3 entries with avg weight `0.5` (no high-cluster, too few for dispersed, mixed sources), and `synth.CommitMsg`/`DiffSnippet` are empty (no AI-delegation)
- **THEN** `Adaptive{}.Resolve` returns `"intermediate"`

#### Scenario: Empty SynthesisContext does not panic
- **WHEN** `synth` is the zero value (`Concepts == nil`, `CommitMsg == ""`, `DiffSnippet == ""`)
- **THEN** `Adaptive{}.Resolve` returns `"intermediate"` (the fallback) without panicking on the empty slice or empty string

#### Scenario: AI-delegation wins over high-cluster when both fire
- **WHEN** both AI-delegation (short msg, large snippet) and high-cluster signals are present
- **THEN** `Adaptive{}.Resolve` returns `"hard"` (both produce `"hard"`; the first-match ordering does not conflict here)

#### Scenario: High-cluster wins over dispersed when both match
- **WHEN** `synth.Concepts` has 5 entries with avg weight `0.8` (matches high-cluster AND has `len >= 5`)
- **THEN** `Adaptive{}.Resolve` returns `"hard"` (high-cluster is evaluated before dispersed per the ordering)

---

### Requirement: Adaptive resolver emits a per-call log line naming the matched signal and selected difficulty
`Adaptive.Resolve` SHALL emit `log.Printf("[recall] adaptive resolver selected %q (signals: %s)", level, signalsFired)` on every resolution, where `signalsFired` is a comma-joined list of the matched heuristic names (`"ai-delegation"`, `"high-cluster"`, `"dispersed"`, `"all-code"`, `"fallback"`). The log line targets the daemon stderr / log stream and is intended for developers debugging their config (e.g., "why are my questions always hard?"). Production usage is low-volume (one line per synthesis call, which is once per commit).

#### Scenario: AI-delegation signal logged
- **WHEN** the AI-delegation heuristic fires and the resolver returns `"hard"`
- **THEN** the log line reads `[recall] adaptive resolver selected "hard" (signals: ai-delegation)`

#### Scenario: Fallback logged
- **WHEN** no heuristic matches and the resolver returns `"intermediate"`
- **THEN** the log line reads `[recall] adaptive resolver selected "intermediate" (signals: fallback)`

#### Scenario: All-code escalation logged
- **WHEN** the all-code source heuristic fires and escalates the fallback to `"hard"`
- **THEN** the log line reads `[recall] adaptive resolver selected "hard" (signals: all-code)`