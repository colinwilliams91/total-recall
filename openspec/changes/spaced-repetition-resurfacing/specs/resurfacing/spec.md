## Purpose

Bring concepts the learner answered correctly in the past back once, near their decay point, as an opt-in bonus question on a normal commit event — closing the retention loop without adding a new event source, an interactive review session, or any commit-path work.

## ADDED Requirements

### Requirement: Correct answers seed resurfacing eligibility
When a question is answered correctly, the system SHALL create (or renew) resurfacing eligibility for every concept of that question, keyed on the user-scoped concept identity so eligibility follows the learner across repositories and branches. Skipped and incorrectly answered questions SHALL NOT seed eligibility — misses are already handled by the difficulty resolver's weakness focus.

#### Scenario: Correct answer seeds each concept
- **WHEN** a question about concepts A and B is answered correctly
- **THEN** both A and B become resurfacing-eligible, due after the ladder interval for the question's resolved difficulty, and the eligibility is usable from any repository on a later commit

#### Scenario: Skip does not seed
- **WHEN** a question is skipped (no choice selected)
- **THEN** no concept of that question gains or renews resurfacing eligibility

#### Scenario: Incorrect answer does not seed
- **WHEN** a question is answered incorrectly
- **THEN** no concept of that question gains or renews resurfacing eligibility

### Requirement: The resurfacing interval is a flat ladder keyed by the answered difficulty
The system SHALL schedule a concept's resurfacing using a fixed interval ladder keyed by the difficulty the question was answered at: easy = 3 days, intermediate = 7 days, hard = 14 days. Unknown or missing difficulty SHALL be treated as intermediate. No multiplicative or per-review-history interval growth SHALL be applied — the ladder is auxiliary scheduling, not a difficulty system.

#### Scenario: Hard question earns the longest interval
- **WHEN** a question answered at difficulty `hard` is answered correctly
- **THEN** the concept becomes due for resurfacing 14 days later

#### Scenario: Easy question earns the shortest interval
- **WHEN** a question answered at difficulty `easy` is answered correctly
- **THEN** the concept becomes due for resurfacing 3 days later

### Requirement: Bonus offers are served to the shell session only, most-overdue first
When the terminal ask session requests a bonus offer after resolving its commit question, and resurfacing is enabled, and at least one concept is due (current time at or past its next-eligible time), the system SHALL offer the concept with the largest overdue amount, breaking ties by oldest seeding time. At most one bonus offer SHALL be made per ask session. When no concept is due, or resurfacing is disabled, the daemon SHALL report no offer and the session ends as it does today. MCP surfaces SHALL NOT receive bonus offers — their tool and resource contracts remain unchanged in this change.

#### Scenario: A due concept is offered on a commit in another repository
- **WHEN** a concept seeded from repository A is due and the learner commits in repository B, then answers the commit question in the ask session
- **THEN** the ask session is offered a bonus about that concept (the deliberate exception to commit-diff relevance, labeled as a retention bonus)

#### Scenario: No concept is due
- **WHEN** the ask session requests a bonus offer and no eligible concept is due
- **THEN** the daemon reports no offer and the session ends exactly as it does without resurfacing

#### Scenario: Resurfacing is disabled
- **WHEN** `recall.resurfacing` is configured to `false`
- **THEN** no bonus offer is ever served and no resurfacing eligibility is written or read

#### Scenario: MCP agent asks for the next question while a concept is due
- **WHEN** an MCP client calls `recall_next` while a concept is due for resurfacing
- **THEN** the MCP response is unchanged by this capability — no bonus offer, no resurfaced question, no new fields

### Requirement: Serving an offer defers the concept by one day
When the daemon serves a bonus offer for a concept, it SHALL immediately push the concept's next-eligible time to one day later. This defer applies regardless of what the learner does with the offer, so a session that terminates at the offer is a silent defer: no decline is recorded and no offer is re-served before the deferred time.

#### Scenario: Session is killed at the offer
- **WHEN** the ask session terminates while the bonus offer is displayed (Ctrl+C, timeout, closed terminal) without an explicit accept or decline
- **THEN** no decline is recorded for the concept and the concept becomes eligible again one day after the offer was served

#### Scenario: Offer is accepted
- **WHEN** the learner accepts the offer
- **THEN** the accept flow supersedes the serve-time defer (see the answered-bonus requirement below)

### Requirement: A decline defers and counts; three declines exit the concept
When the learner explicitly declines an offer, the system SHALL record a decline for the concept. A declined concept is not re-offered before its serve-time defer elapses (one day). Upon the third recorded decline, the concept SHALL permanently exit the resurfacing loop with a declined-out state, distinct from the answered-out state, and SHALL never be offered again.

#### Scenario: First decline defers without exiting
- **WHEN** the learner declines an offer for the first time
- **THEN** the concept's decline count is 1, the concept is not offered again until its deferred eligibility time, and it remains in the loop

#### Scenario: Third decline exits the concept
- **WHEN** the learner declines offers for a concept three times
- **THEN** the concept exits the loop as declined-out and is never offered again, while remaining distinguishable from concepts that exited by answering

### Requirement: A renewed correct answer clears prior declines
When a concept that has recorded declines is answered correctly again in a normal (commit-diff) question, the system SHALL reset its schedule from the new answer (due after the ladder interval) and clear its decline count — the decline cap punishes continuous disinterest, not one bad week. Concepts that have already exited (answered-out or declined-out) SHALL NOT be resurrected by later answers.

#### Scenario: Two declines followed by a fresh correct answer
- **WHEN** a concept has 2 recorded declines and is then answered correctly in a commit question
- **THEN** its decline count returns to 0 and it becomes due again after the ladder interval for that answer's difficulty

#### Scenario: An exited concept is never resurrected
- **WHEN** a concept has exited the loop (answered-out or declined-out) and is answered correctly again in a commit question
- **THEN** it stays exited — no new offer is ever scheduled for it

### Requirement: An accepted bonus question is a fresh synthesis at the current adaptive level
When the learner accepts an offer, the system SHALL synthesize a brand-new question about the due concept — concept-level framing with no commit context, never pretending there was a commit — at the ask session's current adaptive difficulty level. The synthesized question SHALL ride the same storage, delivery, and answer flow as commit questions, and its recorded events SHALL carry an origin marking it as a resurfaced question so downstream statistics can distinguish retention questions from diff questions. After the bonus question is resolved (answered or skipped), the concept SHALL permanently exit the loop as answered-out — each concept is resurfaced at most once — regardless of whether the bonus answer was correct. A missed bonus needs no special handling: it flows into the existing learner-weakness history like any other answer.

#### Scenario: Accepted bonus produces a fresh, marked question
- **WHEN** the learner accepts an offer for a concept
- **THEN** a newly synthesized question about that concept is rendered in the session (no stored question is re-served, no commit context is referenced), its events are marked as resurfaced, and after it is resolved the concept exits the loop as answered-out

#### Scenario: Bonus synthesis fails
- **WHEN** the learner accepts an offer but the AI synthesis call fails
- **THEN** no question is rendered, no decline is recorded, the concept is not exited, and the serve-time defer stands (the concept returns after one day)

#### Scenario: Bonus answered incorrectly
- **WHEN** the learner answers the bonus question incorrectly
- **THEN** the verdict and feedback render as for any question, the concept exits as answered-out, and the miss contributes to the learner's weakness history so subsequent commit questions target it
