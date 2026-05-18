# Phase 00: Tasks

## Group 1 — Go Module Initialization

### 1.1 Initialize Go module
```
go mod init github.com/colinwilliams91/total-recall
```
Run from the repo root. Produces `go.mod` with `go 1.22` (or latest stable).

### 1.2 Add initial dependencies
```
go get github.com/spf13/cobra
go get github.com/charmbracelet/huh
go get github.com/mark3labs/mcp-go
go get modernc.org/sqlite
```

### 1.3 Tidy module
```
go mod tidy
```
Produces clean `go.sum`. Verify no CGO dependencies are introduced.

---

## Group 2 — Directory Structure

### 2.1 Create internal package stubs

Create a `doc.go` in each internal package with a package declaration and a one-line description comment. No logic.

Packages to stub:

| Path | Description |
|---|---|
| `internal/config/` | Config schema, loader, and deep-merge logic |
| `internal/engine/` | Core Engine orchestration and daemon lifecycle |
| `internal/eventmonitor/` | Event Monitor: filesystem, Git index, hook events, MCP events |
| `internal/pipeline/` | Incremental Analysis Pipeline |
| `internal/cache/` | Background Concept Cache (SQLite) |
| `internal/recall/` | Recall Engine: question synthesis and dispatch |
| `internal/ai/` | AI provider abstraction layer |
| `internal/ai/anthropic/` | Anthropic provider implementation |
| `internal/ai/openai/` | OpenAI-compatible provider implementation |
| `internal/mcp/` | MCP API handler |
| `internal/presentation/` | Presentation layer dispatch |
| `internal/presentation/terminal/` | Terminal adapter |
| `internal/presentation/mcp/` | MCP output adapter |

### 2.2 Create CLI entry point

`cmd/total-recall/main.go` with:
- Cobra root command (`total-recall`)
- Four stubbed subcommands: `serve`, `init`, `config`, `status`
- Each subcommand prints a placeholder: `"not implemented"` and returns nil
- `--version` flag on root

### 2.3 Create hook template stubs

In `hooks/`:

| File | Notes |
|---|---|
| `pre-commit.sh` | POSIX shell, exits 0 (fail-safe stub) |
| `commit-msg.sh` | POSIX shell, exits 0 |
| `pre-push.sh` | POSIX shell, exits 0 |
| `pre-commit.bat` | Windows batch, exit /b 0 |
| `commit-msg.bat` | Windows batch, exit /b 0 |
| `pre-push.bat` | Windows batch, exit /b 0 |

Templates will eventually call `localhost:7331` — stubs just exit cleanly. See `GIT_HOOKS.md` for what each hook is responsible for when implemented.

---

## Group 3 — Build Infrastructure

### 3.1 Create Makefile

Targets:

| Target | Command |
|---|---|
| `build` | `go build -o bin/total-recall ./cmd/total-recall` |
| `install` | `go install ./cmd/total-recall` |
| `test` | `go test ./...` |
| `lint` | `golangci-lint run` (if installed; no-op if not) |
| `clean` | `rm -rf bin/` (Unix) / `if exist bin\ rmdir /s /q bin\` (Windows) |
| `tidy` | `go mod tidy` |

Makefile detects OS via the `$(OS)` variable to handle the `clean` target difference.

### 3.2 Verify clean build
```
go build ./...
```
Must produce zero errors. Binary produced at `bin/total-recall` (or `bin/total-recall.exe` on Windows).

### 3.3 Verify test runner
```
go test ./...
```
Must exit 0. No test files exist yet — this just confirms the package graph compiles under test.

---

## Group 4 — Cross-Platform Verification

### 4.1 Confirm no CGO
```
go env CGO_ENABLED
```
Must return `0`. Verify `modernc.org/sqlite` is the only SQLite dependency (not `mattn/go-sqlite3` which requires CGO).

### 4.2 Verify build on Windows

Run `go build ./...` on the primary dev machine (Windows). Confirm binary is `total-recall.exe`.

### 4.3 Cross-compile to Linux (smoke check)
```
$env:GOOS="linux"; $env:GOARCH="amd64"; go build ./...
```
Must succeed. Validates no OS-specific imports have leaked into the stub packages.

---

## Group 5 — Commit

### 5.1 Commit scaffolding

Commit message:
```
feat: scaffold Go project structure (phase 00)

- go mod init github.com/colinwilliams91/total-recall
- internal package stubs for all documented subsystems
- cmd/total-recall with cobra root and subcommand stubs
- hook templates (sh + bat) in hooks/
- Makefile with build, test, lint, clean, tidy targets
- verified: go build ./... and go test ./... pass
- no CGO dependencies
```
