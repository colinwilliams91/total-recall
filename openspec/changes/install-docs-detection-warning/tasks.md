## 1. Implementation — PATH detection function

- [ ] 1.1 Add a new function `checkTrOnPath()` (probably in a new file `cmd/tr/pathdetect.go` alongside main.go, OR inline in main.go — implementer's preference). On Unix (`runtime.GOOS != "windows"`): `_ , err := exec.LookPath("tr")`; if err is nil, `tr` is on PATH — return silently. On Windows: `out, err := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "(Get-Command tr -ErrorAction SilentlyContinue).Name)").Output()`; if err is nil AND `len(strings.TrimSpace(string(out))) > 0`, return silently.
- [ ] 1.2 If `tr` is not found, detect the shell. On Unix: read `os.Getenv("SHELL")`; if it contains `bash` → emit bash advisory; if it contains `zsh` → emit zsh advisory; otherwise default to the bash form (most portable — fish/nushell users can adapt). On Windows: always emit the PowerShell form. Print the advisory to `os.Stderr` (NOT stdout — keep stdout clean for `tr init`-related user-output pipelining).
- [ ] 1.3 Concrete advisory strings (recommended form, wording can be adjusted slightly during implementation):
  - Bash: `⚠  tr not found on PATH. Add $GOPATH/bin to PATH with: echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc`
  - Zsh: `⚠  tr not found on PATH. Add $GOPATH/bin to PATH with: echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc`
  - PowerShell: `⚠  tr not found on PATH. Add $GOPATH/bin to PATH with: Add-Content $PROFILE 'set PATH="$PATH;$(go env GOPATH)/bin"'`
  Append a second line for context: `   (tr repo installs a post-commit hook that relies on PATH resolution at fire time.)`
- [ ] 1.4 Verify the function makes NO file writes. Use `grep -r 'os.OpenFile\|os.WriteFile' cmd/tr/pathdetect.go` (or wherever the function lives) returning zero matches.

## 2. Wire detection into runInit

- [ ] 2.1 At the top of `runInit()` (post-Y3), call `checkTrOnPath()` BEFORE any prompt. Discard the return value (the function only prints or returns silently — it never returns an error to abort the init flow).
- [ ] 2.2 Verify the flow: `checkTrOnPath()` → existing conversation-analysis huh form → `runInitAI(&cfg)` → write `~/.tr/config.yaml` → print next-step guidance → exit.

## 3. (Optional) Wire detection into runRepo

- [ ] 3.1 If the estimate for this task is < 15 minutes, also call `checkTrOnPath()` at the top of `runRepo()` (post-Y3) BEFORE the `hooks.FindRepoRoot()` call. This closes the contrived "user skips `tr init`, runs `tr repo` by absolute path" gap.
- [ ] 3.2 If the estimate exceeds 15 minutes or a reviewer judges the scope-creep unacceptable, DEFER this task. The spec doesn't require it (the `path-detection-warning` spec only mandates detection at `tr init`); it's a defensive nicety.

## 4. README rewrite

- [ ] 4.1 Rewrite the **Setup** section of `README.md` to follow this structure (post-Y2 + post-Y3 reality):
  ```sh
  # 1. Install the binary (one-time, user-level). Note: the module path stays
  #    total-recall; only the binary is named tr.
  go install github.com/colinwilliams91/total-recall@latest

  # 2. One-time: add $GOPATH/bin to PATH. tr init will warn you if it can't
  #    find tr on PATH and suggest the exact one-line command for your shell.

  # 3. Initialize user config (run from any directory; one-time, user-level).
  tr init
  #    → prompts: conversation analysis opt-in, AI provider, API key, model
  #    → writes ~/.tr/config.yaml

  # 4. Start the daemon (long-running terminal; keep alive).
  tr serve
  #    → binds localhost:7331

  # 5. Per-repo setup (run inside each repo you want recall in).
  cd ~/projects/my-app
  tr repo
  #    → resolves .git/hooks via git rev-parse --git-path hooks
  #    → prompts for hook enablement (pre-commit, commit-msg, pre-push)
  #    → writes .tr.yaml + installs hooks

  # 6. Verify
  tr status
  git commit -m "..."  # triggers installed hooks (NOT the binary)
  ```
- [ ] 4.2 Add the prerequisite line near the top of the Setup section: "Requires Git 2.5+ (2015) for linked worktree support (`tr repo` resolves the git hooks directory via `git rev-parse --git-path hooks`, a Git 2.5+ feature)."
- [ ] 4.3 Add a "Non-Go install path" subsection: "Without Go: download the release archive from GitHub Releases, extract, place `tr` (or `tr.exe`) on PATH manually. Same downstream flow."
- [ ] 4.4 Add the legacy-binary-cleanup note (one sentence): "Existing installs from before the binary rename: remove the prior `total-recall` binary with `rm $(go env GOPATH)/bin/total-recall` (or `Remove-Item $(go env GOPATH)/bin/total-recall.exe` on Windows). Future invocations use the `tr` command only."
- [ ] 4.5 Walk the rest of the README and update every `total-recall <subcommand>` → `tr <subcommand>` (this should already be done by Y2's task 4.1; double-check by running `grep -n 'total-recall ' README.md | grep -v 'github.com/colinwilliams91' | grep -v '"total-recall"'` — the only survivors should be product-name references in prose).

## 5. Tests — path detect

- [ ] 5.1 Add a test file (or extend `cmd/tr/main_test.go`): `TestCheckTrOnPath_Found` — mock a PATH that contains a known binary (use a temp dir + symlink or a copy of the test binary itself) so `exec.LookPath` finds it; assert no stderr output.
- [ ] 5.2 `TestCheckTrOnPath_NotFound_Bash` — set `t.Setenv("SHELL", "/bin/bash")` and `t.Setenv("PATH", "/usr/bin")` (empty of `tr`); call `checkTrOnPath`; capture stderr; assert the bash-specific advisory is printed (substring match). Use `runtime.GOOS` to gate this test for non-Windows.
- [ ] 5.3 `TestCheckTrOnPath_NotFound_Zsh` — set `SHELL=/bin/zsh`; assert zsh form is printed. Same platform gate.
- [ ] 5.4 Windows test: `TestCheckTrOnPath_NotFound_PowerShell` — `runtime.GOOS == "windows"` gated; mock the missing-binary detection (testable by replacing the powershell.exe invocation via dependency-injection OR by setting PATH to empty and verifying the call to `Get-Command` returns nothing meaningful). This test may require a small refactor to make the powershell call mockable; if so, defer the test and document the limitation in `cmd/tr/main_test.go`.
- [ ] 5.5 `TestCheckTrOnPath_NeverWrites` — call `checkTrOnPath` with a temp HOME (`t.Setenv("HOME", t.TempDir())` on Unix; equivalent on Windows); after the call, verify no files were created in the temp HOME (no `.bashrc`, no `.zshrc` were created). This catches accidental os.WriteFile escaping into the function.

## 6. Verification

- [ ] 6.1 `go build ./... && go vet ./... && go test ./...` — all must pass.
- [ ] 6.2 Manual smoke (Unix): from a shell with PATH deliberately emptied of the directory containing `tr` (`PATH=/usr/local/bin:/usr/bin:/bin tr init`), verify the bash/zsh-specific warning prints to stderr AND `tr init` proceeds with the normal flow.
- [ ] 6.3 Manual smoke (Windows): invoke `tr init` from a PowerShell session where `$env:PATH` does NOT include the directory containing `tr.exe`; verify the PowerShell-specific warning prints to stderr.
- [ ] 6.4 README visual verification: open the rewritten Setup section; confirm every code block is shell-correct, every `\`tr ...\` invocation is `tr`, and the `go install ...` line still uses the full module path `github.com/colinwilliams91/total-recall@latest`.
- [ ] 6.5 `openspec validate install-docs-detection-warning` passes.