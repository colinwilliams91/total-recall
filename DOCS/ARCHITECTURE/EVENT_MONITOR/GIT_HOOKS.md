# Recommended Git Hook Responsibilities

---

# `pre-commit`

## Purpose
((TODO: maybe changed purpose with Event Monitor Layer now...))
Analyze staged code before commit creation.

This is probably your highest-value hook.

---

## Responsibilities

### Capture

- staged diff
- file types
- LOC delta
- languages
- commit scope heuristics

---

### Trigger Recall Pipeline

Potentially:

- sync
- async
- daemon queue

depending on configuration.

---

### Optional UX

- terminal prompt
- MCP notification
- IDE notification

---

## Important Constraint

Keep this FAST.

Target:

- <500ms without AI
- <2s with AI
- otherwise async/deferred mode

Developers are extremely sensitive to commit latency.

---

# `commit-msg`

## Purpose

Semantic/contextual analysis of intent.

This hook is underrated.

---

## Responsibilities

### Inspect Commit Message

Extract:

- intent
- architecture keywords
- operational semantics

Example:

```
feat: add optimistic locking to payment retries
```

This dramatically improves question quality.

Now you know:

- purpose
- abstraction level
- system intent

—not just raw diff data.

---

## Valuable Future Uses

### Detect Weak Commit Messages
(only prompt this after question answered or skipped. must be fleeting and insignificant as this is low-priority for our tool)

Example:

```
fix stuff
```

Then:

```
🧠 Suggestion:
This commit appears to introduce concurrency coordination.
Consider documenting the reasoning in the commit message.
```

This subtly reinforces engineering rigor.

Very aligned with your mission.

---

# `pre-push`

## Purpose

High-level architectural reflection.

This should NOT fire every tiny question.

Instead:

- aggregate concepts
- broader reasoning
- spaced recall

---

## Responsibilities

### Aggregate Commit Concepts

Example:

```
This push introduced:- caching- retry semantics- distributed locking- optimistic concurrency
```

---

### Generate Higher-Level Recall

Questions here should be:

- architectural
- systems-oriented
- integration-focused

Not syntax-level.

---

## Excellent Use Case

```
Why would optimistic concurrency be preferred over pessimistic locking in a distributed system?
```

That is exactly the type of retention signal your research supports.