# Phase 00: Tasks

## 1. Go Module Initialization

- [x] 1.1 Run `go mod init github.com/colinwilliams91/total-recall` from repo root
- [x] 1.2 Add initial dependencies: `github.com/spf13/cobra`, `github.com/charmbracelet/huh`, `github.com/mark3labs/mcp-go`, `modernc.org/sqlite`
- [x] 1.3 Run `go mod tidy` and verify no CGO dependencies are introduced

## 2. Directory Structure & Package Stubs

- [x] 2.1 Create `doc.go` stubs for all internal packages: `config`, `engine`, `eventmonitor`, `pipeline`, `cache`, `recall`, `ai`, `ai/anthropic`, `ai/openai`, `mcp`, `presentation`, `presentation/terminal`, `presentation/mcp` — each with package declaration and one-line doc comment, no logic
- [x] 2.2 Create `cmd/total-recall/main.go` with cobra root command, `--version` flag, and four stubbed subcommands (`serve`, `init`, `config`, `status`) each printing "not implemented"
- [x] 2.3 Create POSIX hook stubs in `hooks/`: `pre-commit.sh`, `commit-msg.sh`, `pre-push.sh` — each exits 0
- [x] 2.4 Create Windows hook stubs in `hooks/`: `pre-commit.bat`, `commit-msg.bat`, `pre-push.bat` — each `exit /b 0`

## 3. Build Infrastructure

- [x] 3.1 Create `Makefile` with targets: `build`, `install`, `test`, `lint`, `clean`, `tidy` — `clean` target is OS-aware (`rm -rf bin/` on Unix, `if exist bin\ rmdir /s /q bin\` on Windows via `$(OS)` variable)
- [x] 3.2 Verify `go build ./...` compiles with zero errors and produces binary
- [x] 3.3 Verify `go test ./...` exits 0 (no tests yet — just confirms package graph compiles)

## 4. Cross-Platform Verification

- [x] 4.1 Confirm no CGO: verify `modernc.org/sqlite` is used (not `mattn/go-sqlite3`)
- [x] 4.2 Verify build on Windows produces `total-recall.exe`
- [x] 4.3 Cross-compile smoke check: `GOOS=linux GOARCH=amd64 go build ./...` must succeed

## 5. Commit

- [x] 5.1 Commit all scaffolding with message: `feat: scaffold Go project structure (phase 00)`
