## Context

Total Recall's current config model is a single per-repo `.tr.yaml`. As the system grows to include MCP conversation signal capture and BYOK AI credentials, a single flat config creates security and UX problems: API keys duplicate across repos and risk being committed, and privacy preferences (whether to allow conversation analysis) are personal decisions that shouldn't need to be set for every project independently.

The Core Go Engine already runs in two lifecycle modes (daemon via `tr serve`, transient via `tr hook`). It must load config at startup in both modes. The design adds a second config source — `~/.tr/config.yaml` — and defines how the two are merged.

## Goals / Non-Goals

**Goals:**
- Define the schema and location of `~/.tr/config.yaml`
- Define which config keys belong at user-level vs. per-repo
- Define the merge/override precedence model
- Gate MCP conversation signal capture behind `privacy.conversation_analysis`
- Keep the config system simple — no dynamic reloading, no complex inheritance

**Non-Goals:**
- Organization/team-level config (future Phase)
- Config validation UI or config doctor tooling (future)
- Encrypting the config file at rest
- Supporting multiple AI providers simultaneously per session

## Decisions

### 1. Two-tier flat merge: user defaults → per-repo overrides

**Decision**: Load `~/.tr/config.yaml` first, then deep-merge `.tr.yaml` on top. Per-repo values win on any conflict.

**Rationale**: Simple and predictable. Developers understand "project config overrides user config." No complex inheritance chains, no conditional logic per key.

**Alternative considered**: Separate config domains with no override (user keys and repo keys are disjoint, no merging). Rejected because it prevents legitimate per-repo recall tuning (e.g., a practice repo where the user wants more questions than their global default).

```
Load order:
  ~/.tr/config.yaml     ← user defaults (required at init)
         ↓ deep-merged, per-repo wins on conflict
  .tr.yaml              ← per-repo overrides (optional per key)
         ↓
  Resolved config used by Core Engine
```

### 2. Key ownership: which config lives where

**Decision**:

```yaml
# ~/.tr/config.yaml — personal, never committed
privacy:
  conversation_analysis: false  # opt-in, default off

ai:
  provider: anthropic
  model: claude-sonnet
  api-key: env:ANTHROPIC_API_KEY

recall:
  difficulty: adaptive
  max_questions: 1

# .tr.yaml — project-specific, safe to commit
hooks:
  pre-commit: true
  commit-msg: false
  pre-push: true

mode:
  blocking: false

presentation:
  terminal: true
  mcp: true

# recall: can appear here to override user defaults
#   recall:
#     max_questions: 2
```

**Rationale**:
- `privacy` is a personal stance, not a project stance. Placing it in `.tr.yaml` would mean opting in separately for every repo.
- `ai` credentials are personal. Putting API keys in `.tr.yaml` risks accidental commits and requires duplication across repos.
- `hooks`, `mode`, `presentation` are legitimately project-specific — different repos may want different hook configurations.
- `recall` defaults to user-level but is overridable per-repo to support specialized repos (e.g., learning projects with higher question frequency).

### 3. `privacy.conversation_analysis` gates Signal 1 and 2 only

**Decision**: The flag controls whether MCP conversation content (user's question to agent + agent's explanation) is processed as input signals by the Incremental Analysis Pipeline. It does NOT gate code context (Signal 3).

**Rationale**: Code context visible to the agent is the same code already being observed by the Filesystem and Git Index watchers. The background runtime captures it regardless. Gating it separately would be confusing and provide no real privacy benefit. The genuine privacy surface is the conversation text — what you asked, what the agent said.

```
conversation_analysis: false (default)
  MCP as output:   ✓ recall prompts delivered via MCP
  Signal 3:        ✓ code context already in cache
  Signal 1+2:      ✗ conversation text not processed

conversation_analysis: true (opt-in)
  MCP as output:   ✓ recall prompts delivered via MCP
  Signal 3:        ✓ code context already in cache
  Signal 1+2:      ✓ conversation intent + content feeds pipeline
                      (concepts extracted, raw text discarded)
```

### 4. Extract-and-discard: conversation text never persists

**Decision**: When `conversation_analysis: true`, conversation content passes through the Incremental Analysis Pipeline for concept extraction only. The raw conversation text is never written to the Background Concept Cache. Only extracted concept fingerprints and confidence scores persist.

