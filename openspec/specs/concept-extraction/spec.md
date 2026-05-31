## Requirements

### Requirement: Concept extraction runs asynchronously, never blocking the hook response
`ExtractConcepts` SHALL be called inside a background goroutine spawned after the hook's 202 Accepted response is written. The committing terminal is never blocked waiting for AI results.

#### Scenario: Hook acknowledged before extraction begins
- **WHEN** the daemon receives a `POST /hooks/pre-commit`
- **THEN** 202 Accepted is written to the hook client before any AI call is initiated

---

### Requirement: Extraction input is the staged diff from the hook payload
`ExtractConcepts` SHALL use the `payload.diff` field from the `HookEnvelope` as the user turn in the extraction request. The diff is the primary signal for concept identification in Phase 3.

#### Scenario: Diff present in payload
- **WHEN** the pre-commit hook sends a payload with a non-empty `diff`
- **THEN** the full diff (up to the truncation limit) is passed as the user turn to the provider

---

### Requirement: Diff exceeding 8000 characters is truncated before extraction
If `len(payload.diff) > 8000`, the extraction request SHALL use only the first 8000 characters of the diff, with a `\n[... diff truncated for context limit ...]` marker appended. A warning SHALL be logged.

#### Scenario: Large diff truncation
- **WHEN** the staged diff is 12000 characters
- **THEN** the extraction request user turn is 8000 characters plus the truncation marker, and a warning is logged

---

### Requirement: Extraction failures degrade gracefully
If the AI call fails (network error, timeout, non-2xx response) or the response cannot be parsed as a JSON array of concepts, `ExtractConcepts` SHALL return an empty slice and log the failure. It SHALL NOT return an error that propagates to the hook response or blocks subsequent pipeline stages.

#### Scenario: Provider timeout during extraction
- **WHEN** the AI call exceeds the 10-second context timeout
- **THEN** `ExtractConcepts` logs the timeout and returns `[]ConceptFingerprint{}`; the pipeline continues without crashing

#### Scenario: Malformed AI response
- **WHEN** the provider returns non-JSON or a mismatched schema
- **THEN** `ExtractConcepts` logs a parse warning and returns `[]ConceptFingerprint{}`

---

### Requirement: Extraction prompt requests JSON array output
The system prompt in `ExtractionRequest` SHALL instruct the AI to return a JSON array where each element has `concept` (string), `source` (string, always `"code"` for diff-based extraction), and `weight` (float in [0.0, 1.0]).

#### Scenario: Valid extraction response shape
- **WHEN** the provider returns `[{"concept":"exponential backoff","source":"code","weight":0.9}]`
- **THEN** `ExtractConcepts` unmarshals it into `[]ConceptFingerprint` successfully
