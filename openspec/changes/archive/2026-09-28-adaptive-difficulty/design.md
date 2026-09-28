## Context

`RecallConfig.Difficulty` defaults to `"adaptive"` (`internal/config/config.go:147`) and the user can override it via `~/.tr/config.yaml` under `recall.difficulty`. The refactor-question-storage-schema change requires the value be injected into the synthesis system prompt, and our standalone fix (`7252429`) wires `s.cfg.Recall.Difficulty` through `server.go runPipeline` to `recall.Engine.Synthesize`. But "adaptive" reaches the AI as a literal string today — there is no logic that adjusts based on signals.

`DOCS/CORE/DATA_ANALYSIS.md` §4 ("Pure Delegation Produced The Worst Outcomes") describes an adaptive mechanism that identifies AI-assisted workflow signals — massive generated diffs, low manual edit ratios, AI-generated commit signatures, large unexplained abstractions — and *intentionally increases conceptual questioning depth*. The DATA_ANALYSIS doc explicitly calls this "an adaptive difficulty mechanism [that] could become extremely valuable" as a product differentiator. Today the user-facing config knob is in place; the implementation behind it is an empty promise.

After Change A (`synthesize-from-context`) lands, `recall.Engine.Synthesize` accepts a `SynthesisContext{Concepts []cache.ConceptRow, CommitMsg, DiffSnippet string}`. The context carries exactly the signals an adaptive resolver needs: concept weights (`ConceptRow.Weight`), source mix (`ConceptRow.Source`), commit message length, and diff snippet size. This change consumes those signals and turns `"adaptive"` from a placeholder into a real selection between concrete difficulty levels.

## Goals / Non-Goals

**Goals:**
- Define a `Resolver` interface in `internal/recall/difficulty` that separates difficulty selection from synthesis prompt construction. Two implementations ship today (`Static`, `Adaptive`); the interface permits future `SpacedRepetition`, `Performance`, and `ModelTier` resolvers without touching `Synthesize`.
- Implement the `Adaptive` resolver's heuristics over the `SynthesisContext`: high-weight concept clusters escalate, dispersed low-weight concepts de-escalate, the AI-delegation signal (short commit message + large diff) escalates, all-code-source concepts escalate, the empty-signals case falls back to `intermediate`.
- Reshape `Engine.Synthesize` to accept a `Resolver` instead of a raw `difficulty string`. The resolver is constructed once at `Engine.New` from `cfg.Recall.Difficulty`; per-synthesis-call, `runPipeline` invokes `resolver.Resolve(...)` to produce the difficulty passed to `SynthesisRequest`.
- Eliminate the `defaultDifficulty` constant in `internal/recall/engine.go`. The `Resolver` owns the default; a defensive fallback to `"intermediate"` remains in `Synthesize` only when the resolver returns `""` (zero-value safety, not a public default).
- Preserve everything Change A introduced: the `SynthesisContext`, the loaded policy doc, the AI JSON contract, and the refactor's `[]Choice` `Question` shape are all untouched.

**Non-Goals:**
- Spaced-repetition scheduling or performance-based resolvers — future implementations; their absence does not block this change.
- Persisting the resolved difficulty on the `questions` row — schema entanglement with the refactor; deferred. The `question_events` `'queued'` event's `payload` JSON column is the natural future home.
- Surfacing the resolved difficulty to the user in the TUI — silent in this change.
- Config-exposed heuristic thresholds — package-private constants in this change; A/B spike (task 5.1) must first confirm the signals are useful.
- A second AI call to judge "is this commit AI-delegated?" — deterministic heuristics only.

## Decisions

### Decision: `Resolver` interface, not a resolution function

`internal/recall/difficulty` exposes `Resolver`:

```go
type Resolver interface {
    Resolve(ctx context.Context, synth SynthesisContext) string
}
```

Two implementations ship today: `Static` (returns the configured value, delegates to `Adaptive` when the value is `"adaptive"`), and `Adaptive` (heuristics-driven). Future resolver implementations (spaced-repetition, performance, model-tier) slot in without modifying `Engine.Synthesize` — the method accepts a `Resolver`, not a concrete type.

