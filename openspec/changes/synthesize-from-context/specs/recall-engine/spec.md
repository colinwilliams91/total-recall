## MODIFIED Requirements

### Requirement: Recall engine accepts an enriched `SynthesisContext` and synthesizes a single question per hook event
`Engine.Synthesize(ctx context.Context, repo, branch, difficulty, model string, synth SynthesisContext) (*Question, error)` SHALL produce at most one `Question` per invocation derived from `synth.Concepts` (a `[]cache.ConceptRow`) for the triggering `repo` AND `branch`. The prior behavior of pulling bare concept names via `store.Recent(ctx, repo, branch, 20)` and stripping them to `[]string` at engine.go:56-59 is removed — `Synthesize` no longer calls `store.Recent` directly; the caller (`runPipeline`) builds and passes `SynthesisContext`. If `len(synth.Concepts) == 0`, `Synthesize` returns `nil, nil` without calling the provider. Both `repo` and `branch` MUST be non-empty; if either is empty, `Synthesize` returns `nil, nil` without touching the provider. The `difficulty` and `model` parameters are unchanged — Change B will reshape `difficulty` into a `Resolver`.

#### Scenario: Enriched concepts available in SynthesisContext
- **WHEN** `Synthesize` is called with `synth.Concepts` containing 3 `ConceptRow` entries for `repo = "/path/X"` and `branch = "feature-X"`
- **THEN** it calls the provider with a system turn composed from the loaded policy doc + format contract and a user turn listing each concept with weight/source/seen-at, and returns a `*Question`

#### Scenario: Empty SynthesisContext concepts refuses to synthesize
- **WHEN** `Synthesize` is called with `len(synth.Concepts) == 0` (e.g., first-ever commit on the branch produced no cached concepts)
- **THEN** `Synthesize` returns `nil, nil` without making an AI call

#### Scenario: Empty repo or branch refuses to synthesize
- **WHEN** `Synthesize` is called with `repo = ""` or `branch = ""` (e.g., detached HEAD scenario upstream)
- **THEN** `Synthesize` returns `nil, nil` without calling the provider; the pipeline skips silently

---

### Requirement: Synthesis system turn is composed from a loaded policy asset plus format contract rules
The system turn passed to `ai.CompletionRequest.System` SHALL be composed at `SynthesisRequest` time from: (1) the `Body` of the `PromptAsset` named `question-generation-policy` loaded by `internal/assets.Load` at `Engine.New` construction; (2) a trailing `## Format contract` section that preserves the existing JSON shape directive, the `choices[0]` is the correct answer contract, the plausible-distractor rule, and the "exercise the concept in the form illustrated by the snippet without naming specific functions/repos" rule. When the policy asset `Body` is empty (loaded `Source == "fallback"`), the system turn SHALL fall back to the legacy `synthesisSystemTmpl` content verbatim — synthesis never blocks on a missing policy doc. The `RecallConfig.Difficulty` value SHALL be injected into the format contract section. The AI JSON contract (`{"question":"...","choices":["correct","wrong 1","wrong 2","wrong 3"]}`) SHALL be unchanged.

#### Scenario: Policy doc loaded from embedded default
- **WHEN** `Engine.New` is constructed and `$TR_HOME` is unset
- **THEN** the policy asset `Source == "embedded"`; subsequent `Synthesize` calls compose a system turn containing known phrases from `assets/prompts/question-generation-policy.md` (e.g., "counterfactual debugging") followed by the format-contract section containing the JSON contract and `choices[0]` rule

#### Scenario: Policy doc overridden at `$TR_HOME`
- **WHEN** `$TR_HOME/prompts/question-generation-policy.md` exists and contains custom body text
- **THEN** the policy asset `Source == "$TR_HOME"`; `Synthesize` composes a system turn containing the custom body text followed by the format-contract section — the embedded default is not loaded into the system turn

#### Scenario: Policy doc missing or malformed — fallback
- **WHEN** `$TR_HOME` is set but the override file is absent or unparseable as markdown, and the embedded default is also unavailable (corrupt build)
- **THEN** the asset `Source == "fallback"`; `Synthesize` falls back to the legacy `synthesisSystemTmpl` content verbatim; the pipeline logs `[assets] prompt asset "question-generation-policy" not found, falling back to inline synthesis template`; synthesis produces a valid question per the prior behavior

#### Scenario: Difficulty injected into the format contract section
- **WHEN** `RecallConfig.Difficulty` is `"hard"`
- **THEN** the format contract section of the system turn includes the difficulty value as before (preserves the existing `Difficulty level: %s` interpolation in the fallback path and the equivalent directive in the composed path)

#### Scenario: AI JSON contract preserved across both composition paths
- **WHEN** the system turn is composed from either the policy doc body or the fallback template
- **THEN** the system turn contains the directive `Return ONLY a JSON object with no surrounding text: {"question":"...","choices":["correct","wrong 1","wrong 2","wrong 3"]}` (the exact wording is allowed to evolve, but the JSON shape and `choices[0]` is correct contract MUST be present)

---

