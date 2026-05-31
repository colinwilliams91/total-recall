## 1. Daemon HTTP Server (`internal/engine`)

- [x] 1.1 Add `go get github.com/colinwilliams91/total-recall` deps — verify no new external deps needed (net/http is stdlib); confirm go.mod is clean
- [x] 1.2 Create `internal/engine/server.go` — define `Server` struct with `*http.ServeMux`, port constant (7331), and `New(cfg *config.Config) *Server` constructor
- [x] 1.3 Implement `(*Server).RegisterRoutes()` — register route groups: `POST /hooks/pre-commit`, `POST /hooks/commit-msg`, `POST /hooks/pre-push`, `GET /health`, placeholder `POST /mcp/*`
- [x] 1.4 Implement `GET /health` handler — return `200 OK` with `{"status":"ok"}`
- [x] 1.5 Define `HookResponse` and `RecallPrompt` structs in `internal/engine/` — `HookResponse{Status string, Recall *RecallPrompt}` with `json:"recall,omitempty"`; Phase 2 always sets `Recall: nil`
- [x] 1.6 Implement hook route handlers (`/hooks/pre-commit`, `/hooks/commit-msg`, `/hooks/pre-push`) — decode JSON envelope, log receipt, return `202 Accepted` with `HookResponse{Status: "received"}`
- [x] 1.6 Implement `(*Server).Start()` — bind to `:7331`, check for port-in-use error, log startup message, block on `http.Server.ListenAndServe`
- [x] 1.7 Implement graceful shutdown — listen for `os.Signal` (SIGTERM, SIGINT) in a goroutine; call `http.Server.Shutdown(ctx)` with 5-second drain timeout on signal receipt
- [x] 1.8 Wire `serveCmd` in `cmd/total-recall/main.go` to instantiate `engine.New(cfg)` and call `server.Start()`

## 2. Hook Installation (`tr init` expansion)

- [x] 2.1 Create `internal/hooks/install.go` — define `Installer` with methods for detecting Git repo root (`git rev-parse --show-toplevel`), reading `.tr.yaml` hooks config, and determining which hooks to install
- [x] 2.2 Implement managed-hook sentinel detection — read first 5 lines of an existing `.git/hooks/<hook>` file and check for `# total-recall managed`
- [x] 2.3 Implement hook chaining logic — if existing unmanaged hook found, produce a chained script (existing content first, Total Recall dispatch appended, sentinel in header)
- [x] 2.4 Implement hook script generation — generate the `.sh` hook content for each hook type using the templates from `hooks/` (to be updated in Group 3)
- [x] 2.5 Implement `Installer.Install(hookName string)` — write generated script to `.git/hooks/<hookName>`, set permissions (`0755` on Unix; Git Bash compatible on Windows)
- [x] 2.6 Implement idempotent re-run — if managed hook exists, regenerate managed portion in place without re-wrapping
- [x] 2.7 Integrate hook selection + installation into `runInit()` — add Huh prompts (pre-populated from existing `.tr.yaml` if present) for each hook with impact descriptions; write selections to `.tr.yaml`; call `hooks.NewInstaller(repoRoot).InstallEnabled(repoCfg)` after config step
- [x] 2.8 Handle non-Git-repo case — if `git rev-parse` fails, skip hook installation and print advisory; do not error out

## 3. Hook Scripts (`hooks/*.sh` and `hooks/*.bat`)

- [x] 3.1 Rewrite `hooks/pre-commit.sh` — detect `curl`, run P0 credential scan on `.tr.yaml`, capture `git diff --cached`, POST JSON envelope to `:7331/hooks/pre-commit` with 2s timeout, degrade gracefully if daemon unreachable
- [x] 3.2 Rewrite `hooks/commit-msg.sh` — read commit message from `$1`, POST JSON envelope to `:7331/hooks/commit-msg` with 2s timeout, degrade gracefully
- [x] 3.3 Rewrite `hooks/pre-push.sh` — read ref list from stdin, POST JSON envelope to `:7331/hooks/pre-push` with 2s timeout, degrade gracefully
- [x] 3.4 Rewrite `hooks/pre-commit.bat` — PowerShell equivalent of pre-commit.sh including P0 scan (`Select-String`) and `Invoke-WebRequest` with timeout
- [x] 3.5 Rewrite `hooks/commit-msg.bat` — PowerShell equivalent of commit-msg.sh
- [x] 3.6 Rewrite `hooks/pre-push.bat` — PowerShell equivalent of pre-push.sh
- [x] 3.7 Verify JSON envelope schema in all hook scripts — `hook`, `repo`, `branch`, `timestamp`, `payload` fields match the spec

## 4. P0 Credential Scan (pre-commit)

- [x] 4.1 Implement `.tr.yaml` raw key detection in `hooks/pre-commit.sh` — grep for `api-key:` line not matching `env:` pattern; emit 🚨 alert and `exit 1` if found
- [x] 4.2 Implement equivalent detection in `hooks/pre-commit.bat` — `Select-String` pattern match for raw api-key; emit alert and `exit 1`
- [x] 4.3 Verify scan runs BEFORE diff capture in script order — security check must not be bypassable by a slow diff

## 5. `tr status` Command

- [x] 5.1 Implement `runStatus()` in `cmd/total-recall/main.go` — attempt `GET localhost:7331/health` with 1s timeout; print `✓ Daemon running on :7331` on success or `✗ Daemon not running` on failure
- [x] 5.2 On successful health check, load and display active config summary (provider, hooks enabled, conversation analysis setting)
- [x] 5.3 `tr status` exits with code 1 when daemon is not running (for scripting composability)

## 6. Docs and Cleanup

- [x] 6.1 Update `DOCS/ARCHITECTURE/DAEMON/INDEX.md` — add note that Phase 2 daemon is implemented (plumbing only; Phase 3 adds AI processing)
- [x] 6.2 Update `README.md` — add "Running the daemon" section with `total-recall serve` and `total-recall status` examples; add hook installation note under Setup
- [x] 6.3 Update `ROADMAP.md` — mark Phase 02 (daemon-foundation) as shipped; update Phase 03 description
- [x] 6.4 Run `go build ./...` and `go vet ./...` — confirm clean build with no new warnings
