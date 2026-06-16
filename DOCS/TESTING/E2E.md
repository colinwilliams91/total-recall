# Total Recall — Manual E2E Testing Guide

This document is the canonical reference for manually verifying that Total Recall works end-to-end after each implementation phase.

Each phase section covers what is testable, what is intentionally untestable (deferred to a later phase), and the exact commands to run. Use prior phase sections as a regression baseline — each new phase should keep all previous checks passing.

---

## How to Use This Guide

1. Build the binary first (see [Build](#build)).
2. Work through the current phase section top-to-bottom.
3. Each check has an **expected result**. If actual ≠ expected, the phase has a regression.
4. When a new phase ships, add its section below following the same template.

---

## Build

Run from the repo root before testing any phase:

```sh
go build -o tr ./cmd/total-recall   # Linux/macOS
go build -o tr.exe ./cmd/total-recall  # Windows
```

Verify clean build and vet:

```sh
go build ./...
go vet ./...
```

Both must exit 0 with no output.

OR run `.\scripts\rebuild.ps1` on Windows PowerShell to clean, build, and vet.

---

## Phase 00 — Foundation

**Goal:** Binary exists and top-level commands are registered.

| Check | Command | Expected |
|-------|---------|----------|
| Binary runs | `./tr --help` | Usage text with `serve`, `init`, `config`, `status` listed |
| Version flag | `./tr --version` | `total-recall version dev` (or semver if built with ldflags) |

**Not yet testable in this phase:** All runtime behaviour (daemon, hooks, config, AI).

---

## Phase 01 — Config Architecture

**Goal:** Two-tier config loads correctly; `init` creates user config; `config --show` displays merged result.

### Setup

```sh
# Optionally clear existing config to test a fresh state:
rm ~/.tr/config.yaml   # Linux/macOS
del %USERPROFILE%\.tr\config.yaml  # Windows
```

### Checks

| # | Check | Command | Expected |
|---|-------|---------|----------|
| 1.1 | User config auto-creates | `./tr serve` (then Ctrl-C) | Advisory: "created ~/.tr/config.yaml" (unless --quiet) |
| 1.2 | Quiet flag suppresses advisory | `./tr serve --quiet` (then Ctrl-C) | No advisory printed |
| 1.3 | Init prompts for opt-in | `./tr init` (inside a git repo) | TUI confirm for conversation analysis; config written |
| 1.4 | Config show displays merged result | `./tr config --show` | Table of keys with `[user]`, `[repo]`, or `[default]` source tags |
| 1.5 | Repo config respected | Add `privacy:\n  conversation_analysis: true` to `.tr.yaml`; run `./tr config --show` | `conversation_analysis` shows `[repo]` source |

**Not yet testable:** Daemon routes, hook installation, AI calls.

---

## Phase 02 — Daemon Foundation

**Goal:** Daemon starts and accepts hook payloads; `tr init` installs hooks; `tr status` reflects live daemon state.

### Prerequisites

- Binary built from current source.
- A temporary Git repository for hook testing (created in step 4).

### Checks

#### Daemon lifecycle

```sh
# 1. Start the daemon (keep this terminal open throughout)
./tr serve # posix
.\tr.exe serve # windows

# Expected: "Total Recall daemon listening on :7331"
```

```sh
# 2. Health endpoint
curl http://localhost:7331/health # posix
Invoke-RestMethod http://localhost:7331/health # windows

# Expected: {"status":"ok"}
```

```sh
# 3. Status command (separate terminal)
./tr status # posix
.\tr.exe status # windows

# Expected:
#   ✓ Daemon running on localhost:7331
#   (followed by config --show output)
```

```sh
# 4. Status when daemon is NOT running (stop daemon first, then):
./tr status # posix
.\tr.exe status # windows
echo $?          # Linux/macOS
$LASTEXITCODE    # Windows PowerShell
# Expected: "✗ Daemon not running on localhost:7331" and exit code 1
```

#### Hook installation

```sh
# 5. Create a scratch repo
mkdir /tmp/tr-test && cd /tmp/tr-test   # Linux/macOS
mkdir C:\tmp\tr-test && cd C:\tmp\tr-test  # Windows

git init

# Run init from the scratch repo (point to your built binary)
/path/to/tr init

# Expected:
#   TUI: conversation analysis confirm
#   TUI: three hook selection confirms (pre-commit / commit-msg / pre-push)
#   "✓ User config saved to ~/.tr/config.yaml"
#   "✓ Repo config saved to ./.tr.yaml"
#   "✓ Installed N hook(s) into ./.git/hooks/"
```

```sh
# 6. Inspect installed hook
cat .git/hooks/pre-commit
# Expected: first line is "#!/usr/bin/env bash"
#           second line contains "# total-recall managed" (sentinel)
```

```sh
# 7. Re-run init (idempotency check)
/path/to/tr init
# Expected: same result; hook file regenerated in-place, NOT duplicated
#           Previous hook selections pre-populated in TUI
```

```sh
# 8. Hook chaining (existing unmanaged hook)
# Reset: remove managed hook
rm .git/hooks/pre-commit # posix
ri .git/hooks/pre-commit # windows

# POSIX:
# Create an unmanaged hook
echo '#!/usr/bin/env bash' > .git/hooks/pre-commit
echo 'echo "existing hook ran"' >> .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit

/path/to/tr init  # enable pre-commit
cat .git/hooks/pre-commit

# ---

# WINDOWS:
# Create an unmanaged hook
@'
#!/usr/bin/env bash
echo "existing hook ran"
'@ | Set-Content .git/hooks/pre-commit

/path/to/tr init  # enable pre-commit
cat .git/hooks/pre-commit

# ---

# Expected: file contains both the TR sentinel section AND
#           the original content wrapped in BEGIN/END existing hook delimiters
```

#### Hook dispatch (daemon must be running)

```sh
# 9. POST a hook payload manually (POSIX)
curl -s -o /dev/null -w "%{http_code}" \
  -X POST http://localhost:7331/hooks/pre-commit \
  -H "Content-Type: application/json" \
  -d '{"diff":"+ foo","files":["main.go"]}'
# Expected: 202

curl -s http://localhost:7331/hooks/pre-commit \
  -X POST -H "Content-Type: application/json" \
  -d '{"diff":"+ foo","files":["main.go"]}'
# Expected body: {"status":"received"}
```

```powershell
# 9. POST a hook payload manually (WINDOWS)
$response = Invoke-WebRequest `
    -Uri 'http://localhost:7331/hooks/pre-commit' `
    -Method POST `
    -ContentType 'application/json' `
    -Body '{"diff":"+ foo","files":["main.go"]}' `
    -SkipHttpErrorCheck

$response.StatusCode

# Expected: 202

Invoke-RestMethod `
    -Uri 'http://localhost:7331/hooks/pre-commit' `
    -Method POST `
    -ContentType 'application/json' `
    -Body '{"diff":"+ foo","files":["main.go"]}'

# Expected body: {"status":"received"}
```

```sh
# 10. Trigger via real commit (inside scratch repo, daemon running)
echo "hello" > foo.txt
git add .
git commit -m "test: trigger TR hook"
# Expected:
#   - Commit succeeds (hook is non-blocking)
#   - Daemon terminal prints a log line per enabled hook, e.g.:
#       2026/01/01 12:00:00 [hook] pre-commit  repo=/tmp/tr-test  branch=main
#       2026/01/01 12:00:00 [hook] commit-msg  repo=/tmp/tr-test  branch=main
#   Note: the daemon logs envelope metadata only (hook name, repo path, branch).
#   The full payload (diff, staged files, commit message) is received but not
#   logged in Phase 2 — it will be processed in Phase 3 by the AI pipeline.
#   Note: branch=HEAD is expected for a repo's very first commit (no named
#   branch exists yet). Subsequent commits will show the branch name.
```

#### P0 credential scan

```sh
# 11. Commit blocked by raw api-key in .tr.yaml
echo 'api-key: sk-supersecret' >> .tr.yaml
git add .tr.yaml
git commit -m "oops: leaked key"
# Expected: commit BLOCKED with message about raw api-key detected
#           "Use 'api-key: env:MY_VAR' instead"

# Cleanup
git checkout .tr.yaml
```

```sh
# 12. env: format is allowed through
# Edit .tr.yaml: api-key: env:OPENAI_API_KEY
git add .tr.yaml
git commit -m "fix: use env reference"
# Expected: commit proceeds (no block)
```

#### Graceful degradation (daemon not running)

```sh
# 13. Hook when daemon is down
# Stop the daemon, then commit:
echo "test" >> foo.txt
git add . && git commit -m "test: no daemon"
# Expected:
#   - Each enabled hook prints once:
#       "[total-recall] Daemon not running at http://localhost:7331 — skipping recall check. Start with 'total-recall serve'."
#   - If multiple hooks are enabled (e.g. pre-commit + commit-msg), you will see
#     the message TWICE — once per hook. This is expected behaviour, not a bug.
#   - Commit SUCCEEDS (all hooks exit 0 — TR never blocks Git)
```

**Not yet testable in this phase:** AI provider calls, concept extraction, recall question generation, terminal quiz presentation (all Phase 03).

---

## Phase 03 — Intelligence Layer

**Goal:** Daemon processes hook payloads through AI asynchronously — extracting concepts into the SQLite cache, synthesising a recall question, and printing it to the daemon terminal after each commit.

### Prerequisites

- Binary built from current source (`go build -o tr ./cmd/total-recall`).
- Scratch Git repo from Phase 02 still available (or create a new one).
- **At least one of the following for live AI checks:**
  - Anthropic API key (set `ANTHROPIC_API_KEY` in your shell)
  - OpenAI API key (set `OPENAI_API_KEY`)
  - Ollama running locally (`ollama serve` + at least one model pulled, e.g. `ollama pull llama3.2`)
- Checks 3.1–3.5 (config + TUI) are runnable without an API key. Checks 3.6–3.12 (AI pipeline) require a configured provider.

---

### Section A — `tr init` AI Provider TUI

```sh
# 3.1  Run init — new AI provider section appears before hooks
cd /tmp/tr-test   # your scratch repo
/path/to/tr init
```

**Expected TUI flow (in order):**
1. Conversation analysis confirm (existing — from Phase 01)
2. **NEW:** `🤖  AI Provider` — select menu with 6 options
3. Cloud provider (Anthropic/OpenAI/Groq): API key input pre-filled with `env:PROVIDER_API_KEY`, model name input
4. Local provider (Ollama/LM Studio): model name input only (no API key prompt)
5. Custom: base URL, model, optional API key
6. Hook selection (existing — from Phase 02)

```sh
# 3.2  Verify config written correctly (Anthropic example)
cat ~/.tr/config.yaml
```

**Expected (cloud provider):** `provider: anthropic`, `model: claude-sonnet-4-5`, `api-key: env:ANTHROPIC_API_KEY`, `base-url:` present and blank with an explanatory comment

```sh
# 3.3  (POSIX) base-url shown blank in config (not hidden by omitempty)
grep "base-url" ~/.tr/config.yaml
# Expected: line like "  base-url:  # ..." — visible even when empty

# 3.3 (WINDOWS) base-url shown blank in config (not hidden by omitempty)

Select-String `
    -Path "$env:USERPROFILE\.tr\config.yaml" `
    -Pattern "base-url"

# Expected:
#   base-url:  # ...
```

```sh
# 3.4  Re-run init — existing AI values pre-populated
/path/to/tr init
# Expected: API key, model, and provider fields pre-filled with
#           the values written in check 3.1 — user can confirm or change
```

```sh
# 3.5  config --show reflects new AI fields
/path/to/tr config --show
# Expected: rows for provider, model, api-key, base-url all present
#           with [user] or [default] source tags
```

---

### Section B — Daemon Startup with AI Configured

```sh
# 3.6 (POSIX)  Start daemon with AI configured (separate terminal)
ANTHROPIC_API_KEY=sk-... /path/to/tr serve
# Expected:
#   ✓ Total Recall daemon running on localhost:7331
#   (no error about provider — key resolved from env: reference)
```

```powershell
# 3.6 Start daemon with AI configured (separate terminal)

$env:ANTHROPIC_API_KEY = "sk-..."
C:\path\to\tr.exe serve
```

```sh
# 3.7  Start daemon WITHOUT AI configured (missing provider)
# Edit ~/.tr/config.yaml — set provider to "" or delete the ai: block, then:
/path/to/tr serve
# Expected:
#   ✓ Total Recall daemon running on localhost:7331
#   Advisory logged: "[daemon] AI provider not configured — recall questions
#     will not be generated. Run 'total-recall init' to configure."
#   Daemon continues running (AI is optional — non-blocking)
```

---

### Section C — Async Pipeline (requires configured provider + running daemon)

**Make sure your repo and user level config (yaml) files are set up correctly (API key, provider, model, base-url) and repo level config presentation.terminal == true in order to see logs**

(POSIX)
```sh
# 3.8  Manual hook POST — async 202 response
curl -s -o /dev/null -w "%{http_code}" \
  -X POST http://localhost:7331/hooks/pre-commit \
  -H "Content-Type: application/json" \
  -d '{
    "hook": "pre-commit",
    "repo": "/tmp/tr-test",
    "branch": "main",
    "timestamp": "2026-01-01T00:00:00Z",
    "payload": {"diff": "+ func retryWithBackoff(maxRetries int) error {\n+   time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)\n+ }"}
  }'
# Expected: 202  (immediate — hook does not wait for AI)
```

(WINDOWS)
```powershell
# 3.8 Manual hook POST — async 202 response

$response = Invoke-WebRequest `
    -Uri 'http://localhost:7331/hooks/pre-commit' `
    -Method POST `
    -ContentType 'application/json' `
    -Body @'
{
  "hook": "pre-commit",
  "repo": "/tmp/tr-test",
  "branch": "main",
  "timestamp": "2026-01-01T00:00:00Z",
  "payload": {
    "diff": "+ func retryWithBackoff(maxRetries int) error {\n+   time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)\n+ }"
  }
}
'@ `

$response.StatusCode

# Expected: 202
```

**Watch daemon terminal after the POST:**
```
Expected (within ~5-10 seconds):
  [hook] pre-commit  repo=/tmp/tr-test  branch=main
  [pipeline] ... (optional extraction log)

  🧠 Recall Check
  ─────────────────────────────────────────
    Why is jitter commonly added to exponential backoff retry intervals?

    1. To prevent retry storms from synchronizing across clients
    2. To increase the maximum retry delay
    3. To improve cache locality
    4. To reduce memory allocations
  ─────────────────────────────────────────
```

> **Note (v1 limitation):** The recall question prints to the daemon's own stdout, not to the committing developer's terminal. Phase 04 will add out-of-band delivery. See `DOCS/ARCHITECTURE/DELIVERY.md`.

(POSIX)
```sh
# 3.9  Real commit triggers async pipeline
cd /tmp/tr-test
cat > retry.go << 'EOF'
package main

import (
    "math"
    "time"
)

func retryWithBackoff(attempt int) {
    delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
    time.Sleep(delay)
}
EOF

git add retry.go
git commit -m "feat: add exponential backoff helper"
# Expected:
#   - Commit completes immediately (hook is non-blocking)
#   - Daemon terminal (within ~5-10 seconds) prints the 🧠 Recall Check block
```

(WINDOWS)
```powershell
# 3.9 Real commit triggers async pipeline

@'
package main

import (
    "math"
    "time"
)

func retryWithBackoff(attempt int) {
    delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
    time.Sleep(delay)
}
'@ | Set-Content retry.go

git add retry.go
git commit -m "feat: add exponential backoff helper"

# Expected:
#   - Commit completes immediately
#   - Recall Check appears in daemon terminal
```

```sh
# 3.10  Empty/whitespace diff — pipeline skips gracefully
# (use commit-msg hook only, no pre-commit)
git commit --allow-empty -m "chore: empty commit"
# Expected:
#   - Commit succeeds
#   - Daemon logs: "[pipeline] no diff in hook payload for commit-msg — skipping"
#   - No AI call made; no error
```

---

### Section D — Concept Cache

```sh
# 3.11  Cache database created after first commit
ls -la ~/.tr/concepts.db # POSIX
Get-Item "$env:USERPROFILE\.tr\concepts.db" # WINDOWS
# Expected: file exists (created on first successful Save)
```

```sh
# 3.12  Inspect cached concepts directly

# (POSIX)
sqlite3 ~/.tr/concepts.db \
  "SELECT concept, source, weight, seen_at FROM concepts ORDER BY seen_at DESC LIMIT 10;"

# (WINDOWS)
sqlite3 "$env:USERPROFILE\.tr\concepts.db" `
    "SELECT concept, source, weight, seen_at FROM concepts ORDER BY seen_at DESC LIMIT 10;"

# Expected: rows with concept names like "exponential backoff", "retry semantics";
#           source = "code"; weight between 0.0 and 1.0
#           (exact concepts depend on what the AI returned)
```

---

### Section E — Provider-Specific Spot Checks

#### Ollama (local, no API key)

```sh
# 3.13  Configure Ollama in init
/path/to/tr init
# Select: Ollama (local · free · runs on your machine)
# Enter model: llama3.2

# Verify config: (POSIX)
grep -A4 "^ai:" ~/.tr/config.yaml

# Verify config: (WINDOWS)
Select-String `
    -Path "$env:USERPROFILE\.tr\config.yaml" `
    -Pattern "^ai:" `
    -Context 0,4

# Expected: provider: ollama, model: llama3.2, api-key: (blank), base-url: (blank)
```

```sh
# 3.14  Daemon with Ollama (must have `ollama serve` running locally)
/path/to/tr serve
git add . && git commit -m "test: ollama provider"
# Expected: recall question printed to daemon terminal (no API key needed)
```

#### Custom endpoint

```sh
# 3.15  Custom provider with explicit base URL
/path/to/tr init
# Select: Custom
# Base URL: http://localhost:11434/v1   (Ollama OpenAI-compat endpoint)
# Model: llama3.2
# API key: (leave blank)

# (POSIX)
grep "base-url" ~/.tr/config.yaml

# (WINDOWS)
Select-String `
    -Path "$env:USERPROFILE\.tr\config.yaml" `
    -Pattern "base-url"

# Expected: base-url: http://localhost:11434/v1
```

---

### Section F — Graceful Degradation

```sh
# 3.16  AI failure (bad API key) — daemon continues, no crash
# Set a garbage API key: (POSIX)
ANTHROPIC_API_KEY=sk-garbage /path/to/tr serve

# Set a garbage API key: (WINDOWS)
$env:ANTHROPIC_API_KEY = "sk-garbage"

git add . && git commit -m "test: bad api key"
# Expected:
#   - Commit succeeds (non-blocking)
#   - Daemon logs: "[pipeline] extraction AI call failed: ..."
#   - No crash; daemon continues accepting new hook POSTs
```

```sh
# 3.17  All Phase 02 regression checks still pass
# Re-run checks 2.2 (health), 2.4 (status exit code), 2.10 (real commit dispatch),
# 2.11 (credential scan), 2.13 (graceful degradation)
# Expected: all pass unchanged — Phase 03 is additive
```

**Not yet testable in this phase:** Out-of-band delivery to the committing developer's terminal (requires Phase 04 VS Code extension or shell integration); `/recall/next` polling endpoint (Phase 04); answer tracking, spaced repetition, cognitive scoring (future).

---

## Phase 04A — Out-of-Band Delivery (MCP + Shell)

**Goal:** Questions are delivered via MCP to AI coding agents and via `tr ask` (post-commit hook) to terminal users — not through daemon stdout.

### Prerequisites

- Binary built from current source.
- Daemon running (`total-recall serve`) with AI configured (see Phase 03 prereqs).
- Scratch Git repo from Phase 03 with at least one concept-generating commit already made (so `memory.db` exists).

---

### Section A — MCP Endpoint

```sh
# 4.1  Verify /mcp/ handler is registered (POSIX)
curl -s -o /dev/null -w "%{http_code}" http://localhost:7331/mcp/
# Expected: 200 or 400 (MCP requires a valid SSE/streaming request — not 404)
```

```powershell
# 4.1 (WINDOWS)
$r = Invoke-WebRequest -Uri 'http://localhost:7331/mcp/' -SkipHttpErrorCheck
$r.StatusCode
# Expected: 200 or 400 (not 404)
```

```sh
# 4.2  recall_status tool via MCP — smoke test: POST a hook to queue a question (POSIX)
curl -s -X POST http://localhost:7331/hooks/pre-commit \
  -H "Content-Type: application/json" \
  -d '{"hook":"pre-commit","repo":"/tmp/tr-test","branch":"main","timestamp":"2026-01-01T00:00:00Z","payload":{"diff":"+ func parseAST(src string) (*ast.File, error)"}}'
# Expected: 202 (question will be queued after AI pipeline completes ~5-10s)
```

```powershell
# 4.2 (WINDOWS)
$response = Invoke-WebRequest `
    -Uri 'http://localhost:7331/hooks/pre-commit' `
    -Method POST `
    -ContentType 'application/json' `
    -Body '{"hook":"pre-commit","repo":"C:\\tmp\\tr-test","branch":"main","timestamp":"2026-01-01T00:00:00Z","payload":{"diff":"+ func parseAST(src string) (*ast.File, error)"}}' `
    -SkipHttpErrorCheck
$response.StatusCode
# Expected: 202
```

---

### Section B — REST Dequeue (`/recall/next` and `/recall/answer`)

> Wait ~10 seconds after the POST in check 4.2 before running check 4.3.

```sh
# 4.3  GET /recall/next — dequeue a question
curl -s http://localhost:7331/recall/next
# Expected (if question is ready):
#   {"id":1,"question":"Why does...","choices":["...","...","..."],"queued_at":"..."}
# Expected (if nothing queued yet):
#   204 No Content
```

```powershell
# 4.3 (WINDOWS)
Invoke-RestMethod -Uri 'http://localhost:7331/recall/next'
# 204 = no question yet; 200 + JSON = question ready
```

```sh
# 4.4  GET /recall/next is idempotent until claimed (POSIX)
# Second call with no answer posted should return 204 (question was claimed by first call)
curl -s -o /dev/null -w "%{http_code}" http://localhost:7331/recall/next
# Expected: 204
```

```powershell
# 4.4 (WINDOWS)
$r = Invoke-WebRequest -Uri 'http://localhost:7331/recall/next' -SkipHttpErrorCheck
$r.StatusCode
# Expected: 204
```

```sh
# 4.5  POST /recall/answer — record answer
# Use the id returned from check 4.3 (replace 1 with actual id)
curl -s -X POST http://localhost:7331/recall/answer \
  -H "Content-Type: application/json" \
  -d '{"id":1,"answer":"1"}'
# Expected: {"ok":true}
```

```powershell
# 4.5 (WINDOWS)
Invoke-RestMethod `
  -Uri 'http://localhost:7331/recall/answer' `
  -Method POST `
  -ContentType 'application/json' `
  -Body '{"id":1,"answer":"1"}'
# Expected: ok = True
```

```sh
# 4.6  POST /recall/answer — skip (POSIX)
curl -s -X POST http://localhost:7331/recall/answer \
  -H "Content-Type: application/json" \
  -d '{"id":1,"answer":"skip"}'
# Expected: {"ok":true}
```

```powershell
# 4.6 (WINDOWS)
Invoke-RestMethod `
  -Uri 'http://localhost:7331/recall/answer' `
  -Method POST `
  -ContentType 'application/json' `
  -Body '{"id":1,"answer":"skip"}'
# Expected: ok = True
```

```sh
# 4.7  Verify memory.db exists and questions table is populated

# (POSIX)
sqlite3 ~/.tr/memory.db \
  "SELECT id, question, claimed_by, answer FROM questions ORDER BY queued_at DESC LIMIT 5;"

# (WINDOWS)
sqlite3 "$env:USERPROFILE\.tr\memory.db" `
  "SELECT id, question, claimed_by, answer FROM questions ORDER BY queued_at DESC LIMIT 5;"

# Expected: rows with question text, claimed_by = "rest" or "mcp", answer or null
```

---

### Section C — `tr ask` Subcommand

```sh
# 4.8  tr ask with no question queued (daemon running)
./tr ask  # POSIX
.\tr.exe ask  # Windows

# Expected: "Thinking." animation while polling, then for the final 4 seconds:
#   "You're all caught up on your recall questions. Great job 🤖💗"
# Then returns to the shell once the 15-second timeout elapses.
# (Run this immediately after check 4.5 cleaned out the queue)
```

```sh
# 4.9  tr ask with a question queued (POSIX)
# First, queue a question by posting a hook payload (check 4.2), wait ~10s, then:
./tr ask

# Expected TUI flow:
#   "Thinking." animation (cycling) while polling
#   → Question text displayed with word-wrap
#   → All returned choices displayed with matching numeric labels
#   → Press the matching number key to select; Enter to skip; q/Esc to abandon
#   → Exits after selection
```

```powershell
# 4.9 (WINDOWS)
# First, post a hook payload (check 4.2), wait ~10s, then:
.\tr.exe ask

# Expected TUI flow: same as POSIX above
```

```sh
# 4.10  tr ask TTY guard — silent in non-interactive shell (POSIX)
echo "" | ./tr ask
# Expected: exits 0 with no output (not a TTY → silently no-op)
```

```powershell
# 4.10 (WINDOWS)
cmd /c "tr.exe ask < nul"
$LASTEXITCODE
# Expected: exits 0 with no output (not a TTY → silently no-op)
```

```sh
# 4.11  tr ask when daemon is not running (POSIX)
# Stop daemon, then:
./tr ask
# Expected: prints "[total-recall] Daemon not running. Start with total-recall serve." and exits 0
```

```powershell
# 4.11 (WINDOWS)
# Stop daemon, then:
.\tr.exe ask
# Expected: prints "[total-recall] Daemon not running. Start with total-recall serve." and exits 0
```

---

### Section D — Post-Commit Hook

```sh
# 4.12  Verify post-commit hook was installed by tr init (POSIX)
cat .git/hooks/post-commit
# Expected: contains "total-recall ask" or "$(which total-recall) ask"
```

```powershell
# 4.12 (WINDOWS)
Get-Content .git/hooks/post-commit
# Expected: contains "total-recall ask" or "$(which total-recall) ask"
```

```sh
# 4.13  Post-commit hook fires after a real commit (POSIX)
# With a question in the queue (daemon running):
echo "test" >> foo.txt && git add . && git commit -m "test: 4A post-commit hook"

# Expected:
#   - Commit completes immediately
#   - daemon terminal logs a single "[recall] question queued for terminal delivery choices=N" line
#   - tr ask TUI appears only in the committing terminal
#   - Question displayed; press key to answer or skip
```

```powershell
# 4.13 (WINDOWS)
# With a question in the queue (daemon running):
Add-Content foo.txt "test"
git add .
git commit -m "test: 4A post-commit hook"

# Expected:
#   - Commit completes immediately
#   - tr ask TUI appears in the committing terminal
#   - Question displayed; press key to answer or skip
```

---

### Section E — Graceful Degradation

```sh
# 4.14  /recall/next when no questions have been generated (POSIX)
# Fresh state: daemon running, no hook POSTs made
curl -s -o /dev/null -w "%{http_code}" http://localhost:7331/recall/next
# Expected: 204 No Content (not 500 or 404)
```

```powershell
# 4.14 (WINDOWS)
$r = Invoke-WebRequest -Uri 'http://localhost:7331/recall/next' -SkipHttpErrorCheck
$r.StatusCode
# Expected: 204 (not 500 or 404)
```

```sh
# 4.15  tr ask when daemon is unreachable (POSIX)
# Stop daemon, then run tr ask
./tr ask
# Expected: advisory printed, exits 0 — no panic, no error
```

```powershell
# 4.15 (WINDOWS)
# Stop daemon, then run tr ask
.\tr.exe ask
# Expected: advisory printed, exits 0 — no panic, no error
```

```sh
# 4.16  All Phase 03 regression checks still pass (POSIX + WINDOWS)
# Re-run checks 3.8 (202 from hook POST), 3.9 (real commit → AI pipeline),
# 3.11 (cache DB exists), 3.16 (bad key → no crash)
# Expected: all pass unchanged — Phase 4A is additive
```

**Not yet testable in this phase:** VS Code extension delivery (Phase 4B); daemon autostart; spaced repetition / answer scoring (future).

---

## Adding a New Phase

When a new phase is archived, append a section following this template:

```markdown
## Phase NN — <Name>

**Goal:** One sentence describing what this phase delivers end-to-end.

### Prerequisites
- ...

### Checks

| # | Check | Command | Expected |
|---|-------|---------|----------|
| N.1 | ... | ... | ... |

**Not yet testable in this phase:** ...
```

Keep prior phase sections intact as regression baselines.
