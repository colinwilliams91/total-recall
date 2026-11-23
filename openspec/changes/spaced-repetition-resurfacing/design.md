# Design: Spaced Repetition Resurfacing

## Context

Facts from the current architecture that shape this design (motivation in
proposal.md — Why):

- Every question is synthesized fresh from a commit's `SynthesisContext` and
  delivered exactly once (`questions`: `queued → delivered → answered/skipped`).
  There is no stored-question re-serving path today.
- The difficulty pipeline already has a concept-injection point:
  `Resolution.FocusConcepts` rides into `SynthesisRequest`, and the resolved
  level rides out in the queued `question_events` payload. A resurfaced
  concept rides the same injection point but arrives via the scheduler
  instead of via weakness detection — it is not a Focus Concept (no
  demonstrated weakness), and unlike a Focus Concept it must never be
  dropped from the feed.
- Concept identity is user-scoped at `concept_registry` (ADR 0004: weaknesses
  follow the learner across repos); sightings and question tables are
  repo+branch scoped.
- `torec ask` is a Bubble Tea state machine (`thinking → question → feedback →
  done`) that polls `/recall/next` (atomic dequeue) and records outcomes via
  `/recall/select`.

## Goals / Non-Goals

**Goals:**

- Scheduler state that is written at answer time and read at ask time — zero
  new work on the commit path.
- Bonus questions indistinguishable from commit questions downstream: same
  synthesis machinery, same tables, same select-and-feedback flow.
- Offer economics that respect a "no": defer, don't nag.

**Non-Goals:**

- (Restated in proposal Non-goals: no SM-2, no re-serving, no MCP in v1.)
- No change to `recall_next` / `recall_select` wire contracts.
- No scheduler GC — exit states make rows ineligible; storage is bounded by
  real usage.

## Decisions

### D1 — Fresh synthesis at the user's current adaptive level (option A)

On `y`, the daemon synthesizes a new question about the due concept: no diff
snippet, concept-level framing, difficulty = the session's current adaptive
`Resolution.Difficulty` (the same value that shaped the commit question). A
resurfaced concept is passed to synthesis the way `FocusConcepts` already is.

*Alternative considered — re-serving the stored question:* rejected. It would
introduce the first-ever re-served row (new delivery mode), test recognition
of choice order rather than retention of the idea, and require answer-dedup
semantics. Fresh synthesis rides existing machinery.

### D2 — Scheduler writes at answer time, reads at ask time

Seeding happens inside the same transaction as the answer: when a question
records a Hit, upsert one `resurfacing` row per concept slug. Due-check
happens only when the ask session calls `GET /recall/bonus`. The hook
pipeline, hook routes, and commit latency are untouched.

Retention Interval key: the *answered question's* resolved Difficulty Level,
read back from its queued `question_events` payload; absent/unknown →
`intermediate`.

*Alternative considered — hook-time staging of a pre-synthesized bonus
question:* rejected. Dequeue is exactly-once, so a staged question that the
user declines is burned (an AI call wasted per decline), and staging couples
the scheduler to the pipeline.

### D3 — Scheduler table (additive, user-scoped)

```sql
CREATE TABLE IF NOT EXISTS resurfacing (
  slug                TEXT PRIMARY KEY REFERENCES concept_registry(slug),
  seeded_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  next_eligible_after DATETIME NOT NULL,  -- due when now >= this
  interval_days       INTEGER NOT NULL,   -- Retention Interval value at seed time
  declined_count      INTEGER NOT NULL DEFAULT 0,
  resurfaced_count    INTEGER NOT NULL DEFAULT 0,
  exit_reason         TEXT                -- NULL | 'answered-out' | 'declined-out'
);
```

Keyed on the registry slug: eligibility is user-scoped (a concept due from
repo A can surface during a repo B commit — the deliberate exception to
"questions follow the diff", opt-in and labeled as a bonus). The question
itself is synthesized with the current repo+branch context, so `Synthesize`'s
required scope is satisfied.

**No `ALTER TABLE` anywhere — this is a new table only, so `CREATE TABLE IF
NOT EXISTS` migrates cleanly without the delete-`memory.db` step** (that
caveat applies when existing tables change; they do not here).

### D4 — Dumb Retention Interval, no SM-2

| Answered at | interval_days |
|---|---|
| easy | 3 |
| intermediate | 7 |
| hard | 14 |

Three constants keyed by the answered Difficulty Level. SM-2's easiness factor and
success chain need multiple reviews per item; the max-once rule supplies
exactly one. Rationale in proposal Non-goals.

### D5 — Decline = serve-time defer; declines are finite; kills are silent

The **serve is the defer**: when `GET /recall/bonus` returns a due concept,
the daemon immediately sets `next_eligible_after = now + 1 day`. Consequences:

- `y` → accept path overrides the defer and exits the concept.
- `n` → client posts a decline; daemon increments `declined_count`. At
  **3 declines** → `exit_reason = 'declined-out'`. The deferred timer from
  serve still stands, so a decline costs one future offer, one day away.
- Session killed at the offer → no client callback, and none is needed: the
  serve-time defer already fired. **Silent defer, no punishment** — this is
  why defer lives on the daemon side of the offer, not in the client.
- A later Hit on the same concept (Commit Question) renews the schedule and
  clears `declined_count` — only while the concept is still in the loop;
  exited rows are terminal. The cap counts continuous disinterest, and fresh
  evidence restarts the lease.

Exit permanence — answered-out and declined-out rows are never resurrected —
is a recorded bet: see docs/adr/0006.

### D6 — One bonus per session, most-overdue wins