**Alternatives considered:**
- *Free function `ResolveDifficulty(cfg, synth)` at the call site.* Rejected: future resolvers would either re-open the function signature (churn cascade through callers) or be dispatched inside the function by a `switch` on `cfg.Difficulty` (conflates dispatch with heuristic). The interface keeps dispatch in `Engine.New` and heuristics in the implementation.
- *`string`-typed difficulty as a switch.*
  Rejected: the resolver's output is a closed vocabulary (`easy`/`intermediate`/`hard`) but its inputs are open (`SynthesisContext` shape evolves with signal research); the interface captures that asymmetry.
- *Resolver struct with method set, not interface.* Rejected: Go-test friendliness and future-implementation seam (a fake resolver for `Synthesize` tests is one-liner).

### Decision: `Engine.New` constructs the `Resolver` once per process

`recall.New(provider, store, cfg)` constructs either `Static{value: cfg.Recall.Difficulty}` (when `cfg.Recall.Difficulty != "adaptive"`) or `Adaptive{thresholds: defaultThresholds}` (when `cfg.Recall.Difficulty == "adaptive"`). The resolver is stored on the `Engine` struct; `runPipeline` calls `engine.resolver.Resolve(ctx, synth)` to produce the difficulty for that call.

**Alternatives considered:**
- *Construct per-call in `runPipeline`.*
  Rejected: redundant work; the configured `Difficulty` does not change at runtime. Move to per-call only if a future hot-reload config lands.
- *Inject the `Resolver` as a separate arg to `Engine.New`.*
  Equivalent; we choose to construct internally from `cfg` for parity with the existing `recall.New` signature, which already takes `cfg`. Decoupling the resolver as an injected dependency is a follow-up if the wiring complexity grows.

### Decision: `Static` delegates to `Adaptive` when `value == "adaptive"`

`Static{value: "adaptive"}.Resolve(...)` invokes `Adaptive{}.Resolve(...)` rather than returning the literal string `"adaptive"`. This keeps the routing logic in `Static` (the user's expressed intent) and the heuristics in `Adaptive` (the implementation). A user setting `difficulty: hard` gets `Static{value: "hard"}.Resolve(...)` → `"hard"`; a user with the default `difficulty: adaptive` (or omitting the field) gets `Static{value: "adaptive"}` → `Adaptive{}.Resolve(...)` → concrete level.

**Alternatives considered:**
- *Dispatch in `Engine.New` instead.* Equivalent complexity; dispatching inside `Static.Resolve` keeps the routing rule co-located with the value it routes on and lets `Engine.New` be a one-liner per branch.
- *Reject `"adaptive"` at config-load time if no adaptive resolver is registered.* Rejected: the adaptive resolver is always available; rejecting complicates the config validator with a runtime concern.

### Decision: Heuristics are deterministic thresholds over `SynthesisContext`

The `Adaptive.Resolve` heuristics, in evaluation order (first match wins):

```
1. AI-delegation signal — commit msg < 50 chars AND diff snippet > 400 chars  →  "hard"
2. High cluster          — avg ConceptRow.Weight > 0.7 AND len(concepts) >= 3   →  "hard"
3. Dispersed             — avg ConceptRow.Weight < 0.3 AND len(concepts) >= 5   →  "easy"
4. All-code source       — all ConceptRow.Source == "code" AND len >= 5         →  escalate one level from fallback
5. Fallback              — no signal fired                                       →  "intermediate"
```

The thresholds are package-private constants:
```go
const (
    adaptiveHighClusterWeight   = 0.7
    adaptiveDispersedWeight      = 0.3
    adaptiveMinHighClusterConcepts = 3
    adaptiveMinDispersedConcepts   = 5
    adaptiveDelegationMsgMaxChars   = 50
    adaptiveDelegationSnippetMinChars = 400
)
```

Order matters: the AI-delegation signal wins outright (`"hard"`); the cluster and dispersed signals are next; the all-code signal escalates the fallback by one level (`"intermediate\" → \"hard"`).

**Alternatives considered:**
- *Weighted sum across all signals.* Rejected: adds opaque tuning constants; the chosen first-match ordering is debuggable by inspection and matches DATA_ANALYSIS §4's framing (delegation is the strongest anti-pattern).
- *Machine-learned thresholds.* Rejected: insufficient data volume (a single developer commits at most dozens of times per day); the heuristic interpretation is legible to the user debugging their config.
- *Per-repo thresholds.* Rejected: the resolver is per-call; per-repo state would force a `repo`-keyed cache and entangle with the refactor's per-repo schema. Per-call statelessness preserves testability.

### Decision: Resolved difficulty is not persisted in this change

`runPipeline` calls `resolver.Resolve(...)` and passes the result to `Synthesize`. The resolved value is not stored on the `questions` row. A future change could write the resolved difficulty into the `'queued'` event's `payload` JSON (the refactor's `question_events` schema has a nullable `payload` column for exactly this kind of extension) to audit which difficulty produced which question — captured as Open Question, not blocked on here. The paranoia budget for entangling with the refactor's event schema is reserved for a future change.

