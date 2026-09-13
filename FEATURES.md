# Features

> The two-minute tour. Setup lives in the [README](README.md).

## What you get today

### 🎯 Recall quizzes on every commit

Total Recall installs Git hooks that fire on your normal workflow — commit, push. The daemon reads the staged diff, caches the engineering concepts you're working with, and generates one short multiple-choice question about them. The question surfaces in your terminal via `tr ask`. Four seconds per Git event boosts your comprehension from C+ -> A+

```
git commit -m "fix: handle retry jitter"   ──>   concept: exponential backoff
                                                 + jitter
                                                 ──> quiz question in your
                                                     terminal
```

### 🖥️ Two delivery surfaces

- **Terminal TUI** — `tr ask` renders the question and grades your answer with a short explanation.
- **MCP** — your AI assistant can pull questions (`recall_next`) and record selections (`recall_select`) from the daemon's MCP endpoint on `localhost:7331`. Quizzes meet you wherever you're already working.

### ✍️ Drop-in prompt customization

Every quiz is shaped by a markdown **policy doc** shipped inside the binary — it's the pedagogy: how questions are framed, what makes a good distractor, what counts as exercising a concept. You can replace it with your own, without recompiling — and you never type an asset name:

```sh
tr asset sync                # copies the shipped policy into the slot as your starting point
# ...edit the file it places at ~/.tr/prompts/question-generation-policy.md then restart 'tr serve'
tr asset list                # what's loaded: resolved source, path & age
tr asset reset               # remove the override — the shipped default returns on next daemon restart
```

**One policy. One file. `sync` names it.** The slot lives at `~/.tr/prompts/` (or `$TR_HOME/prompts/` when `TR_HOME` is set) and holds exactly one file, named exactly like the shipped asset (`question-generation-policy.md`). Anything else you put in the slot is listed as `inactive` — present on disk, ignored by quizzes — and the daemon mentions it once at startup; `tr asset reset <name>` is the cleanup path for strays.

**The slot is a deployment target, not a workspace.** Ideas, drafts, forks, and version history belong in git or a policies folder (`~/policies/` works well); the slot holds the one doc that is active. Swapping = copying a candidate over the slot file — `sync` refreshes it from the shipped canonical, `reset` empties it. Inside the doc, the front-matter keys (`name:`, `description:`) are descriptive metadata only; the filename is how the asset is addressed (a front-matter `name:` of `generate-quiz-question` is normal and unrelated). If someone shares a policy doc, deploying it is the same copy — policy files are prompts, so review what you import the way you'd review a patch.

Total Recall watches for drift: a startup `OVERRIDE WARNING` fires when your override is older than the shipped policy by more than `prompt-asset.drift-warning-days` (default 90, `0` disables) — the shipped pedagogy improves over time, and a stale override silently freezes yours at the old policy. `tr asset sync --force` re-baselines you onto the current canonical doc; edit and re-apply your tweaks from there.

`reset` and `sync` mutate files only — restart `tr serve` to pick up the change.

### 🔍 Observability, not guesswork

`tr config --show` prints every resolved config key annotated with its source (`[user]` / `[repo]` / `[default]`), plus a `prompt assets:` section listing each asset's resolved source, path, and age. When a quiz goes sideways, you can see exactly what's loaded and why.

## Coming soon

### 📈 Adaptive difficulty

Question difficulty that adapts to you — calibrating from your answer history per concept, so quizzes stay in the zone between trivial and demoralizing. In active development.
