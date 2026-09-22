# E2E Test Architecture

## What lives where

| Location | Scope | Strategy |
|----------|-------|----------|
| `cmd/torec/*_test.go` | Automated e2e (58 tests) | Go-native: model isolation, headless integration, golden file |
| `scripts/e2e/manual-init.ps1` | Manual e2e (3 steps) | Interactive TTY — `torec init` TUI only |
| `scripts/e2e/adaptive-ab.{sh,ps1}` | Manual exploration tool | Adaptive-difficulty A/B spike — question quality per signal arm |

## Why `torec init` is still manual

`torec init` uses `huh.NewForm()` which requires a real TTY for its interactive
mode. The `huh` library has an accessible mode (`TERM=dumb`) that switches to
plain `bufio.Scanner`-based stdin/stdout I/O, but it has a per-field
`bufio.Scanner` buffering bug: the first field's scanner buffers all available
input, and subsequent fields get EOF and silently fall back to defaults.

This makes piped-input automation unreliable. Until `huh` fixes the scanner
sharing (or we adopt a PTY library), `torec init` remains a manual test.

## Running the manual test

```powershell
# Build first
.\scripts\rebuild.ps1

# Run the manual init test
.\scripts\e2e\manual-init.ps1
```

The script will:
1. Back up your existing `~/.tr/config.yaml`
2. Create a scratch git repo
3. Prompt you to run `torec init` interactively (3 times)
4. Auto-verify config files, hook installation, and idempotency after each step
5. Restore your original config

## Adaptive difficulty A/B spike

`adaptive-ab.sh` (Linux/macOS) and `adaptive-ab.ps1` (Windows) are exploration
tools, not tests. They build a scratch repo whose commits bracket each
adaptive signal (ai-delegation, high-cluster, dispersed, all-code, fallback),
capture 10 questions per arm, and log the resolver's
`[recall] adaptive resolver selected <level> (signals: <list>)` lines for
side-by-side comparison of question quality. Results inform threshold tuning
in `internal/recall/difficulty/adaptive.go`; record observations in
`docs/CORE/ADAPTIVE_SPIKE.md`.

```bash
# Linux/macOS — requires an AI provider configured in $TR_HOME/config.yaml
scripts/e2e/adaptive-ab.sh ./bin/torec
```

```powershell
# Windows
.\scripts\e2e\adaptive-ab.ps1 -TorecPath .\bin\torec.exe
```

## What the Go tests cover

| Go test file | Strategy | Covers |
|--------------|----------|--------|
| `config_test.go` | Model isolation | Config defaults, merge logic, API key resolution, Show output, auto-create |
| `cache_test.go` | Model isolation | SQLite store: concepts, questions, queue depth, claim/answer |
| `provider_test.go` | Model isolation | Provider factory routing (anthropic, openai fallback, custom) |
| `ask_test.go` | Model isolation | Ask state machine: thinking, question, done, key press, ctrl+c, skip |
| `main_test.go` | Model isolation | Post-commit hook script generation (sentinel, shebang, escaping) |
| `integration_test.go` | Headless integration | Daemon health, hooks (all 3 types), recall next/answer, MCP endpoint |
| `golden_test.go` | Golden file | Ask model View() snapshots (thinking, caught-up, question, done) |
| `helpers_test.go` | Shared helper | `captureStderr()` for stderr assertion tests |

Run all automated tests:

```bash
go test ./cmd/torec/...
```

## Removed scripts

The following scripts were removed because their coverage migrated to Go tests:

| Script | Replaced by |
|--------|-------------|
| `phase-00.ps1` | (CLI flags — trivially verifiable) |
| `phase-01.ps1` | `config_test.go` |
| `phase-02.ps1` | `integration_test.go`, `main_test.go` |
| `phase-03.ps1` | `provider_test.go`, `cache_test.go` |
| `phase-04a.ps1` | `integration_test.go`, `ask_test.go`, `golden_test.go` |
| `run-all.ps1` | `go test ./cmd/torec/...` |
