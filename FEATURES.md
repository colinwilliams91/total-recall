# Features

> The two-minute tour. Setup lives in the [README](README.md).

## What you get today

### 🎯 Recall quizzes on every commit

Total Recall installs Git hooks that fire on your normal workflow — commit, push. The daemon reads the staged diff, caches the engineering concepts you're working with, and generates one short multiple-choice question about them. Four seconds per Git event; the question surfaces in your terminal via `tr ask`.

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

Every quiz is shaped by a markdown **policy doc** shipped inside the binary — it's the pedagogy: how questions are framed, what makes a good distractor, what counts as exercising a concept. You can replace it with your own, per domain, without recompiling:

```sh
tr asset sync question-generation-policy   # copy the shipped policy into ~/.tr/prompts/ as your starting point
# ...edit it to taste, then restart 'tr serve'
tr asset list                              # every asset: resolved source, path & age
tr asset reset [name]                      # remove an override — defaults take effect on next daemon restart
```

The override slot is `~/.tr/prompts/<name>.md` (or `$TR_HOME/prompts/` when `TR_HOME` is set).

**The filename is the address.** The slot holds exactly one file per shipped asset, named exactly like it — today that's `question-generation-policy.md`. Don't name files by hand: `tr asset sync question-generation-policy` creates the correctly-named file for you. `tr asset list` prints the finite set of valid names, and tags anything in the slot that doesn't match a shipped asset as `inactive` — present on disk, but never loaded, so it never affects your quizzes. (The daemon also mentions unmanaged files once at startup.)

**The slot is a deployment target, not a workspace.** Ideas, drafts, forks, and version history belong in git or a policies folder (`~/policies/` works well); the slot holds the one doc that is active. Swapping = copying a candidate over the slot file — `sync` refreshes it from the shipped canonical, `reset` empties it. Inside the doc, the front-matter keys (`name:`, `description:`) are descriptive metadata only — the filename is how the asset is addressed (a front-matter `name:` of `generate-quiz-question` is normal and unrelated).

Total Recall watches for drift: a startup `OVERRIDE WARNING` fires when your override is older than the shipped policy by more than `prompt-asset.drift-warning-days` (default 90, `0` disables) — the shipped pedagogy improves over time, and a stale override silently freezes yours at the old policy. `tr asset sync` re-baselines you onto the current canonical doc; edit and re-apply your tweaks from there.

`reset` and `sync` mutate files only — restart `tr serve` to pick up the change.

### 🔍 Observability, not guesswork

`tr config --show` prints every resolved config key annotated with its source (`[user]` / `[repo]` / `[default]`), plus a `prompt assets:` section listing each asset's resolved source, path, and age. When a quiz goes sideways, you can see exactly what's loaded and why.

## Coming soon

### 📈 Adaptive difficulty

Question difficulty that adapts to you — calibrating from your answer history per concept, so quizzes stay in the zone between trivial and demoralizing. In active development.

## Community

### Share your policy doc

The prompt policy is just markdown — which means it's shareable. A game-dev team can trade an ECS-focused policy; a DBA can circulate a query-plan one. Because the filename is the address, a shared doc deploys by copy: save it in your policies folder, then copy it over the slot file (`tr asset sync question-generation-policy` gives you the correctly-named target to overwrite).

**Policy files are prompts.** A policy doc you import shapes every question you're asked, so review what you import the way you'd review a patch — read it before you deploy it, and `tr asset list` will always show you what's actually loaded.
