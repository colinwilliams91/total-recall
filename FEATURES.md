# Features

> The two-minute tour. Setup lives in the [README](README.md).

## What you get today

### 🎯 Recall quizzes on every commit

Total Recall installs Git hooks that fire on your normal workflow — commit, push. The daemon reads the staged diff, caches the engineering concepts you're working with, and generates one short multiple-choice question about them. The question surfaces in your terminal via `torec ask`. Four seconds per Git event boosts your comprehension from C+ -> A+

```
git commit -m "fix: handle retry jitter"   ──>   concept: exponential backoff
                                                 + jitter
                                                 ──> quiz question in your
                                                     terminal
```

### 🖥️ Two delivery surfaces

- **Terminal TUI** — `torec ask` renders the question and grades your answer with a short explanation.
- **MCP** — your AI assistant can pull questions (`recall_next`) and record selections (`recall_select`) from the daemon's MCP endpoint on `localhost:7331`. Quizzes meet you wherever you're already working.

### 🧠 Difficulty that learns you

With the default `difficulty: adaptive`, quizzes start from a lightweight read of each commit's shape, then your answer history takes over:

- **A strong record earns harder questions** — a high recent correctness ratio escalates the difficulty a level; a rough patch eases off before it demoralizes. In between, the level holds.
- **Your misses lead the next quiz** — concepts you keep missing rank ahead of the rest of the feed, so the next question targets your demonstrated weaknesses, not just what you touched last. Weaknesses follow you across projects too: a concept you struggled with on one repo shapes the quiz on another whenever that concept is already on the table.
- **Cold start is gentle** — until a handful of answered questions exist, the commit-shape baseline runs alone; your answers never move the level before there's real evidence.

Every resolution is auditable: the chosen level and its reasons ride the question's event record, and the daemon log names them (`learner-tier`, `cold-start`, `performance focus concepts: …`). Want a fixed level instead? `recall.difficulty: easy | intermediate | hard` pins it — learner adaptation only applies to `adaptive`.

### ✍️ Drop-in prompt customization

Every quiz is shaped by a markdown **policy doc** shipped inside the binary — it's the pedagogy: how questions are framed, what makes a good distractor, what counts as exercising a concept. You can replace it with your own, without recompiling — and you never type an asset name. The four steps:

1. `torec asset sync` — copies the shipped policy into your override slot, naming the file for you and printing its path
2. Edit the file at the printed path (`~/.tr/prompts/question-generation-policy.md` — or under `$TR_HOME` when set)
3. Restart `torec serve` to pick up the change
4. `torec asset show` confirms your override is what's loaded (source `$TR_HOME`, path, age)

**One policy. One file. `sync` names it — never type an asset name. Anything else in the slot is listed `inactive` — present, ignored, cleanable with `torec asset reset <name>`.** (Inside the doc, the front-matter `name:`/`description:` keys are descriptive metadata — the filename is how the asset is addressed.)

**The slot is a deployment target, not a workspace.** Ideas, drafts, forks, and version history belong in git or a policies folder (`~/policies/` works well); the slot holds the one doc that is active. If someone shares a policy doc, deploying it is the same copy over the slot file — policy files are prompts, so review what you import the way you'd review a patch.

Total Recall watches for drift: a startup `OVERRIDE WARNING` fires when your override is older than the shipped policy by more than `prompt-asset.drift-warning-days` (default 90, `0` disables) — the shipped pedagogy improves over time, and a stale override silently freezes yours at the old policy. `torec asset sync --force` re-baselines you onto the current canonical doc; edit and re-apply your tweaks from there.

### 🔍 Observability, not guesswork

`torec config --show` prints every resolved config key annotated with its source (`[user]` / `[repo]` / `[default]`), plus a `prompt assets:` section listing each asset's resolved source, path, and age. When a quiz goes sideways, you can see exactly what's loaded and why.

## Coming soon

### ⏱️ Spaced repetition resurfacing

Concepts you mastered a while ago will come back right when you're about to forget them — *when* to re-ask is the next axis after *what* to emphasize and *how hard*. In active planning.
