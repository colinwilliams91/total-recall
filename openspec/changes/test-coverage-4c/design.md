## Context

Phase 4C shipped 41 tasks of answer-feedback functionality: schema migration, `GetQuestion`/`SkipQuestion`, `GenerateFeedback`/`FeedbackRequest`, `stateFeedback` blocking state, `?feedback=true` evaluation path, MCP `correct_index` exposure, and `recall://recent` enrichment. The `x2-test-automation-rewrite` (PR #10) aligned existing tests with the new API but did not add coverage for the new behaviors.

Current coverage state (from explore-phase analysis):

```
Coverage Summary
┌──────────────────────────────┬──────┬──────┬──────┐
│ Layer                        │  ✓   │  ◐   │  ✗   │
├──────────────────────────────┼──────┼──────┼──────┤
│ Cache (store.go)             │   0  │   2  │   6  │
│ Ask state machine (ask.go)   │   4  │   0  │   7  │
│ Integration (HTTP + MCP)     │   1  │   2  │   9  │
│ Golden (View snapshots)      │   0  │   0  │   1  │
│ Recall package (engine+prompts) │  0  │   0  │   3  │
├──────────────────────────────┼──────┼──────┼──────┤
│ TOTAL                        │   5  │   4  │  26  │
└──────────────────────────────┴──────┴──────┴──────┘
```

The `internal/recall` package has zero test files. `FeedbackRequest` is pure logic — prompt construction with choice annotations — and `GenerateFeedback` has a degradation path that has never been exercised.

## Goals / Non-Goals

**Goals:**
- Fill all 26 uncovered 4C behaviors with Go-native tests
- Upgrade 4 partially-covered behaviors to full coverage
- Introduce mock `ai.Provider` pattern for testing AI-dependent code without real API calls
- Add golden file for `stateFeedback` view
- Fix ROADMAP drift (4D status, 4B deferral)
- All tests follow the framework documented in AGENTS.md (model isolation, headless integration, golden file)

**Non-Goals:**
- Any application code changes
- Phase 4B VS Code extension
- Refactoring existing tests
- Coverage for phases 00–03 (already covered)
- Spaced repetition or scoring features

## Decisions

### 1. Mock provider pattern — simple struct, no framework

**Decision**: A `mockProvider` struct in `recall_test.go` implementing `ai.Provider` with two fields:

```go
type mockProvider struct {
    response string
    err      error
}

func (m *mockProvider) Complete(ctx context.Context, req ai.CompletionRequest) (string, error) {
    if m.err != nil {
        return "", m.err
    }
    return m.response, nil
}
```

**Rationale**: No mocking framework (testify, gomock, etc.) is in the dependency tree and introducing one would violate the "Go-native, no external test runners" convention from AGENTS.md. A hand-rolled struct with canned responses is sufficient — we only need to test two paths: successful completion and error degradation.

---

### 2. Tests collocated in `cmd/total-recall/` — including `internal/recall`

**Decision**: New `recall_test.go` lives in `cmd/total-recall/` (package `main`), importing `internal/recall` directly.

```
cmd/total-recall/
├── cache_test.go      ← tests internal/cache (existing pattern)
├── recall_test.go     ← tests internal/recall (new, same pattern)
├── ask_test.go        ← tests ask model (existing)
├── integration_test.go← tests engine HTTP (existing)
├── golden_test.go     ← tests ask View() (existing)
└── testdata/          ← golden files
```

**Rationale**: AGENTS.md states "All automated tests are Go-native, collocated in `cmd/total-recall/*_test.go`". `cache_test.go` already tests `internal/cache` from the cmd layer. `recall_test.go` follows the same pattern. Introducing `internal/recall/*_test.go` would break the established convention.

---

### 3. Integration tests — HTTP-level, not MCP SDK

**Decision**: MCP tool tests (`recall_next` returns `correct_index`, `recall_answer` with `answer_index`) use HTTP-level requests against `/mcp/` rather than the MCP Go SDK client.

**Rationale**: The MCP protocol is JSON-over-HTTP. Using `mustPOST` with MCP-formatted JSON bodies keeps tests consistent with the existing integration test style and avoids pulling in MCP SDK test utilities. The existing `TestMCPEndpoint` test already validates the endpoint at the HTTP level.

For `recall://recent` enrichment, the test reads the resource via an MCP `read/resource` JSON-RPC request over HTTP.

---

### 4. Post-alt-screen rendering test — capture stdout

**Decision**: The post-alt-screen rendering test calls the render logic directly (or captures `fmt.Println` output via a helper) rather than running a full `tea.NewProgram` cycle.

**Rationale**: The render logic lives inside `askCmd.RunE` after `p.Run()` returns. Testing it via a full Bubble Tea program run is brittle and slow. Instead, the test sets `feedbackResult`/`skipped`/`advisory` on an `askModel` and calls a render function (or replicates the switch logic) to verify the output. If the render logic isn't extractable, the test captures `os.Stdout` via redirection.

---

### 5. Cache migration test — simulate existing DB

**Decision**: The `addColumnIfMissing` idempotency test creates a DB with the Phase 4A schema (no new columns), then calls `cache.Open()` and verifies the new columns appear.

**Rationale**: This tests the real migration path. `setupCache(t)` creates a fresh DB with the full schema, so the test needs to manually create a DB with the old schema first, then re-open it. Uses `t.Setenv("HOME", ...)` for isolation.

---

### 6. Golden file for stateFeedback — static string

**Decision**: `TestGoldenAskFeedbackView` sets `m.state = stateFeedback` and calls `m.View()`, comparing against `TestGoldenAskFeedbackView.golden`.

**Rationale**: The view is a static string (`"\rEvaluating...   "`), so the golden file will be small. But maintaining the golden file pattern for all ask model views ensures visual regression catches accidental changes to the rendering format.

---

### 7. ROADMAP fixes — status correction and deferral

**Decision**:
- Phase 4D: `### Phase 4D — Extended AI Providers (Planned)` → `### Phase 4D — Extended AI Providers (Shipped)`
- Phase 4B: Move from `### Phase 4B — VS Code Extension (Next)` to a new `### Phase 4B — VS Code Extension (Deferred)` subsection with explanation

**Rationale**: 4D shipped in PR #8 (archived 2026-06-20). 4B is deferred to make room for higher-priority work. The REST API and MCP server are stable delivery surfaces; the VS Code extension is a UX enhancement, not a capability gap.

## Risks / Trade-offs

**[Risk] Mock provider doesn't match real provider behavior** — the `mockProvider` returns a canned string, but real providers (Anthropic, OpenAI) have response format differences (whitespace, JSON wrapping, etc.). Mitigation: the mock is only used for degradation-path testing (error → empty string) and prompt construction testing (verify the request, not the response). Real provider integration is covered by manual e2e.

**[Risk] MCP HTTP-level tests may be brittle** — the MCP protocol uses JSON-RPC with specific envelope shapes. If the MCP SDK changes its HTTP handler behavior, the tests may break even though the application logic is correct. Mitigation: keep MCP tests minimal (tool list, single tool call) and focus assertions on the JSON content, not the envelope structure.

**[Risk] Post-alt-screen rendering test may need refactoring** — if the render logic is inline in `RunE` and not extractable, the test needs stdout redirection which is fragile on Windows. Mitigation: if the logic isn't cleanly testable, extract a `renderResult(am askModel) string` helper (minimal refactor, test-only) — but this is an application code change, so it would need to be scoped as a separate task within this change.

**[Risk] Migration test requires manual schema setup** — creating a DB with the old schema means executing the old `CREATE TABLE` SQL manually in the test. If the old schema drifts from what the migration expects, the test gives false confidence. Mitigation: use the exact SQL from the pre-4C `createQuestionsTableSQL` constant (preserved in git history) and verify the migration adds exactly the 4 new columns.