`GET /recall/bonus` selects the eligible concept with the largest
`now − next_eligible_after` overage. Ties broken by `seeded_at` ascending
(oldest first). No batching.

### D7 — Provenance in event payloads (no schema change to `questions`)

Bonus questions carry `origin: "resurfaced"` in their queued
`question_events` payload (the payload is JSON and already carries resolved
difficulty and fired signals). `torec stats` (Wave B) separates
commit-question accuracy from bonus-question accuracy by this field.

Note: `openspec/specs/question-events/spec.md` names `resurfaced` among
*future lifecycle event types* (row inserts). This design deliberately adds
no such event type — the origin marking rides the existing queued event's
JSON payload, superseding that forward-looking example for resurfacing;
the event-type set stays unchanged.

### D8 — Shell-only v1; MCP contract frozen

The offer exists only in the `torec ask` session. The MCP Delivery Surface
keeps `recall_next` / `recall_select` untouched. Scheduler state is
Delivery-Surface-agnostic, so the future MCP story (issue #68 — auto-accept)
extends the loop without rework.

### D9 — Config kill-switch

`recall.resurfacing: true|false` (default `true`, user-level config). When
`false`: `GET /recall/bonus` returns 204 unconditionally and answer-time
seeding is skipped (no rows written, nothing to un-see). Deep-merged like the
rest of `recall.*`.

### D10 — Bonus question lifecycle

On accept, the daemon synthesizes and stores the question as a normal row
(delivered event `actor = 'shell'`, already delivered to this session) and
returns it in the accept response — no second poll, no re-dequeue. The user
selects a choice via the existing `/recall/select` flow; the store then sets
`exit_reason = 'answered-out'` for the concept's scheduler row (regardless of
whether it Hit or Missed — max one resurface; a Miss is handed to the
weakness loop automatically by the existing Performance resolver, which
reads the same Selection/Hit/Miss history).

A session that dies between acceptance and resolution is intentional
non-exit, not a gap: the retention question never happened, so the concept
keeps its single future offer. The serve-time defer stands — the concept
becomes eligible again one day after the offer was served — and no decline is
recorded. Only a resolved Bonus Question spends the one-resurface budget
(see docs/adr/0006).

## Flows

### Bonus offer (ask session)

```
torec ask                                     daemon
   | GET /recall/next?repo&branch               |
   |------------------------------------------->|
   | 200 commit question (atomic dequeue)       |
   | render, user answers                       |
   | POST /recall/select                        |
   |------------------------------------------->| store selection;
   |                                            | on Hit: seed/reschedule
   | render feedback                            | resurfacing rows (txn)
   | GET /recall/bonus?repo&branch              |
   |------------------------------------------->| due concept? serve + defer
   | 200 {slug, prose} | 204 none               | (next_eligible_after = now+1d)
   |                                            |
   | [offer] "Bonus question? y/n"              |
   |   n --> POST decline ----------------------|--> declined_count++
   |   y --> POST accept ----------------------->| synthesize at current
   | 200 bonus question (delivered, actor shell)|  adaptive level; store row;
   | render, answer, feedback                   |  events origin=resurfaced
   | concept exits answered-out                 |
 done                                          (kill at offer: no callback,
                                                serve-time defer stands)
```

### Answer-time seeding (transaction)

```
POST /recall/select (Hit)
   |
   v
BEGIN
  existing selection/event writes
  for each concept slug of the question:
    upsert resurfacing(slug):
      next_eligible_after = now + interval(question level)
      interval_days       = interval(question level)
      declined_count      = 0            (renewal clears disinterest)
      exit_reason         = NULL         (only if previously NULL;
                                          exited rows are terminal)
COMMIT
```

## Risks / Trade-offs

- [Offer fatigue annoys users into disabling resurfacing] → defer-by-one-day
  plus the 3-decline cap plus one-offer-per-session bounds the nudge rate;
  `recall.resurfacing: false` is the always-available exit.
- [AI latency on accept feels slow] → the same "Thinking..." animation the
  session already uses covers synthesis; synthesis failure is a silent defer
  (no decline increment), so a flaky provider never punishes the concept.
- [Cross-repo surfacing surprises a user ("why is this repo's quiz about that
  other project?")] → the offer labels itself as a knowledge-retention bonus,
  not a commit question; framed in the policy asset and the TUI copy.
- [Table grows unboundedly] → rows are one per concept that has recorded a Hit,
  terminal rows stop being read, and no queries scan the table at commit
  time (only the ask-session due lookup, indexed on `next_eligible_after`).
- [Retention Interval feels arbitrary] → three constants, documented, trivially tunable
  later; deliberately not a formula (see D4).
- [Serve-time defer races a slow accept] → acceptance overrides the defer and
  the row exits when the Bonus Question resolves; a session killed after
  acceptance falls back to the standing defer (intentional non-exit, see
  D10); no double-offer window exists because the row is not eligible again
  until one day after serve.

## Migration Plan

1. Ship: new `resurfacing` table via `CREATE TABLE IF NOT EXISTS` in
   `Open()` (additive — existing `memory.db` files keep working, no manual
   reset required).
2. Config default `recall.resurfacing: true` turns the feature on for
   existing users; `false` reverts to today's behavior exactly (204 + no
   seeding).
3. Rollback: set the config flag to `false`; the table simply stops being
   read or written. No data migration backward.

## Open Questions

- Exact TUI copy for the offer line and its keybindings (`y`/`n`, Esc =
  silent defer as the kill-equivalent for accessibility) — implementation
  detail, deferred to tasks with golden-file coverage per the repo's test
  conventions.
