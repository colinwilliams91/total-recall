## Why

`RecallConfig.Difficulty` defaults to `"adaptive"` (`internal/config/config.go:147`) and the user can set it via `~/.tr/config.yaml`. The refactor-question-storage-schema change mandates that the value be injected into the synthesis system prompt, and our standalone fix (`7252429`) wires it through `server.go runPipeline` to `recall.Engine.Synthesize`. But `"adaptive"` reaches the AI as a literal, meaningless string — there is no logic that actually *adapts* the difficulty based on any signals. Today the field is a config-shaped hole: the name promises a capability the system does not implement.

`DOCS/CORE/DATA_ANALYSIS.md` §4 ("Pure Delegation Produced The Worst Outcomes") describes an adaptive difficulty mechanism that detects signals from AI-assisted workflows — massive generated diffs, low manual edit ratios, AI-generated commit signatures, large unexplained abstractions — and *increases conceptual questioning depth* in response. The synthesis-after-`synthesize-from-context` (Change A) change already enriches the `SynthesisContext` with concept weights, source mix, and a commit-message/diff-snippet pair that are exactly the raw inputs an adaptive resolver needs. Change B consumes those inputs to turn `"adaptive"` from a dead string into a real, signal-driven difficulty selection.

## What Changes

**New capability `difficulty-resolution` (`internal/recall/difficulty`):**
- New package `internal/recall/difficulty` defines a `Resolver` interface: `Resolve(ctx, concepts []cache.ConceptRow, commitMsg, diffSnippet string) string`. Two implementations ship in this change:
  - `Static`: returns the configured `RecallConfig.Difficulty` value verbatim, unless it equals `"adaptive"`, in which case it delegates to `Adaptive`. This is the fallback for users who don't opt into adaptive behavior (e.g., `difficulty: hard` always returns `"hard"`).
  - `Adaptive`: applies heuristics over the `SynthesisContext` signals to choose between `easy`, `intermediate`, and `hard`. The heuristics are derived from `DOCS/CORE/DATA_ANALYSIS.md` §4 (AI-delegation escalation), `assets/prompts/question-generation-policy.md` ("Pure AI-Delegation Produced The Worst Outcomes"), and the existing `RecallConfig.Difficulty == "adaptive"` default the user already opted into.

**Modified capability `recall-engine`:**
- `recall.Engine.Synthesize` signature reshapes from `(ctx, repo, branch, difficulty, model string, synth SynthesisContext)` (post-Change-A signature) to `(ctx, repo, branch, model string, synth SynthesisContext, resolver Resolver)`. The raw `difficulty string` parameter is removed; the resolver owns difficulty selection.
- `internal/recall/engine.go`'s `defaultDifficulty = "intermediate"` constant goes away — the `Resolver` owns the default. When the resolver returns `""`, `Synthesize` falls back to `"intermediate"` as the safety net.
- `server.go runPipeline` constructs the `Resolver` once at `Engine.New` time (loaded from the configured `RecallConfig.Difficulty` value); per `runPipeline` invocation it calls `resolver.Resolve(...)` to produce the difficulty passed to `SynthesisRequest`.

**Adaptive heuristics (codified in `Adaptive.Resolve`):**
- High cluster (average weight > 0.7 across ≥3 concepts) → escalate to `hard` (deep conceptual reasoning targets).
- Many dispersed (average weight < 0.3 across ≥5 concepts) → de-escalate to `easy` (broad recall, not deep reasoning).
- Large unexplained abstraction (commit message length < 50 chars AND diff snippet length > 400 chars) → escalate to `hard` (the "AI-delegation" signal from DATA_ANALYSIS §4).
- All-recent-source-code (all `ConceptRow.Source == "code"` in last N concepts) → escalate one level (concept-density signal).
- Default fallback when no signals fire → `intermediate` (preserves the prior `defaultDifficulty` for the common case).

## Capabilities

### New Capabilities
- `difficulty-resolution`: `Resolver` interface + `Static` and `Adaptive` implementations. Heuristics consume `cache.ConceptRow` weights/sources and the `SynthesisContext` commit-msg/diff-snippet to produce a difficulty string (`easy`/`intermediate`/`hard`). The configured `RecallConfig.Difficulty` routes to either resolver: any concrete value (`easy`/`intermediate`/`hard`) routes to `Static`; `"adaptive"` routes to `Adaptive`.

### Modified Capabilities
- `recall-engine`: `Engine.Synthesize` accepts a `Resolver` instead of a raw `difficulty string`. `Engine.New` constructs the resolver from `cfg.Recall.Difficulty`. `defaultDifficulty` constant removed; `Resolver` owns the default.