**Alternatives considered:**
- *Denormalize a `difficulty TEXT` column on `questions`.* Rejected: schema change entangled with the refactor; no consumer today; the `question_events` audit spine is the idiomatically correct home.
- *Write to `question_events.payload` of the `'queued'` event in this change.* Rejected: scope creep; the refactor's `SaveQuestion` writes the `'queued'` event and we'd have to thread the resolver result through `engine.runPipeline` → `store.SaveQuestion`, which is the refactor's territory.

## Risks / Trade-offs

- **[Trade-off] Heuristic thresholds are uncalibrated.** The constants (`0.7`, `0.3`, `400`, `50`, `≥3`, `≥5`) are best-guess starting points drawn from the research narrative in `DATA_ANALYSIS.md`, not from measured signal-to-outcome correlations. Mitigated by an A/B spike (task 5.1) and by the package-private scoping — tuning the thresholds in a follow-up is a 6-line diff.
- **[Trade-off] "Adaptive always escalates" is a bias risk.** Three of the four signals escalate (`hard`); only one (`dispersed`) de-escalates. A developer whose every commit trips the AI-delegation signal will *always* get `hard` questions — fatiguing. Mitigated by the cluster and fallback signals firing for non-delegated workflows, and by the future spaced-repetition resolver which will modulate based on answer history.
- **[Risk] `Static.Resolve` with a typo in the configured value (e.g., `difficulty: intermediet") silently reaches the prompt as the literal typo.** Mitigated by config-load validation: the `Difficulty` field is constrained to `easy`|`intermediate`|`hard`|`adaptive`; unknown values are refused or warned at config load time (capture as task 4.3 — verify the existing config validator does this or adds it).
- **[Risk] Adaptive behavior is opaque to the user.** A user setting `difficulty: adaptive` cannot easily see why a given question was hard. Mitigated by logging `[recall] adaptive resolver selected <level> (signals: <list>)` on every resolution; a future TUI addition could surface the same info in `renderQuestion`.
- **[Open] Should the resolver receive `repo`/`branch` so it can correlate future per-repo answer history with current signals?** Not in this change — the future spaced-repetition resolver will need them; the current two implementations do not. Captured as an open task to revisit when the first history-aware resolver lands.

## Migration Plan

1. Branch this change from post-Change-A main (which is itself post-refactor main).
2. Implement `internal/recall/difficulty` package; `Resolver` interface, `Static`, `Adaptive` per tasks.
3. Reshape `Engine.Synthesize` signature: drop `difficulty string`, add `resolver Resolver`. Update `recall.New` to construct the resolver from `cfg.Recall.Difficulty`.
4. Update `server.go runPipeline` to construct `SynthesisContext` (already done by Change A) and call `engine.resolver.Resolve(ctx, synth)` to get the difficulty string before calling `SynthesisRequest`.
5. Tests assert: `Static` returns verbatim; `Adaptive` returns `hard`/`easy`/`intermediate` per the table; `Synthesize` with `Static` returning `"hard"` produces a prompt containing `"hard"`; integration with config `"adaptive"` and stubbed signals exercises the adaptive path.
6. No migration script, no schema change, no wire change. Pure Engine + new package.

## Open Questions

- Should the resolver receive `repo`/`branch` for future cross-call state? Captured in design risks — deferred until the first history-aware resolver lands.
- Should the resolved difficulty be audit-logged into the `question_events` `'queued'` event's `payload`? Captured as a follow-up task (task 6.3) — out of scope here to avoid entangling with the refactor's schema.
- Threshold calibration — should we expose `recall.difficulty.thresholds` as config sub-keys? Not until the A/B spike (task 5.1) returns signal-quality data. Captured as a Non-Goal.