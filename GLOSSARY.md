# Total Recall

A git-integrated memory system: it extracts concepts from your coding work, stores them, and resurfaces them as questions so you have to answer to remember.

## Language

### Content

**Concept**:
A piece of knowledge extracted from the user's changes; the unit that Total Recall stores and expects to resurface.

**Question**:
A synthesized prompt testing the user's recall of a Concept. Choice-based; true/false is expressed as a two-choice question, not a separate type.

**Quiz**:
The user-facing word for a Question: what the user "gets" on a git event. One quiz is one Question — the word adds no separate structure or state.
_Avoid_: treating quiz and Question as different things; "quiz" for engine-side state

**Choice**:
One option of a Question.

**Answer Key**:
The engine's designation of which Choice(s) are correct for a Question. The key always lives on the Choice rows, never as free text on the Question.
_Avoid_: "answer" outside of "Answer Key" phrasing

### Difficulty

**Difficulty Level**:
The concrete easy/intermediate/hard value a resolution produces for one synthesis call.
_Avoid_: "tier" as a synonym for the level itself

**Resolution**:
One resolver call's outcome: the Difficulty Level, optional Focus Concepts, and the fired signals carried to the queued-event audit payload.

**Tier**:
A one-level movement of the Difficulty Level driven by learner evidence — escalate or de-escalate, never a fourth value. The `learner-tier` signal marks that the level actually moved: a rule engagement held at a cap (`hard` escalated, `easy` de-escalated) is not a movement and does not signal.

**Baseline**:
The commit-signal resolution (Difficulty Level plus fired signals) the Performance resolver starts from; learner evidence only modulates it. The commit-shape heuristics produce the commit-signal baseline — "shape" names the per-commit input, "signal" names what fired on it.

**Focus Concept**:
A demonstrated-weakness concept the resolver ranks to lead the synthesis feed for one synthesis call. Biasing reorders existing feed rows and never invents content; a Focus Concept absent from the current feed is dropped.

**Working Surface**:
The repo+branch pair a synthesis is scoped to. Sightings and Questions scope to it, and the difficulty tier window reads it; weakness tallies key on user-scoped concept identity and may span surfaces (ADR 0004).
_Avoid_: "surface" without the working- qualifier when scoping matters

### User interaction

**Learner**:
The user as seen through recorded Question outcomes — the evidence base the performance resolver reads. Not a persona: the recorded answered history is the Learner.

**Miss**:
The incorrect outcome of an Answered Question, tallied per linked concept through the provenance join. Misses drive Focus Concept ranking.

**Hit**:
The correct outcome of an Answered Question, tallied per linked concept through the provenance join.

**Selection**:
One user pick: the (Question, Choice) pair recorded when a user answers. A multi-select Question yields multiple Selections; skipping yields none.
_Avoid_: answer, submission (for completed picks)

**Select**:
The user-side verb: the user selects. All user-facing language uses select/selected.
_Avoid_: answer, submit (user-facing phrasing)

### Resurfacing

**Resurfacing**:
The mechanism that brings a concept back once through the normal ask session: a Hit seeds resurfacing eligibility for that concept (skips and Misses never seed), the elapsed Retention Interval makes it due, and the session may then offer a bonus. Not a second event source — it rides normal commit events.

**Retention Interval**:
The flat schedule that sets when a resurfacing-eligible concept comes due, keyed by the Difficulty Level its question was answered at, with no growth on re-offers. Auxiliary scheduling, not a difficulty system.
_Avoid_: decay time, SM-2

**Bonus Offer**:
The opt-in prompt the ask session makes for one due concept — most-overdue first — after the commit question is resolved. Serving it defers the concept one day regardless of the reply; an explicit decline counts toward the three-decline exit.

**Bonus Question**:
The fresh question synthesized about a due concept when a Bonus Offer is accepted — concept-level framing with no commit context, delivered through the normal question flow. One term: "resurfaced" is this question's origin marking, not a second question type.
_Avoid_: "resurfaced question" as a separate term

**Exit**:
A concept leaves the resurfacing loop permanently, as answered-out (its Bonus Question was resolved, answered or skipped) or declined-out (three declines). Exited concepts are never resurrected by later Hits.

### Lifecycle

**Question Event**:
One row in the Question's lifecycle audit log; the source of truth for what happened and when.

**Queued**:
Synthesized and stored, not yet shown to the user.

**Delivered**:
Claimed and presented to the user, awaiting reply.

**Answered**:
A Selection was recorded and grading resolved, atomically with the Selection today.
_Avoid_: graded as a separate user-visible step (not yet built)

**Skipped**:
The user declined to answer, whether or not they ever saw the Question. Event history, not vocabulary, carries the seen/never-seen distinction.

**Cold Start**:
The baseline-only regime before enough answered history exists: the commit-shape heuristics resolve alone — no learner tier, no focus concepts. Marked by the `cold-start` signal.

**Autostart**:
The opt-in arrangement that starts the daemon at login, so recall works across reboots without the user remembering to serve. Autostart only launches the daemon; stopping it and restarting it remain the user's lifecycle actions until the next login.

## Notes

- The word "answer" belongs to the engine only. The user selects; the engine holds the Answer Key.
