## Why

The Recall Engine synthesizes questions from a tiny, decontextualized list of concept names. Today `recall.Engine.Synthesize` receives `[]string` — engine.go:56-59 explicitly strips `cache.ConceptRow` down to its `.Concept` field, discarding `.Source`, `.Weight`, and `.SeenAt` that the cache already provides for free. The synthesis system prompt (`prompts.go:14`, 12 hardcoded lines) is a thin format-contract that ignores the 220-line research-backed policy doc at `assets/prompts/question-generation-policy.md`. The result: questions are uniform, formulaic, and disconnected from the contextualized, counterfactual-debugging style the policy doc and `DOCS/CORE/DATA_ANALYSIS.md` both call for as the highest-value learning surface.

The policy doc exists, is referenced in `openspec/config.yaml` ("runtime cognition assets loaded dynamically by the Recall Engine"), and is loaded by exactly zero Go files today (grep-confirmed). It treats the synthesis prompt as a static compile-time asset, which means iterating on question style/content/difficulty requires a recompile — a high-friction loop for what is supposed to be the "fun" tuning UX.

A separate, parallel change (`refactor-question-storage-schema`, PR #26) reshapes `recall.Question` to `Choices []Choice{Text, IsCorrect bool}` and frontloads a `question_type` enum. That refactor keeps the AI JSON contract unchanged (`choices[0]` is correct) and explicitly defers presentation/grading for non-MC question types. This change branches from the post-refactor main and targets the new `[]Choice`-based `Synthesize`, leaving the AI JSON contract and the refactor's wire/schema decisions untouched.

## What Changes

**Prompt-asset loading (new capability `prompt-asset-loading`):**
- New package `assets` (at the repository root, so `//go:embed` path resolution works — Go's embed directive does not permit `..` in paths) holds an `//go:embed` of `assets/prompts/*.md` and a `Load(name)` function that returns the embedded default, overridden at runtime by a same-named file at `$TR_HOME/prompts/<name>` when present. Parsing extracts the YAML front matter (`name`, `description`) from the body so callers can introspect metadata without re-parsing markdown.
- A `PromptAsset` type carries `Name`, `Description`, `Body`, `Source` fields. Callers use `.Body` directly as a system prompt fragment.
- Loading happens once at `Engine` construction (the `recall.New` call), cached on the `Engine`. No per-call file IO on the synthesis hot path.

**Enriched synthesis context (modified capability `recall-engine`):**
- `recall.Synthesize` signature widens to accept a `SynthesisContext` (or `[]cache.ConceptRow` + commit message + a representative diff snippet). The current `[]string` concept list is replaced by the rich row data `store.Recent` already returns. `internal/recall/engine.go:56-59` (the strip-to-strings loop) goes away.
- `SynthesisRequest` system turn becomes: the loaded policy doc (`assets/prompts/question-generation-policy.md` `.Body`), plus the format/contract rules (JSON shape, `choices[0]` is correct, no codebase references — kept from the current template). The fallback when no policy doc loads is the current 12-line template so a missing/corrupt asset file never blocks synthesis.
- `SynthesisRequest` user turn becomes: structured "Concepts the developer has been working with" listing each concept with its weight, source, and last-seen timestamp; plus a "Recent commit context" section carrying the commit message and a short truncated diff snippet. Both pieces flow from the hook envelope (`runPipeline`) and the cache — no schema changes, transient context only.
- `runPipeline` in `internal/engine/server.go` builds the `SynthesisContext` and passes it to `Synthesize`. The commit message comes from `env` (already present from the hook envelope), and the diff snippet is `payload.Diff` truncated to ≤500 chars with a marker. The full 8000-char diff used by extraction is not reused here — only a short, contextualizing hunk.

**Lexical/stylistic guardrails:**
- This change does not touch the AI JSON output contract (`choices[0]` is correct remains), the `Question` struct shape (post-refactor `[]Choice`), the wire contract, the cache schema, or `GenerateFeedback` (the refactor already reshapes its signature, our change feeds it the same `[]Choice` data).

## Capabilities

### New Capabilities
- `prompt-asset-loading`: Loads `assets/prompts/*.md` files as runtime cognition assets via `//go:embed` defaults plus an optional `$TR_HOME/prompts/` override. Extracts YAML front-matter metadata. Caches at Engine construction.

### Modified Capabilities
- `recall-engine`: `Engine.Synthesize` accepts an enriched `SynthesisContext` (concept rows with weight/source/seen-at, commit message, diff snippet) instead of pulling bare concept names from the cache and stripping them to `[]string`. The synthesis system prompt is composed from the loaded policy doc plus format/contract rules; the user turn carries the enriched context. `SynthesisRequest` signature widens accordingly. AI JSON contract unchanged; refactor's `[]Choice` struct unchanged.

## Impact

- **Code:** `assets` (new package at repository root, ~200 lines: `//go:embed` declarations, `Load(name)`, front-matter parser, override resolver); `internal/recall/engine.go` (signature change, replace strip-to-strings loop, accept `SynthesisContext`); `internal/recall/prompts.go` (`SynthesisRequest` reshapes: system turn composed from policy asset + format rules, user turn composes enriched context); `internal/engine/server.go` (`runPipeline` builds `SynthesisContext` from `env` + `payload.Diff` + `store.Recent` result, passes to `Synthesize`); `hooks/commit-msg.sh` and `.bat` (also send `git diff --cached` alongside the message so `runPipeline` has both inputs).
- **Tests:** `cmd/tr/cache_test.go` (no change — store surface untouched); new `assets/assets_test.go` for loader (front-matter parse, override, fallback when asset missing); `cmd/tr/integration_test.go` (existing pipeline integration tests exercise the enriched path end-to-end); `cmd/tr/recall_test.go` (new tests asserting `SynthesisRequest` embeds policy doc, enriches user turn with weights/commit-msg/snippet, and falls back when policy body is empty).
- **APIs:** No external API change. `recall.Synthesize` is an internal-call interface; consumers (`server.go runPipeline`) update atomically with this change.
- **Dependencies:** None new. `//go:embed` is in stdlib; YAML front-matter parsing is a tiny `strings` split (no dependency on a YAML library — the front matter is two known keys, `name` and `description`).
- **Specs:** This change updates `openspec/specs/recall-engine/spec.md` (MODIFIED requirements) and adds `openspec/specs/prompt-asset-loading/spec.md` (new capability).
- **Future alignments:** Adaptive difficulty (`Change B \u2014 adaptive-difficulty`) consumes the same enriched `SynthesisContext` to drive its `DifficultyResolver` — the weights, source mix, and diff size are exactly the signals `DATA_ANALYSIS.md` §4 identifies as "AI-delegation" tells.

## Key Design Decisions

- **`//go:embed` + runtime override, not runtime-read-only.** The shipped binary works out-of-the-box with the canonical policy doc embedded; a developer iterating on question style drops a replacement at `$TR_HOME/prompts/question-generation-policy.md` and restarts the daemon — no recompile needed for the tuning loop. Pure runtime-read would force path-resolution complexity and break the binary when `assets/` is missing (a real failure mode on a deployed binary far from its source tree).

- **Fallback to the current 12-line template when the asset fails to load.** A corrupt or missing policy file never blocks synthesis: the Engine falls back to the existing `synthesisSystemTmpl` content. The policy doc is an enrichment, not a hard dependency.

- **Enriched context is transient, not persisted.** `cache.ConceptRow` already carries `Source`/`Weight`/`SeenAt` — we stop discarding them at engine.go:56-59 and pass them straight through. The commit message and diff snippet are transient: they ride the `SynthesisContext` from `runPipeline` to `Synthesize` and never touch the SQLite store. Avoids entangling with the refactor's schema work.

- **Short diff snippet, not the full extraction diff.** Extraction uses the full 8000-char diff to identify concepts. Synthesis gets a ≤500-char hunk (or the last commit's diff summary) — enough to ask "why is *this* await necessary?" without busting the synthesis token budget (currently `synthesisMaxTokens = 512`).

- **System prompt composition: policy doc + format rules, not policy doc alone.** `question-generation-policy.md` is research/strategy prose (counterfactual debugging, anti-trivia, anti-delegation signals) — it has no JSON contract or `choices[0]` rule. The format/contract rules from the current template stay as a separate composed section so the AI still emits parseable JSON.

- **Branch from post-refactor main.** This change targets the refactor's `[]Choice` `Question` struct. Developing against pre-refactor main would force a rebase of the `SynthesisRequest` shape across the refactor's restructure of `Synthesize` (refactor task 3.4 explicitly retains "the AI contract" — our enrichment is orthogonal). Waiting for the refactor to merge avoids double-touching the same function.

## Non-Goals

- Adaptive difficulty logic — Change B owns `DifficultyResolver`. This change keeps plumbing the `difficulty string` through `Synthesize` unchanged; the enriched context it produces is what Change B's adaptive resolver reads.
- Multi-select or free-text synthesis — the refactor frontloads `question_type` but defers non-MC presentation/grading. Our synthesis still emits MC-1 questions per the refactor's AI contract.
- Persisting commit messages or diff snippets in the cache — schema change entangled with the refactor; transient context suffices for now.
- Per-concept question generation — still one question synthesized from the recent-20 concept pool.
- OpenAI vs Anthropic adapter changes — `ai.Provider` is unchanged; we just send a richer system+user turn.
- Composing multiple policy docs — only `question-generation-policy.md` is loaded; future assets will follow the same loader pattern but are not built speculatively.