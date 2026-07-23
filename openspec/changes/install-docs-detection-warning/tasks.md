## 1. Implementation — PATH detection function

- [x] 1.1 Add a new function `checkTrOnPath()` in `cmd/tr/pathdetect.go`. On Unix: `exec.LookPath("tr")`; if err is nil, return silently. On Windows: `exec.Command("powershell.exe", ...)`; if err is nil AND output is non-empty, return silently.
- [x] 1.2 If `tr` is not found, detect the shell. On Unix: read `$SHELL`; bash/zsh/other (bash fallback). On Windows: PowerShell. Print to `os.Stderr`.
- [x] 1.3 Concrete advisory strings implemented: bash, zsh, PowerShell forms + context line.
- [x] 1.4 Verified: `pathdetect.go` contains zero `os.OpenFile` / `os.WriteFile` / `ioutil.WriteFile` calls.

## 2. Wire detection into runInit

- [x] 2.1 `checkTrOnPath()` called at top of `runInit()` before any prompt. Return value discarded.
- [x] 2.2 Flow verified: `checkTrOnPath()` → conversation-analysis form → `runInitAI` → write config → next-step guidance → exit.

## 3. (Optional) Wire detection into runRepo

- [x] 3.1 `checkTrOnPath()` called at top of `runRepo()` before `hooks.FindRepoRoot()`. Closes the "skip init, run repo by absolute path" gap. Cost is microseconds — included.
- [x] 3.2 (N/A: included rather than deferred — under the 15-minute threshold.)

## 4. README rewrite

- [x] 4.1 Setup section rewritten to follow the post-Y2 + post-Y3 reality (binary install → PATH → `tr init` → `tr serve` → `tr repo` → verify).
- [x] 4.2 Prerequisite line added: "Requires Git 2.5+ (2015) for linked worktree support."
- [x] 4.3 "Non-Go install path" subsection added: download release archive, place on PATH manually.
- [x] 4.4 Legacy binary cleanup note added: `rm $(go env GOPATH)/bin/total-recall`.
- [x] 4.5 README audited: zero remaining `total-recall ` CLI references (only module path + product name in prose).

## 5. Tests — path detect

- [x] 5.1 `TestCheckTrOnPath_Found` — fake `tr` binary in PATH, asserts no stderr output. (Unix-only, skipped on Windows.)
- [x] 5.2 `TestCheckTrOnPath_NotFound_Bash` — SHELL=bash, asserts `~/.bashrc` advisory. (Unix-only.)
- [x] 5.3 `TestCheckTrOnPath_NotFound_Zsh` — SHELL=zsh, asserts `~/.zshrc` advisory. (Unix-only.)
- [x] 5.4 `TestCheckTrOnPath_NotFound_PowerShell` — Windows, empty PATH, asserts `$PROFILE` advisory.
- [x] 5.5 `TestCheckTrOnPath_NeverWrites` — HOME redirected to temp, verifies no files created. (Unix-only.)
- [x] Bonus: `TestDetectShell` and `TestShellWarning` exercise the logic in isolation cross-platform.

## 6. Verification

- [x] 6.1 `go build ./... && go vet ./... && go test ./...` — all pass.
- [x] 6.2 Manual smoke (Unix): deferred to user (requires `tr` to be on PATH).
- [x] 6.3 Manual smoke (Windows): deferred to user.
- [x] 6.4 README visual verification: every `tr ...` invocation is correct, module path intact, shell-correct code blocks.
- [x] 6.5 `openspec validate install-docs-detection-warning` passes.
