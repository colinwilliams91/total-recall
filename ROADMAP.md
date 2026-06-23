# Roadmap

## Phase 00 — Foundation (Shipped)

- Go binary scaffolding (`total-recall` CLI, `cmd/`, `internal/` package layout)
- Hook script stubs (`hooks/*.sh`, `hooks/*.bat`)
- Go module, Cobra command skeleton, `go.mod`

---

## Phase 01 — Config Architecture (Shipped)

- Two-tier configuration: `~/.tr/config.yaml` (user) + `.tr.yaml` (per-repo)
- `total-recall init` with conversation analysis opt-in (Huh TUI)
- `total-recall config --show` with source annotations and deep-merge
- `EnsureUserConfig` with auto-create and `--quiet` flag
- MCP conversation analysis gate (`privacy.conversation_analysis`)
- Daemon-required architecture; transient mode deferred

---

## Phase 02 — Daemon Foundation (Shipped)

- HTTP daemon at `localhost:7331` (`total-recall serve`)
- Hook routes: `POST /hooks/pre-commit`, `/hooks/commit-msg`, `/hooks/pre-push`
- `GET /health` endpoint; `total-recall status` with exit-code-1 on failure
- Hook installation in `tr init` — Huh prompts, sentinel chaining, idempotent re-runs
- Full hook scripts: P0 credential scan, diff capture, curl dispatch, graceful degradation
- `.bat` variants for Windows environments outside Git Bash
- `HookResponse` typed struct (Phase 3 forward-compatible: `Recall *RecallPrompt omitempty`)
- P0 security: pre-commit blocks commits containing raw `api-key:` values in `.tr.yaml`

---

## Phase 03 — Intelligence Layer (Shipped)

- AI provider interface (`internal/ai/Provider`) — raw HTTP, BYOK-first, no SDK dependencies
- Named provider registry: Anthropic, OpenAI, Groq, Ollama, LM Studio, Custom
- `tr init` AI provider setup TUI — provider select, API key (env: pattern), model name, base URL for custom
- Concept extraction from staged diffs (`internal/pipeline/`) with 8 KB diff guard
- SQLite concept cache (`~/.tr/memory.db`, `modernc.org/sqlite`) — no CGo required
- Recall Engine: question synthesis from recent cached concepts (`internal/recall/`)
- Async pipeline in `handleHook` — hook responds 202 immediately; AI work runs in background goroutine; graceful drain on shutdown via `sync.WaitGroup`
- `Dispatcher` interface + terminal adapter (`internal/presentation/terminal/`) — v1 delivers to daemon stdout
- `DOCS/ARCHITECTURE/DELIVERY.md` — documents v1 limitation and Phase 4 plan

---

## Phase 04 — Out-of-Band Delivery (Shipped: 4A)

### Phase 4A — MCP + Shell (Shipped)

- **MCP server** mounted at `/mcp/` — AI coding agents (Copilot CLI, Claude Code) receive questions via `recall_next` tool, subscribe to `recall://queue` resource, and are guided by the `recall_workflow` prompt
- **REST endpoints**: `GET /recall/next` (atomic dequeue) and `POST /recall/answer` (answer/skip recording)
- **`tr ask` subcommand** — Bubbletea TUI with "Thinking." animation, multiple-choice keypress handler, 30-second timeout; TTY-aware (silent in CI/CD)
- **Post-commit hook** — `tr init` writes `.git/hooks/post-commit` that calls `total-recall ask` after each successful commit
- **`~/.tr/memory.db`** — unified SQLite backing store; `questions` table with exactly-once atomic dequeue (`UPDATE ... RETURNING`); one-time migration guard from `concepts.db`
- **`terminal.Adapter` opt-in** — `presentation.terminal: true` retains daemon-pane delivery for users who prefer it; off by default

### Phase 4C — Answer Feedback Loop (Shipped)

- **Schema extension** — `questions` table gains `correct_index`, `answer_index`, `correct`, `feedback` columns via idempotent `ALTER TABLE ADD COLUMN` migrations
- **Correctness evaluation** — server-side arithmetic (`answer_index == correct_index`); persisted at answer time; `recall.Question.CorrectIndex` (computed at synthesis) is now stored at enqueue, not dropped
- **AI feedback for terminal users** — `POST /recall/answer?feedback=true` triggers a `recall.Engine.GenerateFeedback` call (≤ 150 tokens, plain prose); verdict and feedback render after the alt-screen closes
- **MCP self-explanation** — `recall_next` returns `correct_index` to AI agents; `recall_answer` evaluates and returns `correct`/`correct_index`/`correct_text` but skips the AI call; agents explain using their own knowledge per the updated `recall_workflow` prompt
- **Skip path** — `POST /recall/answer` with `{"skip": true}` records `answer = "skip"`, no evaluation, no AI call; tr ask renders `→ Question saved for later.`
- **Enriched `recall://recent`** — MCP resource includes `correct_index`, `answer_index`, `correct`, `feedback` for each row (NULL for skipped/MCP rows)
- **Foundation for spaced repetition** — `memory.db` now carries the answer history needed for future difficulty progression and recall debt

