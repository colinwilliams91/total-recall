## Requirements

### Requirement: Recall engine synthesizes a single question per hook event
`Engine.Synthesize` SHALL produce at most one `Question` per invocation. The question is derived from recent concepts in the cache for the triggering repo. If no concepts are available, `Synthesize` returns `nil, nil` without calling the provider.

#### Scenario: Concepts available in cache
- **WHEN** the cache contains recent concepts for the repo
- **THEN** `Synthesize` calls the provider and returns a `*Question` with a non-empty `Question` and at least one entry in `Choices`

#### Scenario: No concepts in cache
- **WHEN** the cache contains no concepts for the repo (e.g. first-ever commit)
- **THEN** `Synthesize` returns `nil, nil` without making an AI call

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
- **WHEN** the synthesis AI call exceeds the 10-second context timeout
- **THEN** `Synthesize` logs the timeout and returns `nil, nil`

---

### Requirement: Question struct is the shared output type
`recall.Question` (`Question string`, `Choices []string`) is the output type consumed by the `Dispatcher`. It maps directly to `engine.RecallPrompt` on the wire format — `Synthesize` output populates `HookResponse.Recall` when non-nil.

#### Scenario: Question dispatched via Dispatcher
- **WHEN** `Synthesize` returns a non-nil `*Question`
- **THEN** `runPipeline` calls `dispatcher.Dispatch(*question)` and the question appears in the v1 delivery channel (daemon stdout)
