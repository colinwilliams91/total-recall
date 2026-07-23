# AGENTS.md

## Build & Verify

| Task | Command |
|------|---------|
| Build | `make build` → `bin/tr(.exe)` |
| Quick rebuild (Windows) | `.\scripts\rebuild.ps1` → `tr.exe` in root, also runs `go vet` |
| Test | `go test ./...` |
| Single test | `go test -run TestName ./path/to/pkg/...` |
| Lint | `golangci-lint run` (must be installed separately) |
| Tidy deps | `go mod tidy` |

Run order: `go build ./... && go vet ./... && go test ./...`

## Architecture

- **Entrypoint**: `cmd/tr/main.go` (Cobra CLI)
- **Provider factory**: `cmd/tr/wire.go` — lives in cmd layer intentionally to avoid import cycles between `internal/ai` and its adapter sub-packages
- **Daemon**: `tr serve` binds `localhost:7331`; Git hooks are thin HTTP clients that POST to it
- **Config**: `~/.tr/config.yaml` (user) deep-merged with `.tr.yaml` (repo). `privacy.*` and `ai.*` keys in `.tr.yaml` are silently discarded — those are user-level only. `TR_HOME` env var overrides the data directory (default `~/.tr`) for test/CI isolation
- **Cache**: SQLite at `~/.tr/memory.db` via `modernc.org/sqlite` (pure Go, no CGo) — tables: `concepts`, `questions`. Concepts and questions are scoped per-repo AND per-branch (no global pool). Empty `repo` or `branch` is refused at the store layer.
- **MCP server**: mounted at `/mcp/` inside the daemon
- **Install & layer model**: five independent layers (binary / user config / user cache / repo config / git hooks) with a small set of explicit leak points. Rebuilding the binary does NOT update installed hook files — re-run `tr repo` for that. See [DOCS/ARCHITECTURE/INSTALL_LAYERS.md](DOCS/ARCHITECTURE/INSTALL_LAYERS.md) for the full model, canonical new-user install flow, and the testing simulation rules (scratch must be its own repo; re-`tr repo` after hook-body changes)

### Key packages

| Package | Role |
|---------|------|
| `internal/engine` | HTTP server, hook routes, async pipeline orchestration |
| `internal/ai` | Provider interface + adapters (`anthropic/`, `openai/`) |
| `internal/config` | Two-tier config load, merge, validation |
| `internal/pipeline` | Concept extraction from staged diffs |
| `internal/recall` | Question synthesis from cached concepts |
| `internal/cache` | SQLite concept store |
| `internal/hooks` | Hook installation, script generation, HTTP dispatch |
| `internal/presentation` | Dispatcher interface + terminal adapter |
| `internal/mcp` | MCP tool/resource/prompt registration |

## Conventions

- **No CGo**: SQLite is `modernc.org/sqlite`. Do not introduce `mattn/go-sqlite3` or any CGo dependency
- **Conventional commits**
- **Prompt assets** live under `assets/prompts/` — runtime cognition assets loaded dynamically, not static docs
- **OpenSpec**: repo uses spec-driven development. Specs: `openspec/specs/`. Changes: `openspec/changes/`. Config: `openspec/config.yaml`
- **Hooks**: shell scripts in `hooks/` come in `.sh` + `.bat` pairs. The managed installer writes to `.git/hooks/` at `tr repo` time
- **Keep adapters thin**: Core Go Engine is authoritative; hooks, MCP, and presentation are thin clients

## Testing

### Framework

All automated tests are Go-native, collocated in `cmd/tr/*_test.go`. No external test runners or Node.js dependencies. The test suite uses three strategies from the Bubble Tea testing model:

| Strategy | What it tests | Key tools |
|----------|--------------|-----------|
| **Model isolation** | State transitions, pure logic, data layer | Direct `Update(msg)` / `View()` calls, `bytes.Buffer` for IO |
| **Headless integration** | Daemon HTTP lifecycle, endpoints | `tea.NewProgram` with `WithInput`/`WithOutput`, `net.Listen` + `engine.Serve()` |
| **Golden file** | Visual regression of TUI views | `testdata/*.golden` compared byte-for-byte |