### Phase 4B — VS Code Extension (Next)

- VS Code extension surfaces questions as workspace notifications with clickable answer choices
- Polls `GET /recall/next`; uses VS Code Notifications API (`window.showInformationMessage`)
- Daemon autostart: `tr init` will offer launchd/systemd/Task Scheduler entry so `tr serve` starts on reboot

### Phase 4D — Extended AI Providers (Planned)

- **Qwen** (Alibaba Cloud Model Studio) — OpenAI-compatible via DashScope; default model `qwen3.7-max`
- **MiniMax** — OpenAI-compatible; default model `MiniMax-M3`
- **DeepSeek** — OpenAI-compatible; default model `deepseek-v4-pro`
- **OpenRouter** — OpenAI-compatible unified model catalog; default model `deepseek/deepseek-v4-flash:free` (free tier)
- All four route through existing `openai.New()` adapter — no new adapter packages required
- `tr init` TUI updated with new provider options and API key placeholders

---

- ### Phase 1 - MVP
	- ARCHITECTURE [INDEX](00-SRC/🔓_OSS/🦾TOTAL_RECALL/📐ARCHITECTURE/INDEX.md)
#### KEY FEATURES:
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
- OR
```sh
Commit accepted.

🧠 1 recall check available.
```
- #### 1.1
	- 1.1.1 `[stretch goal] optional strict mode block commits`
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
- #### 4.2
	- 4.2.1 `[stretch goal] recall debt`
	- 4.2.2 `skipped questions accumulate "debt"`
	- 4.2.3 `resurfaced later
	- 4.2.4 `daily reinforcement queue/spaced repetition model`
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

---
## Future Scope
- Phase 2:
	- Terminal UI/UX (TUI)
		- can this use the same API the MCP server exposes?
- Phase 3:
	- VS Code extension
		- manages:
			- UX
			- rendering
			- lifecycle management
			- state synchronization
			- editor events
			- authentication flows
			- extension API quirks
			- webviews
			- notifications
			- local IPC
			- platform compatibility

---

## Hooks Phase — P0 Security Requirement

### `.tr.yaml` credential scan in pre-commit hook
- **Priority**: P0 — must ship with the managed hook system
- **Problem**: `.tr.yaml` is committed to the repository. A user who accidentally puts an `api-key` value in `.tr.yaml` (instead of `~/.tr/config.yaml`) will commit it without any pre-commit safeguard until hooks are installed.
- **Current mitigation**: `LoadRepoConfig` emits a 🚨 security warning with rotation instructions at runtime (after the commit has already happened).
- **Required fix**: The managed `pre-commit` hook MUST scan `.tr.yaml` for:
  - Any `ai:` block
  - Any `privacy:` block
  - Specifically, any `api-key:` value that is not in `env:<VAR_NAME>` format
- **Exit behavior**: If a raw key is detected, the hook MUST block the commit (`exit 1`) with a clear message directing the user to move credentials to `~/.tr/config.yaml`.

---

## Deferred — Community Request Only

### Transient mode (hooks without a running daemon)
- **Status**: Deferred indefinitely
- **Current behavior**: Hooks require `tr serve` to be running. If the daemon is not running, the hook prints an advisory and exits 0 — the Git operation proceeds unblocked.
- **Revisit trigger**: Community demand, e.g. CI/CD pipeline use cases where a persistent daemon is impractical
- **If ever implemented, MUST**:
  - Warn clearly: "Running without daemon — expect slower analysis and extra AI provider round-trips."
  - Strongly recommend `tr serve` for optimal performance and a warm cache
  - Never be the documented default or primary installation path

### Daemon autostart (`tr init` enhancement)
- **Status**: Future Phase 1 task — implement after `config-architecture` is complete
- `tr init` should offer to configure daemon autostart so `tr serve` starts automatically after reboot:
  - macOS: launchd plist (`~/Library/LaunchAgents/`)
  - Linux: systemd user unit (`~/.config/systemd/user/`)
  - Windows: Task Scheduler entry or startup folder shortcut
- Ensures developers don't have to remember to run `tr serve` each session