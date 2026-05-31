# Recall Question Delivery — Architecture

## Overview

Total Recall generates recall questions asynchronously after every Git hook event. The question is synthesized by the AI provider and must reach the developer's terminal. This document describes the v1 delivery mechanism and the Phase 4 plan for true out-of-band delivery.

## v1 Delivery (Phase 3 — Terminal Adapter)

In Phase 3, the `terminal.Adapter` implements the `Dispatcher` interface by writing the question to **daemon stdout** (the process started by `total-recall serve`).

```
git commit  →  pre-commit hook  →  POST /hooks/pre-commit  →  202 Accepted  (immediate)
                                                           ↓
                                               goroutine: runPipeline()
                                                           ↓
                                          ExtractConcepts → cache.Save → Synthesize
                                                           ↓
                                            terminal.Adapter.Dispatch(question)
                                                           ↓
                                                  [daemon stdout]
```

**Limitation**: The question prints to the daemon's own stdout, not to the developer's terminal session. This is acceptable for v1 where the developer has `total-recall serve` running in a visible terminal pane, but is not ideal UX for the common case where the daemon runs in the background.

## Phase 4 Plan — Out-of-Band Delivery

Phase 4 will replace or augment the terminal adapter with real out-of-band delivery:

### Option A: VS Code Extension (Recommended for IDE users)
- The extension polls `GET /recall/next` on the daemon.
- The daemon queues synthesized questions (in-memory or SQLite).
- The extension surfaces the question as a VS Code notification with clickable answer choices.
- Uses the [VS Code Notifications API](https://code.visualstudio.com/api/references/vscode-api#window.showInformationMessage).

### Option B: Shell Integration (terminal-first users)
- A shell function (added to `.zshrc` / `.bashrc` by `tr init`) runs after each commit.
- It calls `GET /recall/next` and, if a question is waiting, renders it inline in the terminal.
- This recovers the native terminal UX without requiring an IDE extension.

### Option C: Hybrid
- Implement both A and B; the `Dispatcher` interface already supports multiple adapters.
- Configure via `~/.tr/config.yaml` `presentation:` block.

## Current Config Surface

```yaml
presentation:
  terminal: true  # enables terminal.Adapter dispatcher in v1
```

The `Dispatcher` interface (`internal/engine/dispatcher.go`) allows Phase 4 adapters to be added without touching existing code.
