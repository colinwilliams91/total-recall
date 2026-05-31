## 1. Database Migration (memory.db + questions table)

- [ ] 1.1 Update `defaultDBFilename` constant in `internal/cache/store.go` from `"concepts.db"` to `"memory.db"`; update the `Open()` doc comment accordingly
- [ ] 1.2 Add migration guard in `store.Open()`: if `memory.db` does not exist but `concepts.db` does, copy `concepts.db` → `memory.db` using `io.Copy` before opening; log `[store] migrated concepts.db → memory.db`
- [ ] 1.3 Add `questions` table to the schema migration in `store.Open()`:
  ```sql
  CREATE TABLE IF NOT EXISTS questions (
      id           INTEGER  PRIMARY KEY AUTOINCREMENT,
      question     TEXT     NOT NULL,
      choices      TEXT     NOT NULL,
      queued_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
      delivered_at DATETIME,
      claimed_by   TEXT,
      answer       TEXT,
      answered_at  DATETIME
  );
  ```
- [ ] 1.4 Implement `(*Store).SaveQuestion(ctx context.Context, q *recall.Question) error` — INSERT into `questions`; marshal `q.Choices` to JSON for the `choices` column
- [ ] 1.5 Implement `(*Store).NextQuestion(ctx context.Context, claimedBy string) (*StoredQuestion, error)` — single atomic `UPDATE questions SET delivered_at = datetime('now'), claimed_by = ? WHERE id = (SELECT id FROM questions WHERE delivered_at IS NULL ORDER BY queued_at ASC LIMIT 1) RETURNING id, question, choices, queued_at`; return `nil, nil` when zero rows affected
- [ ] 1.6 Define `StoredQuestion` struct in `internal/cache/store.go`: `ID int64`, `Question string`, `Choices []string`, `QueuedAt time.Time`; implement JSON unmarshal for `Choices` from the TEXT column
- [ ] 1.7 Implement `(*Store).AnswerQuestion(ctx context.Context, id int64, answer string) error` — `UPDATE questions SET answer = ?, answered_at = datetime('now') WHERE id = ?`
- [ ] 1.8 Implement `(*Store).QueueDepth(ctx context.Context) (int, error)` — `SELECT COUNT(*) FROM questions WHERE delivered_at IS NULL`; used by `recall_status` tool
- [ ] 1.9 Implement `(*Store).RecentAnswered(ctx context.Context, limit int) ([]StoredQuestion, error)` — `SELECT ... FROM questions WHERE answered_at IS NOT NULL ORDER BY answered_at DESC LIMIT ?`; used by `recall://recent` resource
- [ ] 1.10 Run `go build ./...` and `go vet ./...` — verify clean

## 2. REST Delivery Endpoints

- [ ] 2.1 Add `handleRecallNext(w http.ResponseWriter, r *http.Request)` method to `engine.Server` in `internal/engine/server.go` — calls `s.store.NextQuestion(ctx, "shell")`; if nil, responds `204 No Content`; if non-nil, responds `200 OK` with JSON `{"id":N,"question":"...","choices":["...","...","..."]}`
- [ ] 2.2 Add `handleRecallAnswer(w http.ResponseWriter, r *http.Request)` method — decodes `{"id":N,"answer":"..."}` from body; calls `s.store.AnswerQuestion(ctx, id, answer)`; responds `200 OK` with `{"ok":true}`; responds `400 Bad Request` on malformed body
- [ ] 2.3 Register both routes in `RegisterRoutes()`: `mux.HandleFunc("GET /recall/next", s.handleRecallNext)` and `mux.HandleFunc("POST /recall/answer", s.handleRecallAnswer)`
- [ ] 2.4 Run `go build ./...` and `go vet ./...` — verify clean; manually test with `curl` against a running daemon

## 3. MCP Server (internal/engine/mcp.go)

- [ ] 3.1 Add `github.com/modelcontextprotocol/go-sdk` to `go.mod` via `go get github.com/modelcontextprotocol/go-sdk`
- [ ] 3.2 Create `internal/engine/mcp.go` — define `buildMCPServer(store *cache.Store, cfg *config.Config) *mcp.Server`; construct `mcp.NewServer(&mcp.Implementation{Name: "total-recall", Version: "v1"}, nil)`
- [ ] 3.3 Register `recall_next` tool in `buildMCPServer`:
  - Input struct: `struct{}` (no args)
  - Handler calls `store.NextQuestion(ctx, "mcp")`; returns JSON `{"id":N,"question":"...","choices":[...]}` or `{"question":null}` if nil
