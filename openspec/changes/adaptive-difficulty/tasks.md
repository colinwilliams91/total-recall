## 1. New package `internal/recall/difficulty`

- [ ] 1.1 Create package `internal/recall/difficulty` with `resolver.go` exporting the `Resolver` interface: `Resolve(ctx context.Context, synth recall.SynthesisContext) string`. Verify the import path is stable (`recall.SynthesisContext` lives in `internal/recall`; the new package depends on it)
- [ ] 1.2 Package-private threshold constants in `adaptive.go` (highest values, first-match ordering): `adaptiveHighClusterWeight = 0.7`, `adaptiveDispersedWeight = 0.3`, `adaptiveMinHighClusterConcepts = 3`, `adaptiveMinDispersedConcepts = 5`, `adaptiveDelegationMsgMaxChars = 50`, `adaptiveDelegationSnippetMinChars = 400`
- [ ] 1.3 `static.go` implements `Static{ value string }` with `Resolve(ctx, synth) string` returning `s.value` verbatim when `s.value != "adaptive"`, otherwise delegating to `Adaptive{}.Resolve(ctx, synth)`
- [ ] 1.4 `adaptive.go` implements `Adaptive{}` with `Resolve(ctx, synth) string` applying the heuristic table (first-match order): AI-delegation → `"hard"`; high cluster → `"hard"`; dispersed → `"easy"`; all-code → escalate fallback by one level (`"intermediate"` → `"hard"`, others unchanged); else `"intermediate"`. Computes `avg(weights)` and `all sources == "code"` from `synth.Concepts`; reads `synth.CommitMsg`/`synth.DiffSnippet` lengths directly
- [ ] 1.5 Tests `internal/recall/difficulty/static_test.go`:
  - [ ] 1.5.1 `TestStaticVerbatim`: `Static{value: "hard"}.Resolve(...)` returns `"hard"` regardless of `synth` content
  - [ ] 1.5.2 `TestStaticAdaptiveDelegatesToAdaptive`: `Static{value: "adaptive"}.Resolve(...)` with high-cluster signals returns `"hard"` (proves delegation)
  - [ ] 1.5.3 `TestStaticEmptyStringReturnsEmpty`: `Static{value: ""}.Resolve(...)` returns `""` (the safety net in `Synthesize` handles this)
- [ ] 1.6 Tests `internal/recall/difficulty/adaptive_test.go` (table-driven, one row per signal):
  - [ ] 1.6.1 `TestAdaptiveAIDelegationSignal`: short msg (5 chars) + large snippet (500 chars) → `"hard"`, even with low weights
  - [ ] 1.6.2 `TestAdaptiveHighClusterSignal`: 3 concepts w/ avg weight 0.8 → `"hard"`
  - [ ] 1.6.3 `TestAdaptiveHighClusterNotEnoughConcepts`: 2 concepts w/ avg weight 0.8 → no cluster match (falls through)
  - [ ] 1.6.4 `TestAdaptiveDispersedSignal`: 5 concepts w/ avg weight 0.2 → `"easy"`
  - [ ] 1.6.5 `TestAdaptiveAllCodeSourceEscalates`: 5 concepts all `Source="code"`, no other signal → `"hard"` (one level up from `"intermediate"`)
  - [ ] 1.6.6 `TestAdaptiveFallbackNoSignals`: empty `synth.Concepts` and empty commit context → `"intermediate"`
  - [ ] 1.6.7 `TestAdaptiveOrderingAIDelegationWinsOverCluster`: both AI-delegation and high-cluster signals fire → `"hard"` (the resolution value is the same — verify no double-escalation path bugs)
  - [ ] 1.6.8 `TestAdaptiveEmptyCtxConceptsReturnsIntermediate`: `synth.Concepts == nil` → `"intermediate"` (no panic on zero value)

## 2. `Engine.Synthesize` signature reshape (`internal/recall/engine.go`)

- [ ] 2.1 Change signature: `Synthesize(ctx, repo, branch, model string, synth SynthesisContext, resolver Resolver) (*Question, error)` — drop the `difficulty string` parameter; add `resolver Resolver` (the `Resolver` type is imported from `internal/recall/difficulty`)
- [ ] 2.2 Inline the difficulty call at the top of `Synthesize`: `difficulty := resolver.Resolve(ctx, synth); if difficulty == "" { difficulty = "intermediate" }` (defensive safety net preserving the prior `defaultDifficulty` behavior)
- [ ] 2.3 Remove `const defaultDifficulty = "intermediate"` from engine.go:13 — the resolver is now the source of truth; the empty-string fallback is the only safety net left in `Synthesize`
- [ ] 2.4 Verify the existing short-circuits (`repo == "" || branch == ""` → `nil, nil`; `len(synth.Concepts) == 0` → `nil, nil`) do not call `resolver.Resolve` — empty contexts should not pay the resolution cost
- [ ] 2.5 Tests in `cmd/tr/recall_test.go`:
  - [ ] 2.5.1 `TestSynthesizeWithStaticResolverHard`: a `Static{value: "hard"}` resolver produces a prompt (captured via a stub provider) whose system turn contains `"hard"`
  - [ ] 2.5.2 `TestSynthesizeResolverNotCalledForEmptyRepo`: `Synthesize(ctx, "", "branch", ...)` returns `nil, nil` without calling `resolver.Resolve` (assert via a counting resolver that fails the test if `Resolve` is invoked)
  - [ ] 2.5.3 `TestSynthesizeEmptyResolverResultFallsBackToIntermediate`: a resolver returning `""` produces a prompt whose system turn contains `"intermediate"`

