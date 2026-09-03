<p align="center">
	<img src="DOCS/MEDIA/TOTAL_RECALL_LOGO_04.png" alt="total recall logo" width="400"/>
</p>

<p align="center">
    <a href="https://github.com/colinwilliams91/total-recall/actions/workflows/ci.yml">
        <img src="https://github.com/colinwilliams91/total-recall/actions/workflows/ci.yml/badge.svg" alt="CI">
    </a>
    <a href="https://github.com/colinwilliams91/total-recall/releases/latest">
        <img src="https://img.shields.io/github/v/release/colinwilliams91/total-recall" alt="Latest Release">
    </a>
    <a href="https://github.com/colinwilliams91/total-recall/blob/main/LICENSE">
        <img src="https://img.shields.io/github/license/colinwilliams91/total-recall" alt="License">
    </a>
    <a href="https://github.com/colinwilliams91/total-recall/blob/main/go.mod">
        <img src="https://img.shields.io/github/go-mod/go-version/colinwilliams91/total-recall" alt="Go Version">
    </a>
</p>

> _AI coding assistants make us faster while we slowly forget the fundamentals._
>
> _Total-Recall reinforces software engineering knowledge through short, diff-aware quizzes triggered by your normal Git workflow._

***AI can write the code. Total-Recall makes sure you're still learning from it.***

_The cognitive retention layer for AI-assisted engineering. Four seconds per Git event is two letter-grades of skill retention saved._

## Setup

**Prerequisites:**
- Git 2.5+ (2015) for linked worktree support¹.
- Go 1.25.5+ to install binary -- _or see below for non-Go install path._
- LLM access via API key or local.

```sh
# Install the binary    # (one-time, user-level)
go install github.com/colinwilliams91/total-recall@latest

# Init user config      # (run anywhere; one-time, user-level)
tr init                 # creates `~/.tr/config.yaml` for conversation analysis & AI provider setup

# Start the daemon
tr serve				# runs on `localhost:7331` & must be running for hooks & MCP

# New terminal
tr status				# Check daemon status

# Init in a repo
cd your-project/
tr repo					# adds project `.tr.yaml`, installs Git hooks
```

Re-run `tr repo` anytime to change hook selections or update hook scripts. Existing unmanaged hooks are chained — not overwritten.

### Non-Go install path¹

Without Go: download the release archive from GitHub Releases, extract, place `tr` (or `tr.exe`) on PATH manually. Same downstream flow.

## Configuration

Total-Recall uses two config files with clear separation of concerns:

**Inspect the resolved config**

```sh
tr config --show
```

Prints every key annotated with its source (`[user]` / `[repo]` / `[default]`).

Checkout [CONFIG.md](DOCS/ARCHITECTURE/CONFIG.md) for full deep-merge rules.

## Philosophy
You're a software engineer.

AI writes faster. You think deeper.

Don't forget how to design good software.

Every commit is a learning opportunity.

## Problem Statement

**Aggressive adoption of AI assisted development tooling proves to have negative impacts on software engineering skill development _unless engineers stay "cognitively engaged"_.**

> 84% of developers are using AI tools this year.
- [Stack Overflow's developer survey 2025](https://survey.stackoverflow.co/2025/ai#sentiment-and-usage-ai-select-ai-select)

> We find that AI use impairs conceptual understanding, code reading, and debugging abilities... For a 27-point quiz, this translates into a 17% score difference or 2 [letter] grade points.
- [How AI Impacts Skill Formation](https://arxiv.org/pdf/2601.20245), arXiv - Cornell University
	- Judy Hanwen Shen, Alex Tamkin

> We found that using AI assistance led to a statistically significant decrease in mastery... AI may accelerate productivity while inhibiting skills formation.
- [How AI assistance impacts the formation of coding skills](https://www.anthropic.com/research/AI-assistance-coding-skills)
	- Anthropic Research Team

Checkout [DATA.md](DOCS/DATA.md) for more information on the research findings.

---
---

## Contributing
- I strongly believe in the sharing of knowledge, transparent information and FOSS.
- _Learning_ is what will distinguish us from the robots. 🥲
- Contributions are welcome!
- Please see [CONTRIBUTING.md](DOCS/CONTRIBUTING.md) for development workflow, how to run the automated tests and the manual `tr init` test.

---

#### Footnotes
¹ <sup>`tr repo` uses `git rev-parse --git-path hooks` under the hood, a Git 2.5+ feature</sup>

<img src="https://img.shields.io/liberapay/receives/colin-williams-dev.svg?logo=liberapay">

https://liberapay.com/colin-williams-dev/