**Rationale**: Conversation content may include sensitive context (business logic, partial credentials, proprietary reasoning). Storing only the structured output (concept tags, confidence, grounding reference) preserves the learning signal without retaining the raw material.

## Risks / Trade-offs

- **User forgets to `tr init` user config** → The engine should fail with a clear message pointing to `tr init --global` rather than silently using empty defaults. Risk of silent misconfiguration.
  → Mitigation: Core Engine validates presence of `~/.tr/config.yaml` at startup and prompts init if absent.

- **API key in env var vs plaintext** → `api-key: env:ANTHROPIC_API_KEY` is the documented pattern, but users may be tempted to paste the key directly.
  → Mitigation: Document the env var pattern prominently. Consider a warning if a raw key string is detected at load time.

- **Per-repo recall overrides cause confusion** → If a user sets `max_questions: 1` globally but a repo sets `max_questions: 3`, the behavior feels inconsistent across projects.
  → Mitigation: Clear documentation of the override chain. Future: `tr config --show` to display resolved config.

- **`~/.tr/config.yaml` doesn't exist on first install** → Transient hook invocations may occur before `tr serve` has ever run.
  → Mitigation: `tr init` creates the user config with safe defaults. Hooks check for config presence and fail gracefully if absent.

### 5. `tr init` creates the user config and prompts for conversation analysis

**Decision**: `tr init` always creates `~/.tr/config.yaml` (no `--global` flag required). It also prompts the user once about conversation analysis opt-in during setup, in plain language.

**Rationale**: The user config is personal infrastructure, not optional scaffolding. Requiring a separate `--global` flag adds friction and discoverability problems. Creating it on every `tr init` (idempotently — no overwrite if already present) makes it a first-class part of the install experience. The single prompt at init is the natural moment to communicate the privacy contract without burying it in documentation.

**Tone**: The prompt must be simple and earnest — this project has no interest in collecting user data and that should come through clearly. No legal hedging, no dark patterns, no buried defaults. Example:

```
🧠 One quick question before we finish:

  Would you like Total Recall to analyze your AI assistant
  conversations to generate smarter quiz questions?

  When enabled, we look at what you and your AI discuss
  and extract the concepts — nothing else is kept.
  You can change this anytime in ~/.tr/config.yaml.

  Enable conversation analysis? [y/N]:
```

Default is N (off). The user must affirmatively opt in.

**Alternative considered**: Show this prompt on first use of MCP rather than at init. Rejected because surprising the user mid-workflow is worse UX than a clear one-time setup question.

### 6. User config is auto-created with safe defaults if init is bypassed

**Decision**: If the Core Engine starts (daemon or transient hook) and `~/.tr/config.yaml` does not exist, it SHALL create the file with safe defaults silently, then emit a single advisory message.

**Rationale**: Hooks fire automatically as part of Git workflows. A hard failure because the user forgot to run `tr init` would be disruptive and surprising. Safe-default auto-creation is a better experience. The advisory message makes the situation visible without blocking anything.

```
⚠  No Total Recall config found.
   Created ~/.tr/config.yaml with safe defaults.
   Run `tr init` to configure your preferences.
```

`conversation_analysis` defaults to `false` in all auto-created configs. The init prompt is the only place the user is actively asked.

### 7. Per-repo config cannot touch user config keys

**Decision**: `privacy.*` and `ai.*` keys in `.tr.yaml` are silently ignored with a logged warning. These namespaces are user-level only and cannot be overridden per project.

**Rationale**: Privacy is a personal stance. A repo should never be able to opt a developer into conversation analysis, even accidentally. AI credentials are personal and belong in the user config — a project-committed `.tr.yaml` should not carry them.

### 8. Advisory message on auto-creation is suppressible via `--quiet`

**Decision**: The advisory message emitted when `~/.tr/config.yaml` is auto-created SHALL be suppressible by passing `--quiet` to any `total-recall` command. `--quiet` is a global persistent flag that suppresses all non-error advisory output.

**Rationale**: Power users integrating Total Recall into automated workflows (CI scripts, dotfile bootstrappers) may call `tr serve` or `tr hook` headlessly. A non-suppressible advisory would produce noise in those contexts. `--quiet` is a well-understood CLI convention for this purpose.
