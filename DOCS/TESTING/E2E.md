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
./tr serve
# Expected: "Total Recall daemon listening on :7331"
```

```sh
# 2. Health endpoint
curl http://localhost:7331/health
# Expected: {"status":"ok"}
```

```sh
# 3. Status command (separate terminal)
./tr status
# Expected:
#   ✓ Daemon running on localhost:7331
#   (followed by config --show output)
```

```sh
# 4. Status when daemon is NOT running (stop daemon first, then):
./tr status
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
rm .git/hooks/pre-commit

# Create an unmanaged hook
echo '#!/usr/bin/env bash' > .git/hooks/pre-commit
echo 'echo "existing hook ran"' >> .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit

/path/to/tr init  # enable pre-commit
cat .git/hooks/pre-commit
# Expected: file contains both the TR sentinel section AND
#           the original content wrapped in BEGIN/END existing hook delimiters
```

#### Hook dispatch (daemon must be running)

```sh
# 9. POST a hook payload manually
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

```sh
# 10. Trigger via real commit (inside scratch repo, daemon running)
echo "hello" > foo.txt
git add .
git commit -m "test: trigger TR hook"
# Expected:
#   - Commit succeeds (hook is non-blocking)
#   - Daemon terminal logs the received payload
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
#   - Advisory message: "Total Recall daemon not reachable — skipping"
#   - Commit SUCCEEDS (hook exits 0, never blocks Git)
```

**Not yet testable in this phase:** AI provider calls, concept extraction, recall question generation, terminal quiz presentation (all Phase 03).

---

## Phase 03 — Intelligence Layer _(template — fill in when shipped)_

**Goal:** Daemon processes hook payloads through AI, extracts concepts into cache, synthesises a recall question, and presents it in the terminal.

_Prerequisites, checks, and expected results to be defined during Phase 03 implementation._

| # | Check | Command | Expected |
|---|-------|---------|----------|
| 3.1 | ... | ... | ... |

**Not yet testable:** _(list deferred items here)_

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
