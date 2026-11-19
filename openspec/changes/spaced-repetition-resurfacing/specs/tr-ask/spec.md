## MODIFIED Requirements

### Requirement: stateFeedback blocks on evaluation and renders "Evaluating..."
After a choice keypress, `torec ask` SHALL transition to `stateFeedback` and render `"Evaluating..."` as a static string. It SHALL block until a `feedbackMsg` is received, then transition to `stateDone` and call `tea.Quit` — unless a bonus offer was served for this session, in which case it SHALL instead continue into the bonus offer flow (see the bonus requirements below) before reaching `stateDone`.

#### Scenario: Ctrl+C in stateFeedback - clean exit
- **WHEN** the developer presses Ctrl+C while in `stateFeedback`
- **THEN** `torec ask` transitions to `stateDone` and exits; the in-flight HTTP response is discarded

#### Scenario: No bonus offer served - session ends after feedback as before
- **WHEN** feedback resolves and no bonus offer was served for the session (none due, disabled, or not requested)
- **THEN** `torec ask` transitions to `stateDone` and exits exactly as it does without resurfacing

#### Scenario: Bonus offer served - session continues after feedback
- **WHEN** feedback resolves and a bonus offer was served for the session
- **THEN** `torec ask` continues into the bonus offer flow instead of quitting

## ADDED Requirements

### Requirement: Bonus offer is presented after the commit question resolves
When a bonus offer has been served (a due concept returned by the daemon), `torec ask` SHALL present an offer to the learner after the commit question's feedback renders, clearly framed as a knowledge-retention bonus (not a commit question). The offer SHALL accept `y` (accept) and `n` (decline). Pressing `q`/`Esc` at the offer, or session termination of any other kind (Ctrl+C, timeout, closed terminal), SHALL end the session without recording a decline and without posting anything — the daemon's serve-time defer already stands.

#### Scenario: Learner accepts with y
- **WHEN** the learner presses `y` at the bonus offer
- **THEN** the session proceeds to fetch and render the bonus question

#### Scenario: Learner declines with n
- **WHEN** the learner presses `n` at the bonus offer
- **THEN** the session posts the decline to the daemon, the concept's decline count increments, and the session ends

#### Scenario: Session killed at the offer
- **WHEN** the session terminates while the offer is displayed without an explicit `y` or `n`
- **THEN** nothing is posted, no decline is recorded, and the concept remains eligible after its serve-time defer

### Requirement: The bonus question renders and resolves like a commit question
After acceptance, `torec ask` SHALL fetch the freshly synthesized bonus question and render it with the existing question interaction (numbered choices, Enter to skip, q/Esc silent exit), then render the verdict and feedback through the existing feedback flow. The bonus question's answer or skip SHALL be recorded through the same answer endpoint as commit questions.

#### Scenario: Bonus question answered
- **WHEN** the learner selects a choice on the rendered bonus question
- **THEN** the answer is recorded via the same answer POST as commit questions, the verdict and feedback render, and afterwards the session ends

#### Scenario: Bonus question skipped
- **WHEN** the learner presses Enter on the bonus question
- **THEN** the skip is recorded like any skip, the acknowledgement renders, and afterwards the session ends

#### Scenario: Bonus fetch fails after acceptance
- **WHEN** the learner accepts but the bonus question cannot be fetched (synthesis or daemon error)
- **THEN** the session ends without rendering a question and without recording a decline — the concept's serve-time defer stands
