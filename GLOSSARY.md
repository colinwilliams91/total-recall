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

### User interaction

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

## Notes

- The word "answer" belongs to the engine only. The user selects; the engine holds the Answer Key.
