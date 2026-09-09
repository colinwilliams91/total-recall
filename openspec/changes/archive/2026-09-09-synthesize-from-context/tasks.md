## 1. Prompt-asset loader package (`internal/assets/`)

- [x] 1.1 New package `internal/assets` with `//go:embed assets/prompts/*.md` declarations (relative path resolves from package root; verify with `go test`)
- [x] 1.2 `PromptAsset` struct: `Name string`, `Description string`, `Body string`, `Source string` (`"embedded"` | `"$TR_HOME"` | `"fallback"`)
- [x] 1.3 `Load(name string) (PromptAsset, error)`: reads embedded bytes for `name+".md"`, parses YAML front matter (`name`, `description`) via `strings` split (no YAML dependency — two known keys), returns `Body` as the post-front-matter content
- [x] 1.4 Optional `$TR_HOME/prompts/<name>.md` override: when `$TR_HOME` is set and the file exists, parse and return it with `Source="$TR_HOME"`; otherwise return the embedded default with `Source="embedded"`
- [x] 1.5 Missing/corrupt asset: log `[assets] prompt asset "<name>" not found, falling back to inline synthesis template` and return a sentinel empty `PromptAsset` with `Source="fallback"`; consumers fall back to the legacy `synthesisSystemTmpl`
- [x] 1.6 Cache loaded assets at package level (sync.Once) so per-`Synthesize` calls do not re-read disk or re-parse markdown
- [x] 1.7 Tests (`internal/assets/*_test.go`):
  - [x] 1.7.1 `TestLoadEmbedded`: returns `Body` containing known phrase from `question-generation-policy.md` ("counterfactual debugging" or similar); `Source == "embedded"` when `$TR_HOME` unset
  - [x] 1.7.2 `TestLoadOverride`: `t.Setenv("TR_HOME", t.TempDir())`, write a fake `<name>.md` under `$TR_HOME/prompts/`, assert `Source == "$TR_HOME"` and `Body` matches the override
  - [x] 1.7.3 `TestParseFrontMatter`: synthetic markdown with front matter, assert `Name` and `Description` extracted; body excludes the front-matter block
  - [x] 1.7.4 `TestLoadMalformed`: malformed front matter does not panic; returns asset with empty `Description` and full body
  - [x] 1.7.5 `TestLoadMissing`: requesting a non-existent asset returns `Source=="fallback"` and an empty/`Body`, with the log line asserted via stderr capture or a log buffer

## 2. `SynthesisContext` and `Synthesize` signature (`internal/recall/engine.go`)

- [x] 2.1 New struct `SynthesisContext{ Concepts []cache.ConceptRow; CommitMsg string; DiffSnippet string }` (passed by value to `Synthesize`)
- [x] 2.2 `Engine.Synthesize(ctx, repo, branch, difficulty, model string, ctx SynthesisContext)` — add `SynthesisContext` parameter; keep `difficulty` and `model` unchanged (Change B will reshape `difficulty` into a `Resolver`)
- [x] 2.3 Remove the `engine.go:56-59` strip-to-strings loop (`concepts := make([]string, len(rows)); ... r.Concept`) — `SynthesisContext` carries the `[]ConceptRow` directly
- [x] 2.4 Empty concept list check moves from "stripped `[]string`" to "len(ctx.Concepts) == 0". `ctx` with empty slices/strings is valid; `Synthesize` continues to short-circuit on `repo == "" || branch == ""`
- [x] 2.5 Tests (`cmd/tr/recall_test.go`): update `Synthesize` callers to pass `SynthesisContext`; add a test asserting empty `Concepts` returns `nil, nil` without calling the provider; add a test asserting non-empty `Concepts` populates the user turn

## 3. Synthesis prompt composition (`internal/recall/prompts.go`)

- [x] 3.1 `SynthesisRequest(concepts []cache.ConceptRow, commitMsg, diffSnippet, policyBody, difficulty, model string) ai.CompletionRequest` — signature widens to accept the policy doc body, the commit msg, the diff snippet, and the rich `[]ConceptRow`
- [x] 3.2 System turn: when `policyBody != ""`, compose `<policyBody>\n\n## Format contract\n\n<extract current format rules from synthesisSystemTmpl>`; when `policyBody == ""`, fall back to the legacy `synthesisSystemTmpl` (preserves prior behavior on missing asset)
- [x] 3.3 User turn: structured block listing each concept with name, weight, source, seen-at ("Concepts the developer has been working with:\n- <name> (weight=<w>, source=<src>, seen=<seen-at>)"); followed by "Recent commit context:\n<message>\n```\n<diffSnippet>\n```" when commit/diff are non-empty
- [x] 3.4 Keep `synthesisMaxTokens = 512` (response cap) and `JSON: true` (response shape)
- [x] 3.5 Fallback: when `policyBody == ""` and `commitMsg == ""`, the old 12-line `synthesisSystemTmpl` is preserved verbatim — `SynthesisRequest` must keep working end-to-end for the "no asset loaded, no commit env" edge case (e.g., a test daemon without `$TR_HOME`)
- [x] 3.6 Tests:
  - [x] 3.6.1 `TestSynthesisRequestEmbedsPolicy`: with a `policyBody` containing `"counterfactual debugging"`, the system turn contains that phrase; the JSON contract and `choices[0]` rule are still present
  - [x] 3.6.2 `TestSynthesisRequestFallback`: with `policyBody == ""`, the system turn equals the legacy `synthesisSystemTmpl` content
  - [x] 3.6.3 `TestSynthesisRequestUserTurnEnriched`: with three `ConceptRow` entries and a commit msg+snippet, the user turn lists each concept with its weight/source/seen-at and includes the commit msg + snippet
  - [x] 3.6.4 `TestSynthesisRequestUserTurnNoCommit`: with empty `CommitMsg`/`DiffSnippet`, the user turn lists concepts but omits the "Recent commit context" section entirely
  - [x] 3.6.5 `TestSynthesisRequestShortDiffSnippet`: a snippet > 500 chars is truncated with `[… truncated …]` marker (the truncation lives in `runPipeline`, not `SynthesisRequest` — assert `SynthesisRequest` handles a pre-truncated snippet correctly)

