## Context

`assets/prompts/question-generation-policy.md` (220 lines) is the canonical research-backed guidance on what good recall questions look like: counterfactual debugging, anti-trivia, anti-AI-delegation escalation, generation-then-comprehension mechanics. `openspec/config.yaml` declares prompt assets are "runtime cognition assets loaded dynamically by the Recall Engine and they live under `/assets/prompts/`." Grep across all Go code: zero references to the file. The actual synthesis prompt is a 12-line hardcoded template in `internal/recall/prompts.go:14`, which says "do not reference the specific codebase" — the exact opposite of the policy doc's contextualized ("why is *this* await necessary?") guidance.

In parallel, `recall.Engine.Synthesize` receives only concept names. `internal/cache/store.go:53` defines `ConceptRow{ID, Concept, Source, Weight, SeenAt}` and `store.Recent` returns the rich row — but `engine.go:56-59` immediately discards everything except `.Concept` and synthesizes questions from "exponential backoff, jitter for retry synchronization" as bare strings. No weights, no source mix, no signal of when or how recently each concept was seen.

A parallel refactor (`refactor-question-storage-schema`, PR #26) reshapes `recall.Question` from `Choices []string, CorrectIndex int` to `Choices []Choice{Text, IsCorrect bool}` while keeping the AI JSON contract (`choices[0]` is correct). The refactor's `recall-engine/spec.md` mandates "RecallConfig.Difficulty SHALL be injected into the system prompt" — a one-line gap we've already fixed in a standalone commit (`7252429`). This change branches from post-refactor main and stops discarding the rich context the cache already produces, while finally connecting the policy doc to the prompt it was supposed to shape.

## Goals / Non-Goals

**Goals:**
- Load `assets/prompts/*.md` as runtime cognition assets via `//go:embed` defaults plus a `$TR_HOME/prompts/` override so a developer iterating on question style edits markdown and restarts the daemon — no recompile.
- Stop discarding `cache.ConceptRow` data at `engine.go:56-59`; pass `.Source`, `.Weight`, `.SeenAt` straight through into `SynthesisRequest` user turn.
- Compose the synthesis system turn from the loaded policy doc (counterfactual-debugging, anti-trivia, anti-delegation guidance) plus format/contract rules (JSON shape, `choices[0]`=correct, no codebase references).
- Carry short commit context (commit message + ≤500-char diff snippet) into the synthesis user turn so the AI can ask "why is *this* await necessary?" — counterfactual, codebase-anchored questions per the policy doc.
- Preserve the AI JSON contract and the refactor's `[]Choice` `Question` shape unchanged.

**Non-Goals:**
- Adaptive difficulty — Change B owns the `DifficultyResolver`; this change keeps plumbing the `difficulty string` and exposes the signals B will consume.
- Multi-select or free-text synthesis — the refactor's `question_type` enum frontloads the schema dimension, but presentation/grading for non-MC stays deferred per the refactor's Non-Goals.
- Persisting commit messages or diff snippets — transient context only. SQLite schema unchanged.
- Per-concept question generation — still one question from the recent-20 pool.
- A YAML library dependency — front-matter parsing for the two known keys (`name`, `description`) is a tiny `strings` split.

## Decisions

### Decision: Prompt loading — `//go:embed` + `$TR_HOME/prompts/` override

Resolves the "edit-and-recompile vs. true-dynamic-load" tension by combining both: the canonical policy doc lives under `assets/prompts/question-generation-policy.md`, versioned with the code, and `//go:embed` ships it inside every binary. On Engine construction, `Load(name)` first checks `$TR_HOME/prompts/<name>`; if present, that file (raw bytes + parsed front-matter) wins. Otherwise the embedded default is returned. Missing/corrupt file: log and fall back to the embedded default; if even the embedded default is empty (shouldn't happen), `SynthesisRequest` falls back to the current 12-line `synthesisSystemTmpl`.

**Alternatives considered:**
- *Runtime file read only* (`assets/prompts/` always read from disk at init). Rejected: path resolution complexity; a deployed binary far from its source tree breaks when `assets/` is missing. Forces the maintainer to ship prompt files alongside the binary.
- *`//go:embed` only* (no runtime override). Rejected: iteration on question style becomes a recompile loop. The whole point of decoupling the policy from the Go template was to enable markdown-and-restart tuning.
- *Two-tier: `//go:embed` for system, separate runtime-only vars*. Rejected: same effect, more complexity. The override is one path, one file, one resolver.

### Decision: Enriched context is transient, not persisted

`cache.ConceptRow` already carries `.Source`, `.Weight`, `.SeenAt`; `store.Recent` returns them; engine.go:56-59 strips them. This change just removes the strip. The commit message and a short diff snippet (≤500 chars, sourced from `payload.Diff`) ride the new `SynthesisContext` from `runPipeline` → `Synthesize` and never enter the SQLite store. No schema columns added; no entanglement with the parallel refactor's schema work.

**Alternatives considered:**
- *Persist a `commit_msg TEXT` and `diff_snippet TEXT` on `concepts` rows.* Rejected: entangles with the refactor's schema migration; duplicates content already captured (the diff was the source of the concept extraction, the message is in the git history); no current consumer needs the persisted form (a future adaptive resolver can read from `question_events` audit).
- *Re-extract the diff from git at synthesis time.* Rejected: synthesis is async-post-commit, the HEAD has moved; re-extraction races. The hook envelope already has the diff and the commit message in memory.

### Decision: Short diff snippet (≤500 chars), not the full extraction diff

Extraction uses the full `payload.Diff` (up to 8000 chars per `pipeline.extractionMaxDiffChars`). Synthesis gets a ≤500-char snippet — the first hunk, or a representative slice if the diff is large, with a `[… truncated …]` marker. Enough for the AI to ask "why is *this* `await` necessary?" (the policy doc's GOOD PROMPTS archetype), bounded by `synthesisMaxTokens = 512` on the response side.

**Alternatives considered:**
- *Reuse the full extraction diff.* Rejected: extraction consumes 8000 chars for concept identification; carrying 8000 chars into the synthesis prompt eats the entire user-turn token budget and pushes question generation to ≥2k tokens response-side. Question quality does not scale linearly with diff size.
- *Per-concept snippet extraction.* Rejected: requires a second AI pass to select "the most relevant hunk for this concept" — adds latency and complexity for marginal gain. A single representative hunk is enough.

### Decision: System prompt composition — policy doc + format rules, not policy doc alone

`question-generation-policy.md` is research/strategy prose: " disproportionately target debugging, causal reasoning, architectural tradeoffs", "Avoid: syntax trivia, rote API recall", "What invariant is being preserved here?", etc. It contains no JSON contract, no `choices[0]` rule, no `MaxTokens` directive. The format/contract rules from the current `synthesisSystemTmpl` stay as a separate composed section appended after the policy doc body. The composed system prompt is:

```
<policy-doc body>

## Format contract

Difficulty level: <difficulty>

Return ONLY a JSON object with no surrounding text:
{"question":"<question text>","choices":["<correct answer>","<wrong answer 1>","<wrong answer 2>","<wrong answer 3>"]}

Rules:
- The first choice must be the correct answer
- Wrong answers must be plausible but clearly incorrect to someone who understands the concept
- Keep the question concise and directly related to one of the provided concepts
- Do not reference the specific codebase or project — make the question about the concept itself
```

The last rule ("Do not reference the specific codebase") is in tension with the policy doc's "contextualized by the Incremental Analysis Pipeline" guidance — see Open Question below. We resolve it by softening: questions should *exercise* the concept in the *form illustrated* by the diff snippet, not reference the specific function names or repo.

**Alternatives considered:**
- *Replace `synthesisSystemTmpl` with the policy doc body alone.* Rejected: the AI loses the JSON contract and `choices[0]` ordering, breaking the refactor's parse step.
- *Edit the policy doc to embed the JSON contract.* Rejected: the policy doc is research/strategy prose, not a prompt template; coupling them forces edits to the policy doc for every JSON shape iteration.

### Decision: `SynthesisContext` value type (not a pointer struct), passed by value to `Synthesize`

A small struct (`Concepts []cache.ConceptRow`, `CommitMsg string`, `DiffSnippet string`) passed by value keeps the method signature simple and self-describing. No nil-check — the zero value (empty slice, empty strings) is well-defined and `Synthesize` already short-circuits on empty concept lists.

**Alternatives considered:**
- *Pointer to a `SynthesisContext` struct.* Rejected: forces nil-checking at the call site; the field set is small and stable; pass-by-value copies a slice header + two string headers (~48 bytes).
- *Continuing to expand `Synthesize` signature with new args.* Rejected: ergonomic cliff — `Synthesize(ctx, repo, branch, difficulty, model, concepts, commitMsg, diffSnippet ...)`. The struct groups one concept.

## Risks / Trade-offs

- **[Trade-off] Prompt-asset loading adds an init-time file read** on the first `Engine.New` call per process. Mitigated: cached after first load; embedded default means even a missing `$TR_HOME/prompts/` is one md-parse cost, not a disk miss.
- **[Trade-off] Larger synthesis prompt → higher per-call token cost.** Policy doc body (~3k chars) + enriched user turn (~500 chars diff + structured concept block) is ~5x the current prompt. Mitigated: `synthesisMaxTokens = 512` caps the response; the system turn is cached server-side on the provider (Anthropic prompt caching) so the doc body is paid once per ~5-minute TTL, not per call.
- **[Risk] Policy doc text could leak into the AI output** — e.g. the AI mimics the doc's prose style and emits an essay instead of JSON. Mitigated: the format-contract section at the end of the system prompt is unambiguous ("Return ONLY a JSON object") and the JSON parse step at engine.go:69 fails loudly on prose contamination.
- **[Risk] Override file at `$TR_HOME/prompts/` could silently drift from canonical version** — a developer iterating locally leaves a stale override that diverges from the shipped policy. Mitigated: `tr config show` lists prompt-asset resolution (which path each asset loaded from); log a `loaded from override` line on first use. Cross-reference: override drift / silent rot is mitigated by the `prompt-asset-observability` change (mtime-based `OVERRIDE WARNING` at startup plus `tr asset list|reset|sync` recovery commands).
- **[Open] The "do not reference the specific codebase" rule is in tension with the policy doc's contextualized questioning.** We soften to "exercise the concept in the form illustrated by the snippet, without naming the specific functions/repos." If this loosening produces hallucinated questions (the AI invents a function name), we tighten in iteration — but we'd rather start permissive and observe than start strict and miss the contextualization value.

## Migration Plan

1. Branch this change from post-refactor main (after PR #26 merges).
2. Implement `internal/assets` package; existing `assets/prompts/question-generation-policy.md` becomes the `//go:embed` source — no file move.
3. Rewrite `SynthesisRequest` and `Synthesize` per tasks; `runPipeline` builds `SynthesisContext`.
4. Tests assert: policy doc text appears in system turn; weights/commit msg/snippet appear in user turn; AI JSON contract unchanged; refactor's `[]Choice` struct consumed unchanged.
5. No migration script, no teardown — zero schema change, zero wire change. Pure Engine-internal refactor.

## Open Questions

- Does the policy doc's "contextualize by the Incremental Analysis Pipeline" guidance allow per-question diff snippets, or should we only pass the *concept names + weights* and trust the AI to invent counterfactual scenarios itself? Resolving empirically: A/B the two system-prompt variants on a scratch repo and judge question quality. Captured as a task in `tasks.md` (task 6.1).
- Should `internal/assets` live as a stand-alone package (reusable for future prompt assets) or as a sub-package of `internal/recall`? Decided: stand-alone package. `internal/assets` is the loader; `internal/recall` is the consumer. Future prompt assets (e.g., a `feedback-policy.md`) reuse the same loader without taking a dependency on `recall`.
- Anthropic prompt caching — should the system turn (policy doc + format rules) be marked cacheable to reclaim the token cost? Yes when supported; the `ai.Provider` already exposes this in its adapter layer. Captured as task 5.3.