### Test files

| File | Strategy | Covers |
|------|----------|--------|
| `config_test.go` | Model isolation | Config defaults, merge logic, API key resolution, Show output, auto-create |
| `cache_test.go` | Model isolation | SQLite store: concepts, questions, queue depth, claim/answer |
| `provider_test.go` | Model isolation | `newProvider()` factory routing (anthropic, openai fallback, custom) |
| `ask_test.go` | Model isolation | `askModel` state machine: thinking, question, done, key press, ctrl+c, skip |
| `main_test.go` | Model isolation | Post-commit hook script generation (sentinel, shebang, escaping) |
| `integration_test.go` | Headless integration | Daemon health, hooks (all 3 types), recall next/answer, MCP endpoint |
| `golden_test.go` | Golden file | `askModel.View()` snapshots (thinking, caught-up, question, done) |
| `helpers_test.go` | Shared helper | `captureStderr()` for stderr assertion tests |

### Manual e2e

`scripts/e2e/manual-init.ps1` — the only manual test. Covers `tr init` TUI flow (huh forms require a real TTY; accessible mode has a `bufio.Scanner` buffering bug). See `scripts/e2e/README.md` for details.

### When adding or extending features

Follow these steps to maintain test coverage:

1. **New Bubble Tea model or state machine** → Add model isolation tests in the relevant `*_test.go` file. Call `Update(msg)` directly, type-assert the result, assert on state/fields. See `ask_test.go` for the pattern.

2. **New HTTP endpoint on the daemon** → Add a headless integration test in `integration_test.go`. Use `startTestDaemon(t)` to spin up the server on a random port, then `mustGET`/`mustPOST` helpers. Seed data via the `*cache.Store` returned by `startTestDaemon`.

3. **New config field or merge rule** → Add a test in `config_test.go`. Call `config.DefaultUserConfig()`, `config.Merge()`, or `config.Show()` with a `bytes.Buffer` and assert on the output.

4. **New cache operation** → Add a test in `cache_test.go`. Use `setupCache(t)` which redirects `~/.tr` to a temp dir via `t.Setenv("HOME", ...)`.

5. **New AI provider** → Add a case to `TestNewProviderRoutesOpenAIFallback` (or `TestNewProviderRoutesAnthropic` if it uses the Anthropic adapter) in `provider_test.go`.

6. **New TUI view or visual change** → Add a golden file test in `golden_test.go`. Run `$env:UPDATE_GOLDEN=1; go test -run TestGolden... ./cmd/tr/...` to generate the snapshot, then re-run without the flag to verify.

7. **New hook script content** → Add a test in `main_test.go` asserting on `buildPostCommitHookScript()` output.

### Golden file workflow

```powershell
# Generate or update golden files
$env:UPDATE_GOLDEN=1; go test -run TestGolden ./cmd/tr/...

# Verify (normal CI run)
go test -run TestGolden ./cmd/tr/...
```

Golden files live in `cmd/tr/testdata/*.golden` and are marked `-text` in `.gitattributes` to prevent CRLF corruption on Windows.

### Key patterns

- **Env isolation**: Use `t.Setenv("HOME", t.TempDir())` and `t.Setenv("USERPROFILE", t.TempDir())` for tests that touch `~/.tr/`
- **Stderr capture**: Use `captureStderr(&buf)` from `helpers_test.go` — call `restore()` before reading the buffer
- **Daemon lifecycle**: `startTestDaemon(t)` returns `(*engine.Server, *cache.Store, baseURL)` — cleanup is automatic via `t.Cleanup`
- **Keyboard simulation**: Use `tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'1'}})` for character keys, `tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC})` for Ctrl+C, `tea.KeyMsg(tea.Key{Type: tea.KeyEnter})` for Enter
