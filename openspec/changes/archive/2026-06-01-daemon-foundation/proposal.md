## Why

`total-recall serve` currently prints "not implemented" and all three hook scripts exit 0 immediately. Nothing actually runs. This phase wires up the runtime infrastructure that every subsequent phase depends on: a real HTTP daemon at `:7331` that hooks can talk to, with `tr init` managing hook installation and `tr status` reporting daemon health. After this phase, you can run `git commit` and verify that the daemon received the event — no AI yet, but the plumbing is proven.

## What Changes

- `total-recall serve` starts a real HTTP server on `localhost:7331` with `/hooks/*` and `/mcp/*` route groups
- `total-recall init` expands to install managed hooks into `.git/hooks/` with safe chaining for existing hooks
- `total-recall status` pings the daemon and reports alive/dead with active config summary
- `hooks/pre-commit.sh` (and `.bat`) becomes a real script: captures `git diff --cached`, POSTs to daemon `/hooks/pre-commit`, degrades gracefully if daemon is not running (exits 0 — never blocks Git)
- `hooks/commit-msg.sh` (and `.bat`) becomes a real script: reads `$1` (commit message file), POSTs to daemon `/hooks/commit-msg`
- `hooks/pre-push.sh` (and `.bat`) becomes a real script: reads stdin (ref list), POSTs to daemon `/hooks/pre-push`
- P0 security scan: pre-commit hook scans `.tr.yaml` for raw `api-key:` values and blocks the commit (`exit 1`) if found
- Daemon gracefully exits with advisory if user config is missing (suggests `tr init`)

## Capabilities

### New Capabilities

- `daemon-server`: HTTP server lifecycle — startup, port binding, route registration (`/hooks/*`, `/mcp/*`), health endpoint, graceful shutdown
- `hook-installation`: `tr init` hook management — install managed hooks into `.git/hooks/`, detect and chain existing hooks, idempotent re-runs, cross-platform (sh + bat)
- `hook-dispatch`: Hook scripts and their daemon protocol — diff/context capture per hook type, HTTP POST payload schema, graceful degradation when daemon is unreachable, P0 credential scan in pre-commit

### Modified Capabilities

_(none — existing config, merge, and MCP-gate specs are unchanged at the requirement level)_

## Impact

- `cmd/total-recall/main.go` — `serveCmd` and `statusCmd` gain real implementations; `initCmd`/`runInit` expands to install hooks
- `internal/engine/` — daemon HTTP server scaffolded here (currently a doc.go stub)
- `hooks/*.sh` and `hooks/*.bat` — all six files replaced with real implementations
- `go.mod` — no new dependencies required (`net/http` stdlib covers the daemon; `net/http` client covers hook→daemon POST)
- `.tr.yaml` `hooks:` section — already defined; `tr init` now acts on it to determine which hooks to install
- Cross-platform concern: Windows hook scripts (`.bat`) must replicate the sh behavior; `tr init` detects OS and installs the appropriate script pair