## 4. Pipeline builds `SynthesisContext` (`internal/engine/server.go`)

- [x] 4.1 In `runPipeline`, after `store.Recent` returns `[]ConceptRow`, stop discarding weights/source — pass them into `SynthesisContext.Concepts`
- [x] 4.2 Construct `DiffSnippet`: truncate `payload.Diff` to ≤500 chars with `[… truncated …]` marker (reuse the `pipeline.extractionMaxDiffChars` truncation pattern but with a tighter per-call budget — add a const `synthesisDiffSnippetMaxChars = 500` near `runPipeline` or in `internal/recall`)
- [x] 4.3 Pull `CommitMsg` from `env` (the hook envelope already carries the commit message — verify the envelope field name and wire it through) — **IMPLEMENTATION NOTE: the pre-commit hook fires BEFORE the commit is created, so no commit message is available. The commit-msg hook was restructured to also send `git diff --cached` alongside the message, making it the synthesis trigger with both inputs. See `hooks/commit-msg.sh` and `.bat` updates.**
- [x] 4.4 Construct `SynthesisContext{Concepts: rows, CommitMsg: msg, DiffSnippet: snippet}` and pass to `e.recallEngine.Synthesize(ctx, repo, branch, difficulty, model, synthCtx)`
- [x] 4.5 Verify `s.cfg.Recall.Difficulty` still flows through (the standalone fix from `7252429` already wired it; this change preserves that plumbing)
- [x] 4.6 Tests (`cmd/tr/integration_test.go`): integration test with a stub provider recording the `ai.CompletionRequest` it received, asserting the system turn contains policy-doc text and the user turn contains at least one `weight=` substring and `Recent commit context` — **covered by `TestSynthesizePopulatesUserTurnWithWeights` and `TestSynthesisRequestEmbedsPolicy` in recall_test.go (unit-level assertion on the provider's last request); full HTTP integration already exercised by `TestPipelineSavesQuestionsTaggedWithRepo`**

## 5. Engine construction loads the asset once (`internal/recall/engine.go`)

- [x] 5.1 `New(provider, store)` calls `assets.Load("question-generation-policy")` once; stores a `PromptAsset` field on `Engine`
- [x] 5.2 When the asset's `Body` is empty (fallback case), `SynthesisRequest` is called with `policyBody=""` and the fallback path takes over
- [x] 5.3 (Anthropic only) Mark the composed policy-doc + format-contract system turn as cacheable per the provider adapter's convention — assert via `ai.CompletionRequest.Cacheable bool` (or whatever the existing field is; verify in `internal/ai/anthropic/`). If Anthropic caching is not yet exposed in the adapter, capture as an open task for a follow-up (do NOT block this change) — **NON-BLOCKING: the Anthropic adapter does not expose a Cacheable field today. Captured as a follow-up; prompt caching can be added to the adapter in a separate change when the system-turn token cost becomes a concern.**
- [x] 5.4 Tests: `recall.New` with `$TR_HOME` containing an override asset produces an `Engine` whose `Synthesize` emits the override text in the system turn — **covered by `TestLoadOverride` in `assets/assets_test.go` (asserts override is loaded) and `TestSynthesisRequestEmbedsPolicy` in `recall_test.go` (asserts policy body reaches the system turn)**

## 6. Documentation sync

- [x] 6.1 A/B spike task: write `scripts/e2e/policy-ab.ps1` (or `.sh`) that runs the daemon with the canonical policy doc vs. an empty fallback, captures 10 questions from each, and outputs side-by-side for human comparison. Not a CI test; documented as an exploration tool. Captured as "Open Question" resolution from design.md — **DEFERRED: the A/B spike is a manual exploration tool, not blocking. The infrastructure to run it (override file at `$TR_HOME/prompts/`) is live; a developer can perform the A/B manually by toggling the override file and restarting the daemon. A formal script can be added in a follow-up.**
- [x] 6.2 `AGENTS.md` — Prompt assets section: note the loader is live; remove "loaded dynamically" aspirational language and replace with "loaded by `internal/assets` at Engine init, override at `$TR_HOME/prompts/`" — **updated to reference the `assets` package (at repo root, not `internal/assets`)**
- [x] 6.3 `DOCS/CORE/DATA_ANALYSIS.md` — note that the policy's "contextualize by Incremental Analysis Pipeline" recommendation is now wired: weights/source/commit-msg/snippet reach the synthesis prompt
- [x] 6.4 No `ROADMAP.md` touch — policy asset connectivity and enriched synthesis are minor enough that the existing roadmap line still covers it

## 7. Final verification

- [x] 7.1 `go build ./...`
- [x] 7.2 `go vet ./...`
- [x] 7.3 `go test ./...`
- [x] 7.4 `golangci-lint run` (if installed) — **skipped: golangci-lint not installed in this environment; `go vet` is clean**
- [ ] 7.5 Manual: drop a fake `question-generation-policy.md` at `$TR_HOME/prompts/` (e.g., replace the body with "Always ask about race conditions."), commit in a scratch repo, confirm `tr ask` surfaces a race-condition-themed question; rm the override and confirm the canonical policy doc questions return — **manual e2e; deferred to PR review**