# AGENTS.md

## Build & Verify

| Task | Command |
|------|---------|
| Build | `make build` → `bin/total-recall(.exe)` |
| Quick rebuild (Windows) | `.\scripts\rebuild.ps1` → `tr.exe` in root, also runs `go vet` |
| Test | `go test ./...` |
| Single test | `go test -run TestName ./path/to/pkg/...` |
| Lint | `golangci-lint run` (must be installed separately) |
| Tidy deps | `go mod tidy` |

Run order: `go build ./... && go vet ./... && go test ./...`

## Architecture

- **Entrypoint**: `cmd/total-recall/main.go` (Cobra CLI)
- **Provider factory**: `cmd/total-recall/wire.go` — lives in cmd layer intentionally to avoid import cycles between `internal/ai` and its adapter sub-packages
- **Daemon**: `total-recall serve` binds `localhost:7331`; Git hooks are thin HTTP clients that POST to it
- **Config**: `~/.tr/config.yaml` (user) deep-merged with `.tr.yaml` (repo). `privacy.*` and `ai.*` keys in `.tr.yaml` are silently discarded — those are user-level only
- **Cache**: SQLite at `~/.tr/concepts.db` via `modernc.org/sqlite` (pure Go, no CGo)
- **MCP server**: mounted at `/mcp/` inside the daemon

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
- **Hooks**: shell scripts in `hooks/` come in `.sh` + `.bat` pairs. The managed installer writes to `.git/hooks/` at `total-recall init` time
- **Keep adapters thin**: Core Go Engine is authoritative; hooks, MCP, and presentation are thin clients

## Testing

- Tests are sparse (3 files). Most packages have only `doc.go` stubs
- `go test ./...` passes with "no test files" for most packages — this is expected, not a failure
- Test files: `cmd/total-recall/main_test.go`, `cmd/total-recall/ask_test.go`, `internal/presentation/terminal/adapter_test.go`
