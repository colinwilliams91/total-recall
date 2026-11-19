# Proposal: Spaced Repetition Resurfacing

## Why

Total Recall quizzes the commit diff, so a concept you learned three weeks ago
never comes back on its own — retention decays silently and the product's core
thesis ("preserve engineering intuition") is only half-served: we test what
you just touched, not what you are about to forget. Resurfacing closes the
retention loop: concepts answered correctly come back once, right around the
decay point, as an *opt-in bonus* on a normal commit event. It ships before
the v1.0.0 launch (GTM.md, Wave B) because it is the feature that turns
"quiz on commit" into a retention tool.

## What Changes

- **Resurfacing scheduler** — a new SQLite table keyed on the user-scoped
  `concept_registry` slug tracks, per concept: next-eligible-after, decline
  count, resurfaced count, and exit state. Rows are written at *answer* time
  (same transaction as the answer) for every concept of a correctly answered
  question. Skipped and missed questions seed nothing — misses already feed
  the existing weakness loop.
- **Bonus offer at ask time** — after the commit-diff question is answered in
  a `torec ask` session, the shell asks:
  "Would you like a bonus question to boost your knowledge retention? y/n".
  `y` synthesizes a fresh question about the due concept (option A — no
  stored-question re-serving) at the learner's *current adaptive difficulty*
  level; `n` defers the concept by one day and increments its decline count.
- **Decline semantics** — decline = defer by 1 day (timer rewound one day, not
  re-offered immediately); a cap of **3 declines** exits the concept from the
  loop as `declined-out`. A successfully answered bonus question exits it as
  `answered-out` (max one resurface per concept). A killed ask session at the
  offer is a *silent defer* (+1 day, no decline increment).
- **`GET /recall/bonus` endpoint** — returns the most-due concept for the
  session (204 if none due); the accept/decline outcome is recorded via the
  same endpoint family. One bonus per ask session; hook pipeline and commit
  path untouched.
- **Shell-only in v1** — MCP surfaces keep their current `recall_next` /
  `recall_answer` contract; the future first-class MCP story is tracked in
  issue #68 (auto-accept exploration, deliberately deferred).
- **Config kill-switch** — `recall.resurfacing: true|false` (default true).
- **Policy asset extension** — the question-generation policy gains a
  "resurfaced concept" framing: no commit context, ask about durable
  understanding, never pretend there was a commit.
- **Provenance** — bonus questions and their answer events carry their origin
  in the event payload so `torec stats` (Wave B, `10-opsx-e`) can separate
  diff-question accuracy from retention-question accuracy.

### Developer workflow impact

Zero new friction on the commit path: the hook pipeline is unchanged, commit
latency untouched. The only new interaction is one optional y/n prompt inside
the existing post-commit `torec ask` session, *after* the normal question is
already answered — declining costs one keypress and defers silently.

Background processing vs commit-time execution: all scheduler writes happen in
the daemon's async pipeline at answer time; the bonus synthesis happens on
demand inside the ask session only after `y`. Nothing new runs at commit time.

## Capabilities

### New Capabilities

- `resurfacing`: the spaced-repetition scheduler (pool seeding, intervals,
  decline/defer/exit semantics) and the daemon bonus-offer contract
  (`GET /recall/bonus`, due-concept selection, exit states).

### Modified Capabilities

- `tr-ask`: the ask session gains a post-answer bonus-offer state machine
  extension (offer y/n, bonus question delivery, silent-defer on session
  kill) — behavior change to the TUI session flow.

## Impact

- **Code**: `internal/cache/store.go` (new scheduler table + accessors;
  manual-migration caveat applies — delete `memory.db` after a
  schema-changing build, per AGENTS.md), `internal/recall/engine.go`
  (resurfaced-concept synthesis path reusing `FocusConcepts` + current
  adaptive level), `internal/engine` (new `/recall/bonus` routes),
  `cmd/torec/ask.go` (offer + bonus states), `internal/config`
  (`recall.resurfacing`), `assets/prompts/question-generation-policy.md`
  (new framing section).
- **APIs**: new `GET /recall/bonus` (+ record outcome) on the daemon; MCP
  contract unchanged.
- **Dependencies**: none new — modernc.org/sqlite as today.
- **Downstream**: `torec stats` (`10-opsx-e`) consumes the provenance and
  exit states; the B2 change will build on this one's schema.

## Non-goals

- **No SM-2 / easiness factors** — a single-review-per-concept design gives
  SM-2's multiplicative machinery no data; adaptive difficulty already owns
  the "how hard" axis. The interval ladder (3d/7d/14d keyed by answered
  level) is auxiliary scheduling, not a difficulty system.
- **No stored-question re-serving** (option B rejected — choice-order leakage
  measures recognition, not retention).
- **No new event sources** — commits remain the only trigger; the bonus
  piggybacks on the existing post-commit ask session.
- **No MCP delivery in v1** — issue #68 owns the future story.
- **No full SRS sessions / review queues** — this is opportunistic retention
  at commit time, not a flashcard product.
- **No scheduler GC** — exit states prune eligibility; rows are tiny and the
  pool growth is bounded by real usage.
