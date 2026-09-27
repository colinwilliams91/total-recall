## 1. `Resolution` contract widening + heuristic reorder (`internal/recall/difficulty`)

- [ ] 1.1 Introduce `Resolution` type in `resolver.go`: `Resolution{Difficulty string, FocusConcepts []string}`; widen `Resolver.Resolve` to return `Resolution`. `""` difficulty semantics unchanged (caller substitutes the safety-net default)
- [ ] 1.2 `Static.Resolve` returns `Resolution{Difficulty: s.value, FocusConcepts: nil}` verbatim when the value is not `"adaptive"`; delegation to `Adaptive` unchanged on `"adaptive"`; empty-string preservation semantics unchanged
- [ ] 1.3 `Adaptive.Resolve` returns `Resolution{Difficulty: level, FocusConcepts: nil}` with the heuristic table reordered: high-cluster → dispersed → all-code → AI-delegation (final commit-shape rule) → fallback. Threshold constants unchanged
- [ ] 1.4 Update the per-call log line to include the demotion anchor — the same `[recall] adaptive resolver selected <level> (signals: <list>)` format; `ai-delegation` remains a signal name (no double-escalation path)
- [ ] 1.5 Tests `internal/recall/difficulty/static_test.go`: verbatim paths return `Resolution` with empty focus; adaptive delegation reaches heuristics; empty-string case preserved
- [ ] 1.6 Tests `internal/recall/difficulty/adaptive_test.go`: reordered first-match table — dispersed wins over delegation when both fire; delegation still escalates when alone; high-cluster/all-code/fallback scenarios re-asserted; empty-context no-panic preserved
- [ ] 1.7 Update in-tree callers: `internal/recall/engine.go` (`Synthesize` destructures `Resolution`), `cmd/torec/recall_test.go` stub resolvers, integration test resolver fakes — signature widening only, no behavior change in these call sites beyond adoption

## 2. Mastery aggregation (`internal/cache`)

- [ ] 2.1 Add read-only stats interface + store-backed query joining `'answered'` `question_events` → per-question correctness (selections vs `choices.is_correct`) → linked concept ids, scoped repo+branch, window = last N answered. Returns per-question outcome rows (question id, correct bool, targeted concept slugs). Skips excluded from ratios
- [ ] 2.2 Windowed aggregate helper: correctness ratio + answered count over the window; per-concept miss/hit tallies (concept → misses, hits, last-answered-at) honoring the focus cap target granularity
- [ ] 2.3 Persist resolution metadata at queue time: extend the engine's `SaveQuestion` path so the `'queued'` question_events row's `payload` JSON carries `{"resolved_difficulty": "<level>", "signals": [...]}` — the deferred task 6.3 of `adaptive-difficulty`; no CHECK/table changes required
- [ ] 2.4 Tests `cache_test.go`: correctness join over seeded answered questions; skip exclusion; payload persistence on the queued event; empty-history returns zero stats without error

## 3. `Performance` resolver (`internal/recall/difficulty/performance.go`)

- [ ] 3.1 Implement `Performance` resolver wrapping the `Adaptive` baseline: resolve baseline difficulty via the (reordered) `Adaptive` heuristics, then apply the learner tier — ratio ≥ 0.92 → escalate one level; ratio < 0.50 → de-escalate one level; mid-range unchanged; `hard` caps escalation (no double-escalation), `easy` floors de-escalation
- [ ] 3.2 Insufficient-history gate: fewer than the minimum answered count (default 5, package-private constant) → return the baseline resolution verbatim with empty focus
- [ ] 3.3 Focus-concept ranking from the mastery stats: recent misses first (miss count descending, most-recent-first tiebreak), concept must link to ≥1 answered question in the window, zero-outcome concepts excluded, all-recently-correct → empty focus, cap at 3, cross-surface concepts fill only leftover slots
- [ ] 3.4 Stats interface seam: constructor injects a narrow learner-stats provider (per-repo/branch) rather than the raw store; nil/unavailable provider → behave as cold start (baseline only) — never block synthesis
- [ ] 3.5 Logging: resolution log line names the fired signal(s) — `learner-tier` appears when the tier rule moved the baseline (escalation or de-escalation), `cold-start` when the baseline resolved alone; focus set contents logged when non-empty
- [ ] 3.6 Tests `internal/recall/difficulty/performance_test.go`: table-driven — ratio tiers (0.95 → escalate from intermediate; 0.95 with baseline hard stays hard; 0.35 → de-escalate from hard), window-boundary inclusion (exactly 5 answered engages rules), cold-start delegation, focus ranking (miss ordering, cap, cross-surface leftover rule, empty-focus when all correct)