## 3. `Engine.New` constructs the resolver (`internal/recall/engine.go` + wiring call site)

- [ ] 3.1 Add `Resolver` field to `Engine` struct; `New(provider, store, cfg)` constructs either `Static{value: cfg.Recall.Difficulty}` (when `cfg.Recall.Difficulty != ""`) or a default that handles the empty case (e.g., `Static{value: "adaptive"}` — preserves prior `defaultDifficulty` behavior for users with an unset config field)
- [ ] 3.2 Verify the `recall.New` call site (in `cmd/tr/wire.go` or wherever the Engine is constructed from `cfg` — grep for `recall.New`) passes the `cfg` through
- [ ] 3.3 Tests: `TestEngineNewAdaptiveConfigUsesAdaptiveResolver` asserts an `Engine` constructed with `cfg.Recall.Difficulty == "adaptive"` exhibits adaptive behavior on `Synthesize` calls; `TestEngineNewHardConfigUsesStaticResolver` asserts the same with `"hard"`
- [ ] 3.4 Integration: `cmd/tr/integration_test.go` — a daemon constructed with `RecallConfig{Difficulty: "adaptive"}` and a stubbed provider captures the prompt; verify the prompt's difficulty directive is a concrete `"easy"`/`"intermediate"`/`"hard"` value (never the literal `"adaptive"`) when the adaptive signals are seeded into the test store

## 4. Config validation (defense against typos)

- [ ] 4.1 Verify the existing config-load validator (`internal/config/`) either checks `RecallConfig.Difficulty` against `easy`/`intermediate`/`hard`/`adaptive` or adds that check; unknown values refuse the config or warn loudly. (Captured in this change because the resolver's `Static` is no longer the only path that touches the configured value — a typo silently reaching the prompt as a meaningless literal would degrade question quality without an actionable error)
- [ ] 4.2 Tests `cmd/tr/config_test.go`:
  - [ ] 4.2.1 `TestConfigRejectsUnknownDifficulty`: a config with `difficulty: "invalid"` fails load or warns
  - [ ] 4.2.2 `TestConfigAcceptsAdaptive`: `difficulty: "adaptive"` passes load (preserves existing default)
  - [ ] 4.2.3 `TestConfigAcceptsEasyIntermediateHard`: the three concrete values pass load

## 5. A/B spike (manual exploration tool)

- [ ] 5.1 Write `scripts/e2e/adaptive-ab.ps1` (and `.sh` for cross-platform parity per the `hooks/` convention): sets up a scratch repo with synthetic commits bracketing each adaptive signal (one commit triggering AI-delegation, one triggering high-cluster, one dispersed, one all-code, one no-signal); runs `tr serve` and `tr ask` after each; logs the resolved difficulty (the resolver should log `[recall] adaptive resolver selected <level> (signals: <list>)` per task 6.1); captures 10 questions per signal arm for human side-by-side comparison. Not a CI test; documented as an exploration tool in `scripts/e2e/README.md`
- [ ] 5.2 Run the spike locally, capture observations in `DOCS/CORE/DATA_ANALYSIS.md` appendix or a new `DOCS/CORE/ADAPTIVE_SPIKE.md` — anecdotal data on which signals produce which question quality. Not blocking; informs future threshold tuning

## 6. Observability + documentation sync

- [ ] 6.1 Logging: in `Adaptive.Resolve`, emit `log.Printf("[recall] adaptive resolver selected %q (signals: %s)", level, strings.Join(signalsFired, ", "))` on every resolution (the `signalsFired` slice lists which heuristic matched — "ai-delegation" / "high-cluster" / "dispersed" / "all-code" / "fallback"). Goal: a developer debugging their config can `tr serve` and tail the log to see why their questions are getting easy/hard
- [ ] 6.2 `AGENTS.md` — Recall Engine section: note the `Resolver` interface and the two implementations; note that `"adaptive"` routes to the adaptive resolver; note the heuristic table lives in `internal/recall/difficulty/adaptive.go`
- [ ] 6.3 Future-followup task (out of scope, captured here for future-self): write the resolved difficulty into the `'queued'` `question_events` row's `payload` JSON column as `{"resolved_difficulty": "<level>", "signals": [...]}`. Requires threading `resolver.Resolve`'s result through `runPipeline → store.SaveQuestion`; defer to avoid entangling with the refactor's schema
- [ ] 6.4 `DOCS/CORE/DATA_ANALYSIS.md` — note that the §4 adaptive recommendation is now implemented; cite the heuristic-map table

## 7. Final verification

- [ ] 7.1 `go build ./...`
- [ ] 7.2 `go vet ./...`
- [ ] 7.3 `go test ./...`
- [ ] 7.4 `golangci-lint run` (if installed)
- [ ] 7.5 Manual: configure `~/.tr/config.yaml` with `recall.difficulty: adaptive`. Commit an AI-delegation-shaped commit (large diff, "fix:" 5-char message) in a scratch repo. Tail the daemon log. Verify the resolver emits `selected "hard" (signals: ai-delegation)`. Commit a small hand-written change. Verify the resolver emits `selected "intermediate" (signals: fallback)` or an escalated variant
- [ ] 7.6 Manual: switch config to `recall.difficulty: hard`. Repeat the same commits. Verify the resolver always emits `selected "hard"` (Static passthrough); the prompt reflects `"hard"`; adaptive signals are not consulted