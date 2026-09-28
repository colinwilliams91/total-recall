# learner-performance

## Purpose

Turn recorded answer outcomes into learner-driven question difficulty and content feedback: aggregate per-concept results (which topics the user is missing) and drive both the difficulty tier and the synthesis feed's weak-concept focus from the learner's demonstrated performance, with the commit-shape heuristics demoted to a cold-start baseline.

## ADDED Requirements

### Requirement: Resolution aggregates windowed per-question outcomes from the event spine
The learner stats source SHALL join `question_events` ('answered' rows, ordered by `occurred_at DESC`) to their per-question selections and correct-choice flags, scoped per repo and branch. The aggregation SHALL yield, per question: whether the user answered correctly (all selected choices correct for `multiple_choice`), and, through the provenance join, the set of concepts the question targeted. Skipped questions are excluded from ratios and never count as failures for difficulty tiering (they count toward history-freshness only).

#### Scenario: Correctness per concept derived from outcomes
- **WHEN** the last three answered questions link concept `circuit-breaker` and the user missed two of them
- **THEN** the aggregate reports 1 correct / 3 answered for `circuit-breaker` (and every other linked concept of those questions)

#### Scenario: Skipped questions do not count as failures
- **WHEN** the recent history is 4 correct, 1 incorrect, and 2 skipped answers
- **THEN** the correctness ratio is 4/5 (skips excluded) while the concept(s) of the skipped questions show no outcome-derived weakness

---

### Requirement: Correctness ratio tiers escalate or de-escalate the commit-signal baseline
With a sufficient recent answered window (default 20, configurable), the aggregation's correctness ratio SHALL adjust the commit-signal baseline exactly one tier per rule:

- ratio ≥ 0.92 → the resolver SHALL escalate the baseline one level (`easy` → `intermediate`, `intermediate` → `hard`, `hard` unchanged);
- ratio < 0.50 → the resolver SHALL de-escalate one level (`hard` → `intermediate`, `intermediate` → `easy`, `easy` unchanged);
- otherwise → the baseline unchanged.

The returned difficulty is one of `easy`, `intermediate`, `hard` — always a concrete level.

#### Scenario: High-performing learner escalates
- **WHEN** the last 20 answers include 19 correct (ratio 0.95) and the commit-signal baseline is `intermediate`
- **THEN** the resolved difficulty is `hard` (and the log names `learner-tier` among the fired signals)

#### Scenario: High-performing learner at the ceiling is capped
- **WHEN** the user's ratio ≥ 0.92 and the commit-signal baseline is already `hard`
- **THEN** the resolved difficulty is `hard` (no double escalation)

#### Scenario: Struggling learner de-escalates
- **WHEN** the last 20 answers include 7 correct (ratio 0.35) and the commit-signal baseline is `hard`
- **THEN** the resolved difficulty is `intermediate`

#### Scenario: Mid-range ratio keeps the baseline
- **WHEN** the user's ratio is 0.75 and the commit-signal baseline is `intermediate`
- **THEN** the resolved difficulty is `intermediate`

---

### Requirement: Insufficient learner history preserves the commit-signal baseline
When the number of answered questions in scope is below the minimum history threshold, the resolution SHALL be the commit-signal baseline only: the commit-shape heuristics resolve the difficulty, no weak-concept focus is produced, and no escalation or de-escalation applies. This is the cold start in which the existing commit-signal behavior is fully preserved.

#### Scenario: Fresh install delegates to commit-signal heuristics
- **WHEN** the store has fewer answered questions than the minimum history window (default 5) at synthesis time
- **THEN** the resolved difficulty equals what the commit-shape heuristics produce for the same `SynthesisContext`, and the returned focus set is empty

#### Scenario: History window boundary is inclusive
- **WHEN** the store holds exactly the minimum answer count (e.g. 5 answered) and the ratio crosses a tier threshold
- **THEN** the tier rules apply (history is sufficient from the Nth answered question onward)

---

### Requirement: Weakness-driven focus concepts bias the synthesis feed toward demonstrated weaknesses
Whenever learner history exists, the resolution SHALL return non-empty focus concepts selected from the user's demonstrated weaknesses: concepts with recent answers rank by miss count (misses descending, then most-recent first), bounded to a configurable focus cap (default 3). A focus concept is only ranked weak when the learner answered at least one question linked to it and the recent outcomes for it are not exclusively correct. Zero-outcome concepts never enter the focus set.

#### Scenario: Repeatedly-missed concept becomes the focus
- **WHEN** `circuit-breaker` has 2 misses and 1 recent hit, and other linked concepts are all recently correct
- **THEN** `circuit-breaker` is the first (or sole) focus concept returned

#### Scenario: All-recently-correct concepts produce no focus
- **WHEN** every concept linked to the last N answers has a perfect recent record
- **THEN** the returned focus set is empty and synthesis feeding is unaffected

#### Scenario: Focus respects the per-branch working surface
- **WHEN** a weak concept was answered under a different repo/branch than the current synthesis
- **THEN** that concept does not outrank the current repo/branch's weak concepts (it may be appended only if the focus cap is not already filled) — working-surface relevance stays first-class, tutor scheduling second-class
