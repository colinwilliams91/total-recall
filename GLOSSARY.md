# Total Recall

A git-integrated memory system: it extracts concepts from your coding work, stores them, and resurfaces them as questions so you have to answer to remember.

## Language

### Content

**Concept**:
A piece of knowledge extracted from the user's changes; the unit that Total Recall stores and expects to resurface.

**Question**:
A synthesized prompt testing the user's recall of a Concept. Choice-based; true/false is expressed as a two-choice question, not a separate type.

**Choice**:
One option of a Question.

**Answer Key**:
The engine's designation of which Choice(s) are correct for a Question. The key always lives on the Choice rows, never as free text on the Question.
_Avoid_: "answer" outside of "Answer Key" phrasing

### Difficulty

**Difficulty Level**:
The concrete easy/intermediate/hard value a resolution produces for one synthesis call.
_Avoid_: "tier" as a synonym for the level itself

**Tier**:
A one-level movement of the Difficulty Level driven by learner evidence — escalate or de-escalate, never a fourth value. The `learner-tier` signal marks that the rule fired.

**Baseline**:
The commit-signal resolution (Difficulty Level plus fired signals) the Performance resolver starts from; learner evidence only modulates it. "Commit-signal baseline" anchors the resolver composition.

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

## Notes

- The word "answer" belongs to the engine only. The user selects; the engine holds the Answer Key.
