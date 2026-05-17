<p align="center">
	<img src="DOCS/MEDIA/TOTAL_RECALL_LOGO_03.png" alt="total recall logo" width="400"/>
</p>

Preserve engineering intuition in the age of AI agents.

Cognitive retention layer for AI-assisted engineering. Four seconds per Git event is two letter-grades of skill retention saved.

## Problem Statement

Aggressive adoption of AI assisted development tooling has proven to have negative impacts on software engineering skill development *unless engineers stay "cognitively engaged"*.

> 84% of developers are using AI tools this year.
- [Stack Overflow's developer survey 2025](https://survey.stackoverflow.co/2025/ai#sentiment-and-usage-ai-select-ai-select)

> We find that AI use impairs conceptual understanding, code reading, and debugging abilities... For a 27-point quiz, this translates into a 17% score difference or 2 [letter] grade points.
- [How AI Impacts Skill Formation](https://arxiv.org/pdf/2601.20245), arXiv - Cornell University
	- Judy Hanwen Shen, Alex Tamkin

> We found that using AI assistance led to a statistically significant decrease in mastery... AI may accelerate productivity while inhibiting skills formation.
- [How AI assistance impacts the formation of coding skills](https://www.anthropic.com/research/AI-assistance-coding-skills)
	- Anthropic Research Team
## Philosophy
Git is the stable substrate.

That is where we anchor.

We should never feel like "work after the work".

We should feel like "a 4-second mental rep".
## Key Features
- [ ] 1.0 Fast and Interruptible
	- [ ] 1.1 Optional Strict Mode
- [ ] 2.0 Difficulty Configurable
	- [ ] 2.1 Adaptive Recall Difficulty
- [ ] 3.0 Feels Invisible
	- [ ] 3.1 Async/Cache Layers
- [ ] 4.0 Gamification/Hosted Leaderboard
	- [ ] 4.1 see -> 1.0.3
	- [ ] 4.2 "Recall Debt"
- [ ] 5.0 Concept Fingerprinting
	- [ ] 5.1. Long-term Adaptive Curricula
---
## Setup
- Global Install
```sh
# Homebrew (recommended)
brew install total-recall

# Go
go install github.com/total-recall/tr@latest

# npm wrapper (optional)
npm install -g total-recall
```
- Start Local Daemon/MCP Server
```sh
tr serve
.
..
...
🧠 Total Recall Engine Running
MCP Server: localhost:7331
Git Event Listener: Active
```
- Per-Repo Config
	- OR
- [stretch goal] User-Based Config
	- `~/.tr/config.yaml` (see below config example. maybe add `ai-provider-api-key` K/V)
```sh
# ...added to PATH && inside a repo
tr init
# installs Git hooks into
.git/hooks/
# TODO: `STAGE HOOK?` THIS SHOULD TRIGGER AI IN BACKGROUND?
# `pre-commit`
# `commit-msg`
# `pre-push`
# TODO: investigate more options...

# default to x, y, z, hooks but be configurable:
tr hooks enable pre-commit
tr hooks disable pre-push
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
- **Alternative Per-Repo Config *File***
```sh
.tr.yaml
```
- Example config:
```YAML
hooks:
  pre-commit: true
  commit-msg: true
  pre-push: true

mode:
  blocking: false

presentation:
  terminal: true
  mcp: true

ai:
  provider: anthropic
  model: claude-sonnet
  api-key: env:ANTHROPIC_API_KEY

recall:
  difficulty: adaptive
  max_questions: 1
```
## Data

- ## 84% of respondents are using AI tools this year
	- According to Stack Overflow's [2025 developer survey](https://survey.stackoverflow.co/2025/ai#sentiment-and-usage-ai-select-ai-select)

#### Quotes

> Among participants who use AI, we find a stark divide in skill formation outcomes between high-scoring interaction patterns (65%-86% quiz score) vs low-scoring interaction patterns (24%-39% quiz score). *The high scorers* only asked AI conceptual questions instead of code generation or *asked for explanations to accompany generated code; these usage patterns demonstrate a high level of cognitive engagement.* [emphasis mine]
> - Shen & Tamkin, [How AI Impacts Skill Formation](https://arxiv.org/pdf/2601.20245)

> Together, our results suggest that the aggressive incorporation of AI into the workplace can have negative impacts on the professional development workers if they do not remain cognitatively [sic] engaged. Given time constraints and organizational pressures, junior developers or other professionals may rely on AI to complete tasks as fast as possible at the cost of real skill development. Furthermore, we found that the biggest difference in test scores is between the debugging questions. This suggests that as companies transition to more AI code writing with human supervision, humans may not possess the necessary skills to validate and debug AI-written code if their skill formation was inhibited by using AI in the first place.
> - Shen & Tamkin, [How AI Impacts Skill Formation](https://arxiv.org/pdf/2601.20245)
#### Results

> In the initial coding phase of our qualitative analysis... we developed a typology of six AI interaction patterns... yielding different outcomes for both completion time and skill formation (i.e. quiz score). (figure 11)
> - Shen & Tamkin, [How AI Impacts Skill Formation](https://arxiv.org/pdf/2601.20245)

![figure 11 AI interaction patterns](DOCS/MEDIA/DATA/FIG_11.png)

The group with in optimal intersection of *lowest* Completion Time and *highest* Quiz Score axes "Generation-Then-Comprehension".
> *High-scoring interaction patterns* were clusters of behaviors where the average quiz score is 65% or higher. *Participants in these clusters used AI both for code generation, conceptual queries or a combination of the two*.
> • Generation-Then-Comprehension (n=2): After their code was generated, they then asked the AI assistant follow-up questions to improve understanding.

> **Figure 6** shows that while using AI to complete our coding task did not significantly improve task completion time, *the level of skill formation gained by completing the task, measured by our quiz, is significantly reduced* (Cohen d=0.738, p=0.01). There is a 4.15 point difference between the means of the treatment and control groups. ***For a 27-point quiz, this translates into a 17% score difference or 2 grade points.*** Controlling for warm-up task time as a covariate, the treatment effect remains significant (Cohen’s d=0.725, p=0.016). [emphasis mine]

![figure 6 task completion time over quiz score](DOCS/MEDIA/DATA/FIG_6.png)

## Contributing
...