## 4. Engine wiring + user-turn biasing (`internal/recall/engine.go`, `internal/engine/server.go`)

- [ ] 4.1 Engine construction: when `cfg.Recall.Difficulty == "adaptive"`, the engine chains `Performance` (with the stats provider) over `Static{value: "adaptive"}`; for concrete configured values the chain remains `Static` resolving verbatim (no learner override for explicitly pinched difficulty)
- [ ] 4.2 `Synthesize` consumes `Resolution.FocusConcepts`: reorders `synth.Concepts` so the focus concepts (matched by concept name) surface first, drops focus names absent from the context, appends a focused lead section to the user turn formatting ("Concepts the developer is currently working through") without altering the concept rows' content
- [ ] 4.3 Thread the lean stats provider through the composition root: `Engine.New` (or the engine package's constructor graph) gains the stats provider injection; server construction unchanged externally
- [ ] 4.4 Queued-event write: engine passes the resolved difficulty + fired signals into the `SaveQuestion` path (task 2.3's payload contract) atomically with question persistence
- [ ] 4.5 Tests `cmd/torec/recall_test.go`: user-turn reordering with focus set (focused concept leads; unknown focus name ignored); static-difficulty config unaffected by the widened contract
- [ ] 4.6 Integration tests `cmd/torec/integration_test.go`: daemon with seeded answered history — high performer (≥92% recent) escalates the difficulty directive in the captured synthesis prompt; struggling learner de-escalates; fresh install (no history) resolves identically to the commit-signal baseline; stored `'queued'` event payload carries the resolved difficulty JSON

## 5. Config sentence + validation check

- [ ] 5.1 Verify `ValidDifficulties` unchanged, `adaptive` continues to default; update the loader template's difficulty comment to describe the learner-performance layering (commit shape = cold start baseline, learner history moves the tier)
- [ ] 5.2 Tests `cmd/torec/config_test.go`: difficulty `adaptive` still passes load and no new warning; `easy`/`intermediate`/`hard` untouched by the resolver-chain change

## 6. Observability + spikes

- [ ] 6.1 `scripts/e2e/adaptive-ab.sh`/`.ps1`: extend or add a companion harness arm that seeds answered history (correct-heavy and miss-heavy branches) via the daemon/HTTP capture flow and shows the tier-shifted log lines and focus-driven captures
- [ ] 6.2 `AGENTS.md` difficulty-resolution row: note the `Resolution` return type and the `Performance` wrapper over the heuristics table
- [ ] 6.3 `docs/CORE/DATA_ANALYSIS.md` §4 note: extend the implemented-mechanism paragraph to record the learner-driven composition and the demotion of commit-shape signals to second class
- [ ] 6.4 Log-line unit assertions exist covering: `cold-start`, `learner-tier` (escalate and de-escalate), and the baseline (no history) cases

## 7. Final verification

- [ ] 7.1 `go build ./...`
- [ ] 7.2 `go vet ./...`
- [ ] 7.3 `go test ./...`
- [ ] 7.4 `golangci-lint run` (if installed)
- [ ] 7.5 Manual: run the spike harness with ≥20 real answered history — verify the resolver emits `learner-tier` log lines and focus concepts land in the captures; adjust constants only with spike evidence
