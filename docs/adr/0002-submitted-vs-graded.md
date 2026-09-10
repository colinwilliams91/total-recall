# "Answered" is atomic; "Submitted" is reserved for failure recovery

Today a Selection is recorded, graded, and the Question reaches terminal state in one atomic step — so the vocabulary has one word for it, **Answered**, and no intermediate state. The plausible failure case (a Submission arrives but grading never completes, e.g. the connection to the LLM drops) will need a distinguishable **Submitted** state with retry/idempotency handling, but we deliberately deferred building it rather than speculatively splitting the lifecycle now. We also considered grading as a designed async step (as free-text deferred grading would have required) and rejected it: grading for choice-based Questions resolves essentially simultaneously with the Selection, so a two-phase lifecycle adds vocabulary without clarity for the everyday path.

## Consequences

- `grade` and related event types stay available in the `question_events` CHECK but are intentionally unused; do not mistake their presence in the enum for live behavior.
- When the failure-recovery state is built, it needs its own design pass (durable storage of the ungraded Submission, retries, idempotency) — this ADR records the reservation, not a design for it.
- These states are lifecycle machinery, not user-facing language: user-facing phrasing is select/selected; "answered" describes the completed outcome.
