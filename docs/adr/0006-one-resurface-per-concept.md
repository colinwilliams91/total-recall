# Each concept is resurfaced at most once: resurfacing exits are permanent

Classic spaced repetition resurfaces an item repeatedly, with intervals that
grow as the item survives. Total Recall resurfaces a concept exactly once.
The pressure that shaped this: questions must stay relevant to the commit
diff, so resurfacing piggybacks on normal commit events as an opt-in bonus —
and a bonus that keeps coming back for the same concept stops being a bonus
and becomes a second question stream competing with the freshness the product
sells. The "resurfaced once" promise is also what keeps the offer economics
thin: one bonus per session, most-overdue first, no review queues, no
scheduler GC.

Decision: **resurfacing exits are permanent.** A concept exits the loop as
`answered-out` when its Bonus Question is resolved — answered or skipped,
regardless of whether it Hit or Missed — and as `declined-out` upon the third
recorded decline. Exited rows are terminal: never resurrected by later Hits,
never re-offered, never GC'd. The rejected alternative was the recurring
spaced schedule (reset-on-Hit, repeated resurfacing) — defensible SRS
orthodoxy, but it multiplies offers per concept and silently converts the
opt-in bonus into a parallel curriculum. If outcome data ever shows
one-shot resurfacing underdelivers retention, that is a superseding ADR with
a stats-compatibility story for the `origin` marking, not a quiet change.

Decisions folded in: (1) the serve-time defer lives on the daemon side of the
offer — serving sets `next_eligible_after = now + 1 day` before the user
replies, so a killed session is a silent defer with no decline recorded and
no client callback needed, keeping the client thin and the nudge
punishment-free; (2) exits are two distinct states — `answered-out`
(resolution happened) vs `declined-out` (three declines) — so `torec stats`
can tell retention questions apart from disinterest; (3) a renewed Hit clears
prior declines and reschedules only while the concept is still in the loop —
the decline cap punishes continuous disinterest, not one bad week, and never
undoes an exit.

Full design, flows, and the scheduler schema:
[openspec change](../../openspec/changes/spaced-repetition-resurfacing/design.md).

## Consequences

- Long-horizon retention leans on the weakness loop (Misses feeding Focus
  Concept ranking, ADR 0004) and fresh synthesis; resurfacing is a one-shot
  retention boost per concept, by design.
- The resurfacing table only grows: one row per concept that has recorded a
  Hit, and terminal rows are never read again. Bounded by real usage; the
  design's no-GC non-goal depends on this permanence.
- The never-again promise is user-visible product behavior; relaxing it is a
  product bet that must supersede this ADR, not a template tweak.
