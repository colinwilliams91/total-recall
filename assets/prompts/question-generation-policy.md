---
name: generate-quiz-question
description: Generate short quiz question when Recall Engine prompts Question Synthesis. Quiz Questions purpose are to strengthen software developer's skill formation and retention when using AI-assisted development. Content will be contextualized by the Incremental Analysis Pipeline which uses the Event Monitor to track Filesystem changes and the Git Index. Use this context to keep the Quiz Questions relevant.
---

## Goals

- restore cognitive engagement
- force retrieval
- reinforce conceptual understanding
- preserve debugging intuition
- enable code explanation and comprehension
- reduce erosion of systems reasoning and modeling skills

## Your quizzes should disproportionately target:

- debugging
- causal reasoning
- architectural tradeoffs
- runtime implications
- failure analysis
- “why did this break?”
- concurrency hazards
- side effects
- state transitions
- conceptual inquiry
- async semantics

—not trivia.

## GOOD PROMPTS

Questions like:

- “Why is this pattern used?”
- “What bug would occur if X changed?”
- “What problem does this abstraction solve?”
- “Why is this async primitive safer than Y?”
- “What invariant is being preserved here?”
- “What edge case is this retry logic defending against?”
- “Why would this implementation deadlock?”
- “What makes this algorithm O(n log n) instead of O(n²)?”

These induce:

- retrieval
- reasoning
- conceptual reconstruction

## BAD PROMPTS

Avoid:

- syntax trivia
- rote API recall
- variable-name memory tests
- “what function was used?”
- shallow fill-in-the-blank

## Error Encounter Was Essential For Learning

The following challenges have proven to improve skill formation:

- more runtime failures
- more technology-specific errors
- more debugging friction

That means:

Your questions should intentionally surface “counterfactual debugging.”

Example:

```
What runtime issue would occur if this await were removed?
```

or:

```
Why would this retry loop potentially create a thundering herd problem?
```

You are recreating the learning value of encountering errors
without necessarily requiring the user to manually suffer through them.

## Pure AI-Delegation Produced The Worst Outcomes

This is your strongest anti-pattern category.

Consider if any of these signals are present when generating questions:

- massive generated diffs
- low manual edit ratios
- direct paste patterns
- AI-generated commit signatures
- large unexplained abstractions

And then:
increase conceptual questioning depth.

## AI-Assisted Reflective Reinforcement

You should not only quiz--you should sometimes explain.

Data-backed thesis: Generation-Then-Comprehension proves to boost comprehension and retention of skill development.

### Potential Product Mechanic

After commit:

```
🧠 Recall Check

This commit introduced optimistic concurrency.

Question:
Why is optimistic locking preferred here over a mutex?

[answer]

Want a deeper explanation? [y/n]
```

That aligns almost perfectly with the successful behavioral patterns from leading research into our topic.

## PRIORITIZE

### Conceptual Understanding

Questions should ask:

- why
- tradeoffs
- invariants
- failure modes
- architecture reasoning

---

### Debugging Reasoning

Strong emphasis on:

- runtime failures
- async behavior
- state corruption
- race conditions
- retries
- side effects
- edge cases

The paper strongly suggests debugging skill is the most vulnerable.

---

### Counterfactual Thinking

Ask:

- what breaks if…
- why not use…
- what edge case…
- how could this fail…

This recreates the learning value of independent debugging.

---

### Retrieval Over Recognition

Avoid:

- yes/no
- multiple-choice-only
- shallow recognition

Prefer:

- short recall
- explanation
- reasoning

---

### Explain Generated Abstractions

If AI introduced:

- patterns
- frameworks
- concurrency
- caching
- retry systems

…force conceptual reflection afterward.

# DE-EMPHASIZE

### Syntax Memorization

The paper explicitly avoided syntax-heavy evaluation because AI already trivializes it.

---

### Passive Reading

Reading alone produced weaker retention than explanation + reasoning.

---

### Pure Code Generation Prompts

Avoid asking:

- “what does this function do?”

Prefer:

- “why is this implementation structured this way?”