### Requirement: Synthesis user turn carries enriched concept rows, commit message, and diff snippet
The user turn passed to `ai.CompletionRequest.UserTurn` SHALL be composed from the `SynthesisContext`: a "Concepts the developer has been working with" block listing each `cache.ConceptRow` with its `Concept`, `Weight`, `Source`, and `SeenAt` (one bullet per concept); followed by a "Recent commit context" section containing the `CommitMsg` and `DiffSnippet` fields, formatted as the commit message followed by a fenced code block containing the snippet. When `CommitMsg` is empty or `DiffSnippet` is empty, the "Recent commit context" section SHALL be omitted entirely. The `DiffSnippet` passed to `SynthesisRequest` SHALL already be truncated to ≤500 chars (the truncation happens in `runPipeline`, not `SynthesisRequest`).

#### Scenario: All fields populated
- **WHEN** `SynthesisContext` carries 3 concept rows with weights 0.9/0.7/0.5, a non-empty commit message "fix: handle race in cache.Save", and a 200-char diff snippet
- **THEN** the user turn contains 3 bulleted concept entries (`- <concept> (weight=0.9, source=code, seen=<RFC3339 timestamp>)` etc.) and a "Recent commit context" block containing the commit message line and a fenced ```\n<snippet>\n``` block

#### Scenario: Empty commit context omits the section
- **WHEN** `SynthesisContext.CommitMsg == ""` and `DiffSnippet == ""`
- **THEN** the user turn lists the concept rows only and contains no "Recent commit context" header or fence block

#### Scenario: Pre-truncated diff snippet is passed through
- **WHEN** `runPipeline` truncates `payload.Diff` to 500 chars with a `[… truncated …]` marker before passing it as `DiffSnippet`
- **THEN** `SynthesisRequest` emits the snippet verbatim — `SynthesisRequest` does not re-truncate or modify the snippet

---

### Requirement: `GenerateFeedback` signature and behavior are unchanged by this change
The refactor (`refactor-question-storage-schema`) reshapes `GenerateFeedback` from `(ctx, question, choices []string, correctIndex, answerIndex int, model)` to `(ctx, question, choices []Choice, selectedIndex, correctIndex int, model)`. This change SHALL NOT further reshape `GenerateFeedback` — it MUST consume the refactor's signature unchanged. The feedback system prompt SHALL remain the legacy `feedbackSystemTmpl` content (3-sentence prose, no markdown); a future change may swap it for a loaded feedback-policy asset, but this change owns only the *synthesis* policy asset.

#### Scenario: Feedback generation signature unchanged
- **WHEN** `GenerateFeedback` is called with the refactor's `[]Choice` signature
- **THEN** the existing behavior and feedback system prompt are preserved verbatim by this change; no new asset is loaded for feedback

---

## ADDED Requirements

### Requirement: Prompt-asset loading surfaces the policy doc to the Engine at construction time
`assets.Load(name string) (PromptAsset, error)` SHALL load markdown prompt assets at process init. The canonical asset (`assets/prompts/question-generation-policy.md`) is embedded via `//go:embed`. A same-named file at `$TR_HOME/prompts/<name>.md` overrides the embedded default when `$TR_HOME` is set and the file exists. The returned `PromptAsset` carries `Name`, `Description` (parsed from YAML front matter), `Body` (post-front-matter markdown), and `Source` (`"embedded"` | `"$TR_HOME"` | `"fallback"`). Loaded assets are cached via a package-level mutex-guarded map so per-`Synthesize` calls do not re-read disk or re-parse markdown. When both the override and embedded default are unavailable, `Load` returns `Source == "fallback"` and an empty `Body`; consumers fall back to the legacy `synthesisSystemTmpl`.

#### Scenario: Embedded default loads when `$TR_HOME` is unset
- **WHEN** `assets.Load("question-generation-policy")` is called and `$TR_HOME` is unset
- **THEN** the returned `PromptAsset.Source == "embedded"`, `Body` contains the markdown body of `assets/prompts/question-generation-policy.md`, and the front-matter `name`/`description` keys are extracted into `Name`/`Description`

#### Scenario: Override at `$TR_HOME` wins
- **WHEN** `$TR_HOME` is set and `$TR_HOME/prompts/question-generation-policy.md` exists
- **THEN** the returned `PromptAsset.Source == "$TR_HOME"` and `Body` matches the override file content; the embedded default is not read

#### Scenario: Override malformed but embedded default available
- **WHEN** `$TR_HOME` is set, the override file exists but has malformed front matter, and the embedded default is available
- **THEN** the returned `PromptAsset.Source == "$TR_HOME"` with empty `Description` (front-matter parse is best-effort) and `Body` equal to the full override file content (treated as a body with no front matter)

#### Scenario: Both override and embedded default unavailable — fallback
- **WHEN** `$TR_HOME` is set but the override is missing AND the embedded default is empty (corrupt build scenario)
- **THEN** `Load` returns `PromptAsset{Source: "fallback", Body: ""}` and the pipeline logs `[assets] prompt asset "question-generation-policy" not found, falling back to inline synthesis template`; `SynthesisRequest` falls back to the legacy `synthesisSystemTmpl`

#### Scenario: First load pays the parse cost; subsequent calls are cached
- **WHEN** `assets.Load` is called 100 times in a row
- **THEN** the disk read and markdown parse happen exactly once; the subsequent 99 calls return the cached `PromptAsset` value