## Impact

- **Code:** New `internal/recall/difficulty` package (`resolver.go`, `static.go`, `adaptive.go`, ~200 lines total). `internal/recall/engine.go` (`Synthesize` signature change, drop `defaultDifficulty`, accept `Resolver`). `internal/engine/server.go` (`Engine.New` constructs resolver; `runPipeline` calls `resolver.Resolve`). `cmd/tr/wire.go` (or wherever the `Provider`/`Store`/`recall.Engine` are wired — verify) updated to construct the resolver from `cfg.Recall.Difficulty`.
- **Tests:** `internal/recall/difficulty/*_test.go` (Table-driven tests for `Static` and `Adaptive` — high cluster escalates, dispersed de-escalates, AI-delegation signal escalates, fallback `intermediate` when no signals). `cmd/tr/recall_test.go` (`Synthesize` called with a `Static` resolver returning `"hard"` — verify the prompt receives `"hard"`). `cmd/tr/integration_test.go` (config `"adaptive"` exercises the adaptive resolver; config `"hard"` exercises the static resolver via `Engine.New`).
- **APIs:** No external API change. `recall.Engine.Synthesize` is an internal-call interface; `server.go` updates atomic with this change.
- **Dependencies:** None new.
- **Specs:** This change adds `openspec/specs/difficulty-resolution/spec.md` and updates `openspec/specs/recall-engine/spec.md`.

## Key Design Decisions

- **`Resolver` interface, not a free function.** Two implementations today (`Static`, `Adaptive`); future implementations (spaced-repetition-based on `question_events` audit history, performance-based on `correct` ratio, model-tier-based on provider family) plug in without touching `Synthesize`. The interface boundary is the right one — difficulty selection is separable from system-prompt construction.

- **Adaptive is signal-based heuristics, not a second AI call.** DATA_ANALYSIS §4 identifies computable signals (diff size, message length, concept density); we apply deterministic weights (not learned, not ML). This avoids the cost and latency of a second `provider.Complete` call per synthesis; the resolver is a pure function over the `SynthesisContext`.

- **`"adaptive"` stops being a dead string.** Today it reaches the prompt as a literal word; after this change it routes to the `Adaptive` resolver which returns a *real* difficulty (`easy`/`intermediate`/`hard`) before the prompt is constructed. The literal `"adaptive"` never reaches the system prompt — only the resolved concrete value does.

- **Fallback to ` intermediate` when resolver returns empty.** The `Synthesize` method retains one defensive fallback: if the resolver returns `""` (e.g., a future resolver implementation returns zero-value), `Synthesize` substitutes `"intermediate"` before calling `SynthesisRequest`. This preserves the prior `defaultDifficulty = "intermediate"` behavior as a safety net without elevating the constant to a public API.

- **Sequenced after Change A.** The `Adaptive` resolver reads `SynthesisContext.Concepts` (rich `[]cache.ConceptRow`), `CommitMsg`, and `DiffSnippet` — all three are introduced by Change A. Change B is dependent on Change A landing first. Branching from post-Change-A main avoids the rebase cost of synthesizing the `SynthesisContext` shape before Change A defines it.

- **No persistence of resolved difficulty.** The resolver returns a per-call difficulty string that drives the prompt; we do not store it on `questions` rows. The `question_events` audit spine introduced by the refactor could record the resolved difficulty in the `payload` JSON of the `'queued'` event — this is captured as an open task, not a hard requirement, to avoid entangling with the refactor's event schema beyond its existing extensibility hook.

## Non-Goals

- Spaced-repetition scheduling — the `question_events` audit spine supports it (per the refactor's design doc), but the algorithm is deferred. A `SpacedRepetition` resolver implementation is a future change.
- Per-question performance-based difficulty — requires aggregating `correct` ratios across answer history; future change. The `Performance` resolver implementation is a stub.
- User-facing difficulty visibility — the difficulty reaches the system prompt only; the user does not see which difficulty was chosen. A future TUI addition could surface it as a header in `renderQuestion` if signal-driven difficulty proves to be a useful UX cue.
- Persisting resolved difficulty on the `questions` row — schema change entangled with the refactor; deferred. The `question_events` `payload` JSON is the natural future home.
- Tuning the heuristic thresholds via config — the constants (`0.7`, `0.3`, `400`, `50`, `≥3`, `≥5`) are package-private in this change. A future `recall.difficulty.thresholds` config section could surface them, but premature before the A/B spike (task 5.1) confirms the signals are useful.
- A second AI call to assess "is this commit AI-delegated?" — the heuristics are deterministic; the AI-delegation determination is made from signals, not from a model judgement.