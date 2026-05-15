# Total Recall
Preserve engineering intuition in the age of AI agents.

Cognitive retention layer for AI-assisted engineering. Four seconds per Git event is 2 letter-grades of skill retention saved.
## Problem Statement

Aggressive adoption of AI assisted development tooling has proven to have negative impacts on software engineering skill development *unless engineers stay "cognitively engaged"*.

> We find that AI use impairs conceptual understanding, code reading, and debugging abilities... For a 27-point quiz, this translates into a 17% score difference or 2 [letter] grade points.
>
> - Whitepaper [How AI Impacts Skill Formation](https://arxiv.org/pdf/2601.20245)
>   - Judy Hanwen Shen, Alex Tamkin
>
> We found that using AI assistance led to a statistically significant decrease in mastery... AI may accelerate productivity while inhibiting skills formation.
>
> - [How AI assistance impacts the formation of coding skills](https://www.anthropic.com/research/AI-assistance-coding-skills)
> 	- Anthropic Research Team
## Philosophy
Git is the stable substrate.

Git is the operating system layer of software engineering.

That is where we anchor.

We should never feel like "work after the work".

We should feel like "a 4-second mental rep".
## Key Features
- [ ] 1.0 Fast and Interruptible
- [ ] 2.0 Difficulty Configurable
	- [ ] 2.1 Adaptive Recall Difficulty
- [ ] 3.0 Feels Invisible
	- [ ] 3.1 Async/Cache Layers
- [ ] 4.0 Gamification/Hosted Leaderboard
	- [ ] 4.1 see -> 1.0.3
- [ ] 5.0 Concept Fingerprinting
	- [ ] 5.1. Long-term Adaptive Curricula
---
## Roadmap
#### Terminal Prompt MVP
Example:

```
🧠 Recall Check

This commit introduced exponential backoff.

Why is jitter commonly added to retry intervals?

[1] Prevent retry synchronization
[2] Reduce memory usage
[3] Improve cache locality
```

- ### 1.0
	- 1.0.1 `key detail: press enter to skip...`
	- 1.0.2 `can track skips?`
	- 1.0.3 `[stretch goal] gamify streaks?`
```sh
Analyzing staged diff...
Generating recall prompt...

🧠 Recall Check
What problem does debouncing primarily solve?

[1] Race conditions
[2] Excessive repeated invocation
[3] Deadlocks

Press Enter to skip ->
```
- ### 2.0
	- 2.0.1 `pick difficulty level on init`
	- 2.0.2 `can be updated w/ CLI arg. later`
- #### 2.1
	- 2.1.1 `[stretch goal] scoped difficulties/progression`
```sh
Example:

- junior dev gets fundamentals
- senior dev gets architecture tradeoffs
- repeated mistakes become recurring prompts
- concepts decay over time and reappear later

That becomes:

- personalized retention
- engineering cognition telemetry
- real learning reinforcement
```
- ### 3.0
	- 3.0.1 `Install, Init, Code`
- #### 3.1
	- 3.1.1 `[stretch goal] local fallbacks`
```sh
git diff
  ->
lightweight local summarizer
  ->
cache embeddings/concepts
  ->
generate question
```
- #### 3.1 cont.
	- 3.1.2 `local "heuristic" fallback generation`
	- 3.1.3 `"offline mode/fast mode"
	- 3.1.4 `example fallback: regex concept extraction -> "AST parsing" -> "known framework mappings"
- ### 4.0
	- 4.0.1 `multiplayer leaderboard`
	- 4.0.2 `in CLI? makes curl to endpoint?`
- #### 4.1
	- 4.1.1 `see -> 1.0.3 correct answers == streaks`
- ### 5.0
	- 5.0.1. `commit relevant quizzing, e.g.`
```sh
This commit involved:
- memoization
- DFS traversal
- optimistic concurrency
- retry semantics
```
- #### 5.1
	- 5.1.1 `[stretch goal] personalized reinforcement`
```sh
- spaced repetition resurfaces concepts
- weak concepts recur
```
- #### 5.1 cont.
	- 5.1.2 `[stretch goal] long-term memory reinforcement`
```sh
- forgotten concepts reappear
```
## Setup
- Global Install
```sh
brew install total-recall
# or
go install total-recall
# or
npm install -g tota-recall
```
- Per-Repo Config || [stretch goal] User-Based Config
```sh
# ...added to PATH && inside a repo
tr init
# installs Git hooks into
.git/hooks/
# `pre-commit`
# `commit-msg`
# `pre-push`
# TODO: investigate more options...
```
- Scaling Difficulty Default
```sh
# last step in CLI: prompts user if they want to config. difficulty?
y/n?
# YoE?
- 0-2
- 3-5
- 5+
# How many Rounds of Questions do you want per commit?
- 1
- 2
# if 1: R1 || R2
# if 2: R1 => R2
# R1 == Concrete Example
# R2 == Abstract/Theory Example
```
## Data
#### Quotes

> Among participants who use AI, we find a stark divide in skill formation outcomes between high-scoring interaction patterns (65%-86% quiz score) vs low-scoring interaction patterns (24%-39% quiz score). *The high scorers* only asked AI conceptual questions instead of code generation or *asked for explanations to accompany generated code; these usage patterns demonstrate a high level of cognitive engagement.* [emphasis mine]
> - Shen & Tamkin, [How AI Impacts Skill Formation](https://arxiv.org/pdf/2601.20245)

> Together, our results suggest that the aggressive incorporation of AI into the workplace can have negative impacts on the professional development workers if they do not remain cognitatively [sic] engaged. Given time constraints and organizational pressures, junior developers or other professionals may rely on AI to complete tasks as fast as possible at the cost of real skill development. Furthermore, we found that the biggest difference in test scores is between the debugging questions. This suggests that as companies transition to more AI code writing with human supervision, humans may not possess the necessary skills to validate and debug AI-written code if their skill formation was inhibited by using AI in the first place.
> - Shen & Tamkin, [How AI Impacts Skill Formation](https://arxiv.org/pdf/2601.20245)
#### Results

> In the initial coding phase of our qualitative analysis... we developed a typology of six AI interaction patterns... yielding different outcomes for both completion time and skill formation (i.e. quiz score). (figure 11)
> - Shen & Tamkin, [How AI Impacts Skill Formation](https://arxiv.org/pdf/2601.20245)

![figure 11 AI interaction patterns](docs/data/fig_11.png)

The group with in optimal intersection of *lowest* Completion Time and *highest* Quiz Score axes "Generation-Then-Comprehension".
> *High-scoring interaction patterns* were clusters of behaviors where the average quiz score is 65% or higher. *Participants in these clusters used AI both for code generation, conceptual queries or a combination of the two*.
> • Generation-Then-Comprehension (n=2): After their code was generated, they then asked the AI assistant follow-up questions to improve understanding.

> Figure 6 shows that while using AI to complete our coding task did not significantly improve task completion time, *the level of skill formation gained by completing the task, measured by our quiz, is significantly reduced* (Cohen d=0.738, p=0.01). There is a 4.15 point difference between the means of the treatment and control groups. ***For a 27-point quiz, this translates into a 17% score difference or 2 grade points.*** Controlling for warm-up task time as a covariate, the treatment effect remains significant (Cohen’s d=0.725, p=0.016). [emphasis mine]

![figure 6 task completion time over quiz score](docs/data/fig_6.png)

## Contributing
...
