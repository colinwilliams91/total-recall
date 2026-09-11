# Config Architecture

Total Recall uses a two-tier configuration model: a personal user config and an optional per-repo config. The two are deep-merged at runtime, with the per-repo config winning on any key both define.

---

## Two-Tier Config Files

| File | Scope | Committed? | Created by |
|---|---|---|---|
| `~/.tr/config.yaml` | User-level: privacy, AI credentials, recall defaults | **No** | `total-recall init` (or auto-created on first `tr serve`) |
| `.tr.yaml` | Per-repo: hooks, mode, presentation, optional recall overrides | **Yes** (safe to commit) | Developer — created manually or via future `tr init --repo` |

---

## User Config (`~/.tr/config.yaml`)

Created by `total-recall init`. Auto-created with safe defaults if absent when `total-recall serve` starts (with an advisory message, suppressible via `--quiet`).

```yaml
privacy:
  conversation_analysis: false   # opt-in: set during `total-recall init`

ai:
  provider: anthropic
  model: claude-sonnet
  api-key: env:ANTHROPIC_API_KEY  # use env:<VAR_NAME> — never paste raw keys

recall:
  difficulty: adaptive
  max_questions: 1
```

### `ai.api-key` pattern

Always reference API keys via an environment variable:

```yaml
api-key: env:ANTHROPIC_API_KEY
```

If a raw string is detected, Total Recall emits a warning. The value `env:ANTHROPIC_API_KEY` is stored as-is; the env var is resolved lazily at the point of use so that `total-recall config --show` never prints secrets.

---

## Per-Repo Config (`.tr.yaml`)

Optional. Safe to commit. Controls project-specific behaviour.

```yaml
hooks:
  pre-commit: true
  commit-msg: false
  pre-push: false

mode:
  blocking: false

presentation:
  terminal: true
  mcp: false

# Optional: override user recall defaults for this repo
recall:
  max_questions: 3
```

### Keys that are user-level only

`privacy.*` and `ai.*` are **always discarded** if present in `.tr.yaml`, with a warning printed to stderr. These keys contain personal privacy choices and credentials that must never be committed to a repository.

---

## Deep-Merge Rules

1. User config is loaded first and provides all defaults.
2. If `.tr.yaml` exists, its keys override the user defaults — but only for the keys it explicitly sets.
3. `recall` is deep-merged at the field level: a repo override of `recall.max_questions` does not discard the user's `recall.difficulty`.
4. `privacy.*` and `ai.*` in `.tr.yaml` are silently discarded with a warning.
5. If `.tr.yaml` is absent, all hook/mode/presentation values remain at their zero defaults.

---

## Inspecting the Resolved Config

```sh
total-recall config --show
```

Prints the fully resolved config with inline source annotations:

```
privacy:
  conversation_analysis: false  # user
ai:
  provider: anthropic  # user
  model: claude-sonnet  # user
  api-key: env:ANTHROPIC_API_KEY  # user
recall:
  difficulty: adaptive  # user
  max_questions: 3  # repo
hooks:
  pre-commit: true  # repo
  commit-msg: false  # repo
  pre-push: false  # repo
mode:
  blocking: false  # repo
presentation:
  terminal: true  # repo
  mcp: false  # repo
```

Sources:
- `user` — value came from `~/.tr/config.yaml`
- `repo` — value came from `.tr.yaml`
- `default` — key is not configured in either file; zero/safe default applies

---

## Config Loading Lifecycle

1. On `total-recall serve`:
   - Call `EnsureUserConfig(quiet)` → load `~/.tr/config.yaml`, or auto-create with safe defaults.
   - Load `.tr.yaml` from the current directory (returns nil if absent — not an error).
   - `Merge(user, repo)` produces the resolved `Config`.

2. On `total-recall init`:
   - Load existing user config if present, otherwise start from `DefaultUserConfig()`.
   - Present the conversation analysis opt-in prompt (Huh TUI).
   - Write the result to `~/.tr/config.yaml` (mode 0600, dir mode 0700).
   - **Never overwrites** existing keys the user hasn't touched — prompts reflect current values.

3. On `total-recall config --show`:
   - Calls `Load(quiet)` which performs the full EnsureUserConfig + LoadRepoConfig + Merge pipeline.
   - Passes the result to `Show(cfg, os.Stdout)` with source annotations.

---

## Related Architecture

- [Daemon / HTTP Server](DAEMON/INDEX.md) — consumes the resolved config at startup
- [Event Monitor / Git Hooks](EVENT_MONITOR/GIT_HOOKS.md) — hooks send HTTP POST to `:7331`, config controls which hooks are active
- [MCP Adapter](MCP_ADAPTER.md) — `presentation.mcp` and `privacy.conversation_analysis` gate MCP features