- [ ] 3.4 Register `recall_answer` tool:
  - Input struct: `struct{ ID int64 \`json:"id"\`; Answer string \`json:"answer"\` }`
  - Handler calls `store.AnswerQuestion(ctx, id, answer)`; returns `{"ok":true}`
- [ ] 3.5 Register `recall_status` tool:
  - Input struct: `struct{}` (no args)
  - Handler calls `store.QueueDepth(ctx)` and checks `cfg.AI.Provider != ""`; returns `{"daemon":"ok","ai_configured":bool,"queue_depth":N}`
- [ ] 3.6 Register `recall://queue` resource: handler calls `store.QueueDepth` and `store.NextQuestion` (peek, not dequeue — read without UPDATE); returns JSON `{"depth":N,"next":{"question":"...","choices":[...]} | null}`; mark resource with `mcp.Resource{URI: "recall://queue", MIMEType: "application/json"}`
- [ ] 3.7 Register `recall://recent` resource: handler calls `store.RecentAnswered(ctx, 10)`; returns JSON array; mark `mcp.Resource{URI: "recall://recent", MIMEType: "application/json"}`
- [ ] 3.8 Register `recall_workflow` prompt: returns a `GetPromptResult` with a single user message containing the workflow instruction text (see design.md §8)
- [ ] 3.9 Add `*mcp.Server` field to `engine.Server` struct in `internal/engine/server.go`
- [ ] 3.10 Update `engine.New()` signature to accept `mcpSrv *mcp.Server`; store as `s.mcpServer`
- [ ] 3.11 In `RegisterRoutes()`, replace the `/mcp/` stub with `mux.Handle("/mcp/", mcp.NewStreamableHTTPHandler(s.mcpServer, nil))`
- [ ] 3.12 Update `cmd/total-recall/main.go` `serveCmd` to call `buildMCPServer(store, cfg)` and pass the result to `engine.New()`
- [ ] 3.13 Run `go build ./...` and `go vet ./...` — verify clean

## 4. Pipeline Bridge (runPipeline → notify MCP)

- [ ] 4.1 In `(*Server).runPipeline()` in `internal/engine/server.go`, after `recall.Synthesize` returns a non-nil question, call `s.store.SaveQuestion(ctx, question)` — log on error, continue
- [ ] 4.2 After saving, iterate `s.mcpServer.Sessions()` and call `sess.NotifyResourceUpdated(ctx, "recall://queue")` for each session — log errors per-session, do not abort the pipeline
- [ ] 4.3 Remove (or guard with `presentation.terminal: true` check) the existing `s.dispatcher.Dispatch(question)` call — terminal.Adapter is now opt-in only; dispatcher is nil unless explicitly configured
- [ ] 4.4 Update `engine.New()`: only wire `terminal.Adapter` as dispatcher when `cfg.Presentation.Terminal == true`; otherwise leave dispatcher nil
- [ ] 4.5 Add Phase 4B stub comment to `internal/engine/dispatcher.go`:
  ```go
  // Phase 4B: VSCodeAdapter will implement Dispatcher using the VS Code extension API.
  // See: https://code.visualstudio.com/api/references/vscode-api#window.showInformationMessage
  // Planned for a fast-follow release in its own repository.
  ```
- [ ] 4.6 Run `go build ./...` and `go vet ./...` — verify clean; commit a file and confirm question appears in `memory.db` via `sqlite3 ~/.tr/memory.db "SELECT * FROM questions;"`

## 5. tr ask Subcommand

- [ ] 5.1 Create `cmd/total-recall/ask.go` — define `askCmd` as a Cobra command: `Use: "ask"`, `Short: "Surface the next recall question in your terminal"`; register `--timeout` flag (default 15 seconds)
- [ ] 5.2 Add TTY check at start of `askCmd.RunE`: if `!term.IsTerminal(int(os.Stdout.Fd()))`, return nil immediately (exit 0, no output); import `golang.org/x/term`
- [ ] 5.3 Implement the Bubbletea model for `tr ask`:
  - States: `stateThinking`, `stateQuestion`, `stateDone`
  - `thinkingModel`: frame index (0-2), timeout ticker, poll result channel
  - `questionModel`: `StoredQuestion`, selected choice index
