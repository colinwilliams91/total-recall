---
name: total-recall-domain-modeling
description: Build and sharpen this project's domain model. Use when discussing codebase terminology, writing or editing GLOSSARY.md, or recording or editing an ADR in this repo.
---

# Domain Modeling

Actively build and sharpen the project's domain model as you design. This is the *active* discipline: challenging terms, inventing edge-case scenarios, and writing the glossary and decisions down the moment they crystallise. (Merely *reading* `GLOSSARY.md` for vocabulary is not this skill: that's a one-line habit any skill can do. This skill is for when you're changing the model, not just consuming it.)

This is the repo-local copy of the `domain-modeling` skill (from mattpocock/skills), renamed to `total-recall-domain-modeling` so it can coexist with the unmodified global install, and adapted for this repo's house conventions — notably the ADR procedure, which is defined in-repo at [docs/adr/README.md](../../../docs/adr/README.md) rather than the upstream generic format.

## File structure

This repo is single-context:

```
/
├── GLOSSARY.md
├── docs/
│   └── adr/
│       ├── 0001-question-storage-schema.md
│       └── ...
└── src/
```

`GLOSSARY.md` is the one glossary at the repo root; the repo has deliberately opted out of the multi-contex GLOSSARY-MAP layering. If a genuine second bounded context ever shows up, raise it as a discussion before inventing the structure.

Create files lazily: only when you have something to write. If no `docs/adr/` file exists for the decision, create it when the ADR is needed.

## During the session

### Challenge against the glossary

When the user uses a term that conflicts with the existing language in `GLOSSARY.md`, call it out immediately. "Your glossary defines 'cancellation' as X, but you seem to mean Y. Which is it?"

### Sharpen fuzzy language

When the user uses vague or overloaded terms, propose a precise canonical term. "You're saying 'account': do you mean the Customer or the User? Those are different things."

### Discuss concrete scenarios

When domain relationships are being discussed, stress-test them with specific scenarios. Invent scenarios that probe edge cases and force the user to be precise about the boundaries between concepts.

### Cross-reference with code

When the user states how something works, check whether the code agrees. If you find a contradiction, surface it: "Your code cancels entire Orders, but you just said partial cancellation is possible. Which is right?"

### Update GLOSSARY.md inline

When a term is resolved, update `GLOSSARY.md` right there. Don't batch these up: capture them as they happen. Use the format in [GLOSSARY-FORMAT.md](./GLOSSARY-FORMAT.md).

`GLOSSARY.md` should be totally devoid of implementation details. Do not treat `GLOSSARY.md` as a spec, a scratch pad, or a repository for implementation decisions. It is a glossary and nothing else.

### Offer ADRs sparingly

Only offer to create an ADR when all three are true:

1. **Hard to reverse**: the cost of changing your mind later is meaningful
2. **Surprising without context**: a future reader will wonder "why did they do it this way?"
3. **The result of a real trade-off**: there were genuine alternatives and you picked one for specific reasons

If any of the three is missing, skip the ADR.

**When the decision does qualify, follow the house ADR procedure in [docs/adr/README.md](../../../docs/adr/README.md)** — including the OpenSpec ownership split (behavior → `openspec/specs/`, full design → `openspec/changes/<slug>/design.md`, decision/bet → `docs/adr/`) and the "decisions folded in" pattern for absorbed sub-decisions.
