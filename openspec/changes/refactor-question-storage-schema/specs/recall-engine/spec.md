## MODIFIED Requirements

### Requirement: Question struct carries choices as typed rows with an IsCorrect flag
The prior `recall.Question` (`Question string`, `Choices []string`, `CorrectIndex int`) is reshaped. `recall.Question` SHALL carry `Question string`, `Choices []Choice` where `Choice` is `{Text string; IsCorrect bool}`, and `CorrectIndex int` (derived: the position of the `Choice` with `IsCorrect = true` within `Choices`, computed during the shuffle in `Synthesize` — the source of truth is the `IsCorrect` boolean on the row, not the integer). The derived `CorrectIndex` is carried through the lifecycle for client-side convenience (e.g., TUI highlighting, MCP response) but the engine's correctness key is the row's `IsCorrect` property, persisted as `choices.is_correct` at the storage layer. This eliminates the prior design's positional reference into a JSON array — reordering or truncating `Choices` cannot silently corrupt grading because the `IsCorrect` boolean travels with its row.

#### Scenario: Synthesize produces a Question with a typed choices slice
- **WHEN** `Synthesize` returns a `*Question` for a multiple-choice prompt
- **THEN** `q.Choices` is a `[]Choice` with at least 2 entries; exactly one entry has `IsCorrect = true`; `q.CorrectIndex` is the slice index of that entry

#### Scenario: CorrectIndex is derived from IsCorrect, not stored independently
- **WHEN** `Choices` is reordered after `Synthesize` (e.g., by an external caller)
- **THEN** `CorrectIndex` no longer points at the correct entry because it was computed at shuffle time — the correct entry remains discoverable by scanning for `IsCorrect = true`, which is the authoritative source. The recommendation is that callers do not reorder `Choices` after `Synthesize` returns

---

### Requirement: Synthesis prompt produces multiple-choice JSON output
The system prompt in `SynthesisRequest` SHALL instruct the AI to return JSON `{"question":"...","choices":["...","...","..."]}` with the AI contract that `choices[0]` is the correct answer. During `Synthesize`, the parsed `choices` slice is wrapped into a `[]Choice` with `IsCorrect = true` on index 0 and `IsCorrect = false` on all others, then shuffled (presentation logic — random display order) producing the final `Position` ordering. The shuffle's `correctIdx == i` index arithmetic from the prior implementation is replaced by the `IsCorrect` boolean traveling with its row — the shuffle mutates the slice order but never has to track "where did index 0 go." The difficulty level from `RecallConfig.Difficulty` SHALL be injected into the system prompt to calibrate question complexity.

#### Scenario: Valid synthesis response shape
- **WHEN** the provider returns `{"question":"Why is jitter added to retry intervals?","choices":["Prevent retry synchronization","Reduce memory usage","Improve cache locality"]}`
- **THEN** `Synthesize` unmarshals it, wraps index 0 as the correct `Choice`, shuffles producing the final `[]Choice`, and returns a `*Question` with `Choices` populated and `CorrectIndex` set to the post-shuffle position of the `IsCorrect = true` entry

#### Scenario: Difficulty injected into prompt
- **WHEN** `RecallConfig.Difficulty` is `"hard"`
- **THEN** the synthesis system prompt includes language instructing the AI to generate an architecture-level or tradeoff-focused question

#### Scenario: Shuffle keeps IsCorrect attached to its text
- **WHEN** the shuffle permutes a `[]Choice` of length 4
- **THEN** the entry with `IsCorrect = true` retains its `IsCorrect` flag and its `Text` after the shuffle; only its slice position changes; no separate "track where did index 0 go" arithmetic is needed

---

### Requirement: GenerateFeedback produces a post-answer explanation
`(*Engine).GenerateFeedback(ctx, question string, choices []Choice, selectedIndex, correctIndex int, model string)` SHALL call `FeedbackRequest(...)` to build the prompt, then call `e.provider.Complete(ctx, req)`. It SHALL return the raw response string (plain prose, not JSON). The signature reshapes from `(question string, choices []string, correctIndex, answerIndex int, ...)` to take `choices []Choice` (typed rows with `IsCorrect`) plus the two positional indices for prompt annotation; the indices are presentation aids, the `IsCorrect` flag is the source of truth. No `repo` or `branch` parameter is needed because feedback generation is unrelated to cache retrieval — the question content is already in memory.

#### Scenario: Successful feedback generation
- **WHEN** `GenerateFeedback` is called with a question, `choices []Choice`, `selectedIndex`, `correctIndex`, and a model
- **THEN** it calls `FeedbackRequest` to build the prompt (annotating choices using the typed `Choice` rows), calls the provider, and returns the prose explanation

#### Scenario: Feedback AI call failure - degraded, not fatal
- **WHEN** the AI provider returns an error (timeout, bad key, rate limit)
- **THEN** `GenerateFeedback` logs `[recall] feedback AI call failed: <err>` and returns `"", nil`
- **AND** the caller continues with empty feedback rather than failing the answer record

---

### Requirement: FeedbackRequest builds the feedback prompt with choice annotations
`FeedbackRequest(question string, choices []Choice, selectedIndex, correctIndex int, model string) ai.CompletionRequest` SHALL build a `CompletionRequest` with a fixed system prompt and a user turn that lists all choices with annotations. The system prompt SHALL instruct: direct, plain prose, no markdown, max 3 sentences. For correct answers: briefly confirm and explain why right. For incorrect answers: name the correct answer explicitly, explain why it is right, briefly note why the chosen answer doesn't fit, do not apologize. The `Choice` rows carry `IsCorrect` directly (no index arithmetic to look up which is correct), simplifying the annotation logic.

#### Scenario: Correct answer - user turn annotations
- **WHEN** `FeedbackRequest` is built for a correct answer (`selectedIndex == correctIndex`)
- **THEN** the user turn lists all choices with ` correct, chosen` annotating the correct choice (the `Choice` with `IsCorrect = true`)
- **AND** ends with `"The developer answered correctly."`

#### Scenario: Incorrect answer - user turn annotations
- **WHEN** `FeedbackRequest` is built for an incorrect answer
- **THEN** the user turn annotates the correct choice (the `Choice` with `IsCorrect = true`) with ` correct` and the chosen choice with ` chosen (incorrect)`
- **AND** ends with `"The developer chose option N and was incorrect."`

## REMOVED Requirements

### Requirement: Question struct carries CorrectIndex through the lifecycle
**Reason**: `CorrectIndex` was the source of truth for the correctness key, persisted as a positional integer into a JSON array. The refactor splits the source of truth (`Choice.IsCorrect` boolean, persisted to `choices.is_correct` DB column) from the presentation aid (`CorrectIndex`, derived from the `IsCorrect` boolean's position after the shuffle). The integer is no longer authoritative — it's a derived value computed once at shuffle time and preserved for client convenience, never independently updatable at the storage layer.
**Migration**: See MODIFIED `Question struct carries choices as typed rows with an IsCorrect flag` above. Existing consumers continue to read a `CorrectIndex int` field but its semantics shift from "stored fact" to "derived at synthesis." No API surface breaks for the integer itself; the underlying storage mutates.