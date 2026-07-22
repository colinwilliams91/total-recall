## MODIFIED Requirements

### Requirement: Recall engine synthesizes a single question per hook event
`Engine.Synthesize(ctx, repo, branch, difficulty, model)` SHALL produce at most one `Question` per invocation. The question is derived from recent concepts in the cache for the triggering repo AND branch. If no concepts are available for that repo and branch combination, `Synthesize` returns `nil, nil` without calling the provider. Both `repo` and `branch` MUST be non-empty; if either is empty, `Synthesize` returns `nil, nil` without calling the store or provider.

#### Scenario: Concepts available in cache for the repo and branch
- **WHEN** the cache contains recent concepts for `repo = "/path/X"` and `branch = "feature-X"`
- **THEN** `Synthesize` calls the provider with those concepts and returns a `*Question` with a non-empty `Question` and at least one entry in `Choices`

#### Scenario: No concepts in cache for the repo and branch
- **WHEN** the cache contains no concepts for `repo = "/path/X"` and `branch = "feature-X"` (e.g. first-ever commit on the branch)
- **THEN** `Synthesize` returns `nil, nil` without making an AI call

#### Scenario: Empty repo or branch refuses to synthesize
- **WHEN** `Synthesize` is called with `repo = ""` or `branch = ""` (e.g. detached HEAD scenario upstream)
- **THEN** `Synthesize` returns `nil, nil` without calling `store.Recent` or the AI provider; the pipeline skips silently

---

### Requirement: Synthesis prompt produces multiple-choice JSON output
The system prompt in `SynthesisRequest` SHALL instruct the AI to return JSON `{"question":"...","choices":["...","...","..."]}` with exactly 3 answer choices. The difficulty level from `RecallConfig.Difficulty` SHALL be injected into the system prompt to calibrate question complexity.

#### Scenario: Valid synthesis response shape
- **WHEN** the provider returns `{"question":"Why is jitter added to retry intervals?","choices":["Prevent retry synchronization","Reduce memory usage","Improve cache locality"]}`
- **THEN** `Synthesize` unmarshals it into `Question{Question: "...", Choices: [...]}`

#### Scenario: Difficulty injected into prompt
- **WHEN** `RecallConfig.Difficulty` is `"hard"`
- **THEN** the synthesis system prompt includes language instructing the AI to generate an architecture-level or tradeoff-focused question

---

### Requirement: Synthesis failures degrade gracefully
If the AI call fails or the response cannot be parsed as a `Question`, `Synthesize` SHALL return `nil, nil` and log the failure. No error is propagated to the caller; the pipeline continues.

#### Scenario: Provider timeout during synthesis
- **WHEN** the synthesis AI call exceeds the context timeout
- **THEN** `Synthesize` logs the timeout and returns `nil, nil`

---

### Requirement: Question struct carries CorrectIndex through the lifecycle
`recall.Question` (`Question string`, `Choices []string`, `CorrectIndex int`) is the output type consumed by the `Dispatcher` and persisted by `SaveQuestion`. `CorrectIndex` is computed during shuffle in `Synthesize` and SHALL be persisted at enqueue time so it is available for server-side evaluation at answer time.

#### Scenario: CorrectIndex persisted at enqueue
- **WHEN** `Synthesize` returns a `*Question` with `CorrectIndex = 2`
- **THEN** `runPipeline` calls `SaveQuestion(ctx, repo, branch, q.Question, q.Choices, q.CorrectIndex)` and the row has `correct_index = 2`

---

### Requirement: GenerateFeedback produces a post-answer explanation
`(*Engine).GenerateFeedback(ctx, question string, choices []string, correctIndex, answerIndex int, model string) (string, error)` SHALL call `FeedbackRequest(...)` to build the prompt, then call `e.provider.Complete(ctx, req)`. It SHALL return the raw response string (plain prose, not JSON). No `repo` or `branch` parameter is needed because feedback generation is unrelated to cache retrieval — the question content is already in memory.

#### Scenario: Successful feedback generation
- **WHEN** `GenerateFeedback` is called with a question, choices, correct index, and answer index
- **THEN** it calls `FeedbackRequest` to build the prompt, calls the provider, and returns the prose explanation

#### Scenario: Feedback AI call failure - degraded, not fatal
- **WHEN** the AI provider returns an error (timeout, bad key, rate limit)
- **THEN** `GenerateFeedback` logs `[recall] feedback AI call failed: <err>` and returns `"", nil`
- **AND** the caller continues with empty feedback rather than failing the answer record

---

### Requirement: FeedbackRequest builds the feedback prompt with choice annotations
`FeedbackRequest(question string, choices []string, correctIndex, answerIndex int, model string) ai.CompletionRequest` SHALL build a `CompletionRequest` with a fixed system prompt and a user turn that lists all choices with annotations. The system prompt SHALL instruct: direct, plain prose, no markdown, max 3 sentences. For correct answers: briefly confirm and explain why right. For incorrect answers: name the correct answer explicitly, explain why it is right, briefly note why the chosen answer doesn't fit, do not apologize.

#### Scenario: Correct answer - user turn annotations
- **WHEN** `FeedbackRequest` is built for a correct answer (`correctIndex == answerIndex`)
- **THEN** the user turn lists all choices with ` correct, chosen` annotating the correct choice
- **AND** ends with `"The developer answered correctly."`

#### Scenario: Incorrect answer - user turn annotations
- **WHEN** `FeedbackRequest` is built for an incorrect answer
- **THEN** the user turn annotates the correct choice with ` correct` and the chosen choice with ` chosen (incorrect)`
- **AND** ends with `"The developer chose option N and was incorrect."`

---

### Requirement: Feedback token budget enforced
`FeedbackRequest` SHALL set `MaxTokens` to `feedbackMaxTokens` (150) and `JSON` to `false` (plain prose, not JSON).

#### Scenario: Token budget on feedback request
- **WHEN** `FeedbackRequest` is built
- **THEN** `MaxTokens` is 150 and `JSON` is false