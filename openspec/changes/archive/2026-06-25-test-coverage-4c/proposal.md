## Why

Phase 4C ("answer-feedback") shipped with partial test coverage. The `x2-test-automation-rewrite` (PR #10) aligned existing tests with the 4C API changes — updating `AnswerQuestion` call sites, renaming `feedback` to `advisory`, wiring `stateFeedback` into the state machine tests — but did not add coverage for the new functionality itself. The result: 5 fully covered, 4 partially covered, and 26 uncovered 4C behaviors across the cache, ask state machine, integration, golden, and recall packages.

The most glaring gap is `internal/recall`: zero test files. `FeedbackRequest` is pure logic (prompt construction with choice annotations) that's ideal for model isolation, and `GenerateFeedback` has a degradation path (AI failure → empty string, not error) that has never been exercised.

Additionally, `ROADMAP.md` has drift: Phase 4D still says "(Planned)" despite shipping in PR #8, and Phase 4B (VS Code Extension) remains listed as "(Next)" when it should be deferred out of Phase 4 scope to make room for higher-priority work.

## What Changes

- **26 new tests + 4 test extensions** filling the 4C coverage gaps across 5 existing test files and 1 new test file
- **New `recall_test.go`** in `cmd/total-recall/` — tests `FeedbackRequest` prompt construction and `GenerateFeedback` degradation, using a mock `ai.Provider` (new pattern for this codebase)
- **New golden file** `TestGoldenAskFeedbackView.golden` — snapshot for the `stateFeedback` "Evaluating..." view
- **`ROADMAP.md` fixes**: Phase 4D "(Planned)" → "(Shipped)"; Phase 4B moved from "(Next)" to a "(Deferred)" subsection with rationale
- No application code changes — this is a test-only change

## Capabilities

### New Capabilities
- `test-coverage`: Test coverage for Phase 4C answer-feedback behaviors — cache schema operations, ask state machine feedback flow, HTTP/MCP evaluation endpoints, recall package prompt construction and AI degradation, golden file for stateFeedback view

### Modified Capabilities
<!-- No spec-level behavior changes — this is test-only. Existing specs for question-store, recall-engine, question-delivery, mcp-server, and tr-ask remain unchanged. -->

## Impact

- `cmd/total-recall/cache_test.go` — 6 new tests (SaveQuestion correctIndex, GetQuestion, SkipQuestion, AnswerQuestion with feedback, RecentAnswered enriched, migration idempotency)
- `cmd/total-recall/ask_test.go` — 7 new tests (updateFeedback handler, stateFeedback view, postAnswer/postSkip cmd execution, post-alt-screen rendering, q/Esc exit)
- `cmd/total-recall/integration_test.go` — 9 new tests (?feedback=true path, evaluation correctness, error paths, MCP correct_index, MCP recall_answer, recall://recent enriched)
- `cmd/total-recall/golden_test.go` — 1 new test (stateFeedback view snapshot)
- `cmd/total-recall/recall_test.go` — NEW file, 4 tests (FeedbackRequest correct/incorrect/token budget, GenerateFeedback degradation with mock provider)
- `cmd/total-recall/testdata/TestGoldenAskFeedbackView.golden` — NEW golden file
- `ROADMAP.md` — Phase 4D status fix, Phase 4B deferral

## Key Design Decisions

- **Mock provider in `recall_test.go`**: A simple `mockProvider` struct implementing `ai.Provider` with canned `response` and `err` fields. Collocated with the tests that use it — follows Go convention of test helpers in `_test.go` files. Unlocks testing `GenerateFeedback` and `Synthesize` degradation without real AI calls.
- **Tests collocated in `cmd/total-recall/`**: Follows existing convention (AGENTS.md). `recall_test.go` imports `internal/recall` directly, same as `cache_test.go` imports `internal/cache`.
- **No internal package test files**: All tests live in `cmd/total-recall/` even when testing `internal/recall` functions. This matches the established pattern and avoids introducing a new convention.
- **Integration tests use existing `startTestDaemon`**: The 9 new integration tests reuse `startTestDaemon(t)` and `mustPOST`/`mustGET` helpers. MCP tests require direct tool invocation via the MCP SDK or HTTP-level protocol messages — the latter is simpler and avoids SDK test dependencies.
- **Golden file for stateFeedback**: Single static string `"Evaluating..."` — small golden file, but maintains the visual regression safety net for all ask model views.

## Non-Goals

- Phase 4B VS Code extension (deferred out of Phase 4)
- Any new features or application code changes
- Refactoring existing tests (only additive — new tests and extensions)
- Spaced repetition / difficulty progression
- Test coverage for phases 00–03 (already covered by existing tests)
