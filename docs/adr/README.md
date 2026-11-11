# ADRs — Definition and Procedure

This is the canonical definition of how ADRs work in this repo: what qualifies,
structure and numbering, and the operating procedure. ADRs are the formal record of
the non-reversible decisions that shape the repo; 0001–0003 already embody this
practice, and this write-up makes it explicit.

## What qualifies as an ADR

An ADR is written only when **all three** of these are true:

1. **Hard to reverse.** The cost of changing your mind later is meaningful (data migration, API break, release churn), not a plain refactor.
2. **Surprising without context.** A future reader will look at the code or specs and ask *"why did they do it this way?"* Deliberate deviations from the obvious path qualify even when they feel small — their whole job is stopping the next engineer
   from "fixing" a deliberate choice.
3. **The result of a real trade-off.** There were genuine, defensible alternatives and one was picked for specific reasons.

If any of the three is missing, skip the ADR: a reversible decision will be reversed, an unsurprising decision will never be questioned, an only-option decision has nothing to record beyond "we did the obvious thing." Also skip the ADR when the only new fact is a rejected alternative that is obvious in six months.

## Location and numbering

- ADRs live in `docs/adr/`, named `NNNN-slug.md` (zero-padded, sequential: `0001-question-storage-schema.md`), scan the directory and increment.
- **The set is append-only.** Never renumber, never delete. A reversed or buried decision gets a new ADR that states the new decision and notes `superseded by NNNN`; the old file is not rewritten, because it recorded faithfully what was decided then.

## Format

An ADR is prose, not a form. Structure (title → decision → optional Consequences):

```md
# <The decision, stated as a claim>

<Context. What the pressure was. What the pre-refactor/alternative state looked like.>

<Decision and rationale. What we chose, why, and the immediate cost/benefit reckoning that made it cheap to decide *now*.>

Decisions folded in: (optional) a compact numbered list of sub-decisions absorbed
into this pass, each with a one-line rationale and the rejected alternative.

Full design, rejected alternatives, and implementation tasks:
[openspec change](<link to the change's design.md>).

## Consequences (optional)

- <Reserved-but-unused machinery and warnings against misreading it;
  non-obvious downstream effects.>
```

Notes:

- **Single claim.** One decision per ADR; title says it in one line.
- **Decisions folded in** is the house pattern for the small sub-decisions absorbed while executing a larger one; fold them into the ADR rather than minting one per sub-decision.
- The full design, the extended rejected-alternatives discussion, and the task breakdown live in the **OpenSpec change's `design.md`** — the ADR points there rather than duplicating.

## Relationship to OpenSpec

The ownership split (evolved over `0001`–`0003` and worth preserving):

| Concern | Owner |
|---|---|
| Behavior contract — scenarios, requirements | `openspec/specs/*` |
| Full design + rejected-alternatives trace | `openspec/changes/<slug>/design.md` |
| The decision itself, the product bet, the boundary ("there will never be multiple policies") | `docs/adr/` |

Specs describe how the loader works; they do not enshrine in prose that the capability will never be exercised. If a statement is a *bet about the future* rather than a *behavior contract*, it belongs in the ADR.

## Operating procedure

1. **Surface it during design, not after code.** When design or implementation presses on a decision that passes the three tests, name it, offer the ADR, and record it.
2. **Write it when the change is being implemented.** Not aspirationally, and not weeks later when the rationale has gone soft. The ADR is committed alongside or immediately after the OpenSpec change lands.
3. **Cross-link out.** Where code, docs, or specs implement/depend on the decision, mention `(see docs/adr/NNNN)` so readers can find the why.
4. **Preserve history.** Consequences get appended if they surface later; the decision body itself is never edited — a change of mind is a new superseding ADR.

## Existing ADRs

`0001`–`0003` predate this write-up and follow it in form and spirit. One known variance: `0002` predates the "Full design → openspec change" footer convention and hands its context entirely to prose. Treat them as canonical examples; do not retrofit the variance.
