## MODIFIED Requirements

### Requirement: Extraction prompt requests JSON array output
The system prompt in `ExtractionRequest` SHALL instruct the AI to return a JSON array where each element has `concept` (string), `slug` (string, kebab/snake-cased canonical identity like `go_interfaces` or `sql_joins`), `source` (string, always `"code"` for diff-based extraction), and `weight` (float in [0.0, 1.0]). The `slug` element identifies the concept across repos and branches; the `concept` prose remains the human-readable fingerprint. Malformed or missing slugs degrade the element per the concept-identity validation requirement — they do not fail the whole extraction.

#### Scenario: Valid extraction response shape
- **WHEN** the provider returns `[{"concept":"exponential backoff","slug":"exponential_backoff","source":"code","weight":0.9}]`
- **THEN** `ExtractConcepts` unmarshals it into `[]ConceptFingerprint` successfully, preserving both prose and slug

#### Scenario: Missing slug degrades the element, not the batch
- **WHEN** the provider returns `[{"concept":"retry logic","source":"code","weight":0.8}]` with no `slug` field
- **THEN** that element is dropped with a logged warning, and other elements in the same response are processed normally