- [ ] 5.4 Implement "Thinking." animation: `tickMsg` fired every 400ms via `tea.Tick`; advance frame: `frames := []string{"Thinking.", "Thinking..", "Thinking..."}` cycling on modulo 3; render on the same line (clear + reprint)
- [ ] 5.5 Implement REST poll in Bubbletea: on each tick, if no outstanding poll, fire a `tea.Cmd` that calls `GET http://localhost:7331/recall/next`; on 204 No Content, return a `noQuestionMsg`; on 200, return a `questionMsg{StoredQuestion}`; on connection error, return a `daemonUnreachableMsg`
- [ ] 5.6 Implement state transitions:
  - `questionMsg` received → transition to `stateQuestion`; render question UI
  - `daemonUnreachableMsg` → transition to `stateDone`; return `tea.Quit`
  - `noQuestionMsg` + timeout elapsed → transition to `stateDone`; return `tea.Quit`
- [ ] 5.7 Implement keypress handler in `stateQuestion`:
  - `'1'`, `'2'`, `'3'`: `POST /recall/answer {id, answer: choices[n-1]}`; print `"✓ recorded"`; return `tea.Quit`
  - `tea.KeyEnter`: `POST /recall/answer {id, answer: "skip"}`; print `"→ skipped"`; return `tea.Quit`
  - `'q'`, `tea.KeyEsc`: return `tea.Quit` without posting (question remains unclaimed for next poll)
- [ ] 5.8 Implement question view:
  ```
  🧠  Recall Check
  ──────────────────────────────────────
    <question text (wrapped at 60 chars)>
    1. <choice>
    2. <choice>
    3. <choice>
  ──────────────────────────────────────
  [1-3] or Enter to skip:
  ```
- [ ] 5.9 Register `askCmd` in `main.go` root command
- [ ] 5.10 Add `golang.org/x/term` to `go.mod` if not already present
- [ ] 5.11 Run `go build ./...` and `go vet ./...` — verify clean; manually test `tr ask` with daemon running and a question in the queue

## 6. Post-Commit Hook (tr init)

- [ ] 6.1 Add post-commit hook template string in the hooks section of `cmd/total-recall/main.go` (or the relevant `init.go` file):
  ```sh
  #!/bin/sh
  # Total Recall post-commit hook
  # Surfaces a recall question after each successful commit.
  # Generated by 'total-recall init'. Run 'total-recall init' again to reinstall.
  exec "$(which total-recall)" ask
  ```
- [ ] 6.2 In `runInit()`, after installing existing hooks (pre-commit, commit-msg, pre-push), write the post-commit hook to `.git/hooks/post-commit`, `chmod 0755`, and log `"Installed post-commit hook"`
- [ ] 6.3 Add a Huh confirmation or info note in the TUI flow indicating that the post-commit hook will surface recall questions in the terminal after each commit (sets user expectations)
- [ ] 6.4 Run `go build ./...` and `go vet ./...`; run `total-recall init` in a test repo and verify `.git/hooks/post-commit` is created and executable

## 7. Documentation + Docs

- [ ] 7.1 Update `DOCS/ARCHITECTURE/DELIVERY.md`:
  - Replace the v1 "daemon stdout" section with a summary of Phase 4A delivery paths (MCP + shell)
  - Update Phase 4 plan section to reflect what was implemented vs. deferred (4B)
  - Note `terminal.Adapter` as opt-in for `presentation.terminal: true`
  - Add VS Code extension as Phase 4B
- [ ] 7.2 Update `ROADMAP.md`:
  - Mark Phase 03 wording as shipped (if not already from Phase 3)
  - Update Phase 04 entry: MCP as primary, shell (`tr ask`) as fallback, VS Code as 4B
- [ ] 7.3 Add Phase 4A section to `DOCS/TESTING/E2E.md` following the Phase 3 format (Sections A–E covering: MCP connectivity check, question dequeue via REST, `tr ask` animation + answer flow, post-commit hook integration, graceful degradation when daemon is unreachable)
- [ ] 7.4 Run `go build ./...` and `go vet ./...` — final clean build verification across entire repo
