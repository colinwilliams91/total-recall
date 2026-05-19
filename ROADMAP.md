# Roadmap

## Phase 00 — Foundation (Shipped)

- Go binary scaffolding (`total-recall` CLI)
- Two-tier configuration: `~/.tr/config.yaml` (user) + `.tr.yaml` (per-repo)
- `total-recall init` with conversation analysis opt-in (Huh TUI)
- `total-recall serve` with auto-config creation and `--quiet` flag
- `total-recall config --show` with source annotations
- MCP conversation analysis gate (`privacy.conversation_analysis`)
- Daemon-required architecture; transient mode deferred

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