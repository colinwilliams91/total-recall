## Why

The README's Setup section today describes a single `total-recall init` flow that Y3 will split into two commands (`tr init` + `tr repo`) and Y2 already renamed the binary to `tr`. After Y2-Y4 land, the README is stale: it tells users to run `total-recall init` (wrong command name post-Y2), it conflates user-config setup and repo-hook install into one command (wrong post-Y3), and it never guides the user to put `$GOPATH/bin` on PATH — yet Y4's post-commit hook now relies entirely on PATH resolution. If `tr` isn't on PATH, the post-commit hook silently fails; Y5 fixes this with a `tr init` startup detection that prints a one-line shell-specific instruction when `tr` is not reachable via `command -v tr` / `Get-Command tr`. The detection never modifies rc files — it only warns.

## What Changes

- **README Setup section rewrite** to reflect the post-Y3 install flow:
  ```sh
  # 1. Install the binary (one-time, user-level)
  go install github.com/colinwilliams91/total-recall@latest
  #    → places `tr` (or `tr.exe`) on $GOPATH/bin (user adds $GOPATH/bin to PATH once)
  # 2. Initialize user config (run from any directory; one-time, user-level)
  tr init
  #    → prompts: conversation analysis opt-in, AI provider, API key, model
  #    → writes ~/.tr/config.yaml
  # 3. Start the daemon (long-running terminal; keep alive)
  tr serve
  #    → binds localhost:7331
  # 4. Per-repo setup (run inside each repo you want recall in)
  cd ~/projects/my-app
  tr repo
  #    → resolves .git/hooks via git rev-parse --git-path hooks
  #    → prompts for hook enablement; writes .tr.yaml; installs hooks
  ```
  Update each existing Setup subsection to match the new flow. Update examples throughout the README: every `total-recall` CLI invocation is already `tr` post-Y2 (the Y2 task covered this, but Y5 verifies the README is consistent). The module path `github.com/colinwilliams91/total-recall` stays completely intact in the `go install ...` instruction (Y2 deliberately did not rename the module).
- **Add a non-Go install path** to the Setup section: "Download the release archive from GitHub Releases, extract, place `tr` on PATH manually. Same downstream flow."
- **Add a one-line prerequisite** near the top of Setup: "Requires Git 2.5+ (2015) for linked worktree support (`tr repo` resolves the git hooks directory via `git rev-parse --git-path hooks`, a Git 2.5+ feature)."
- **PATH detection at `tr init` startup** — new behavior in `runInit()`. Before any prompts are shown:
  1. Detect the user's shell: check `$SHELL` on Unix (bash/zsh); use PowerShell-specific detection (`$env:SHELL` is usually unset on Windows — detect by `os.Getenv("PSModulePath")` non-empty, or by `os.Getenv("OS") == "Windows_NT"`, then default to PowerShell).
  2. Run `command -v tr` (Unix) or `Get-Command tr` (Windows PowerShell via `exec.Command("powershell.exe", "-NoProfile", "-Command", "(Get-Command tr -ErrorAction SilentlyContinue).Name")`).
  3. If `tr` is found, proceed silently.
  4. If `tr` is NOT found, print a warning to stderr with the exact one-line command for the detected shell to add `$GOPATH/bin` to PATH:
     - Bash: `echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc`
     - Zsh:  `echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc`
     - PowerShell: `Add-Content $PROFILE 'Add-$(go env GOPATH)/bin to PATH'` (or similar — see implementation)
  5. Continue with the normal `tr init` flow regardless of detection result. The detection NEVER writes to rc files automatically; it only prints the recommended one-line command for the user to copy-paste.
- **Optional subtask (decide during Y5 implementation): also run the PATH detection at `tr repo` startup.** Imagine a user who skips `tr init` and runs `tr repo` first by absolute path (`~/Downloads/tr repo`). Without `tr init`'s warning, Y4's post-commit hook installs and silently fails on first commit. Capturing the warning at `tr repo` startup closes that gap. Optional because the documented install flow includes `tr init` before `tr repo`; the warning-at-`tr repo` is a defensive nicety.
- The existing `tr config --show` and `tr status` README mentions need to be updated to use `tr` (post-Y2 these are likely already correct, but Y5 verifies).

**Non-changes (deliberate):**

- No auto-PATH-modification. The tool never writes to `~/.bashrc`, `~/.zshrc`, `$PROFILE`, or any shell rc file. The warning is printed, the user pastes the command. (Anchoring constraint.)
- No "tr install doctor" / "tr install check" subcommand — the detection runs inline in `tr init` (and optionally `tr repo`), not as a separate tool.
- No introduction of a shell-rc-file auto-mutation feature under any flag.
- No change to the Go module path (`github.com/colinwilliams91/total-recall`) — Y2 left it intact and Y5 builds on that.
- No rollback guidance in the README (an OpenSpec-propose design.md may discuss rollback, but the README itself is silent on it).

## Capabilities

### New Capabilities

- `path-detection-warning`: behavior of `tr init` (and optionally `tr repo`) on startup to detect whether `tr` is reachable via `command -v tr`/`Get-Command tr` and warn the user with a shell-specific one-line PATH-fix command if not.

### Modified Capabilities

- `init-ai-setup`: `tr init` performs the PATH-detection check BEFORE any AI-provider prompts (this is the same `init-ai-setup` capability that Y3 modified to remove the hooks section; Y5 adds the startup-detection requirement on top).

## Impact

- **Code edited:**
  - `cmd/tr/main.go`'s `runInit()` (function as defined post-Y3) is prepended with a `checkTrOnPath()` call that performs the detection + optional warning, then proceeds with the existing AI-provider form.
  - (Optional, see Non-changes) `cmd/tr/main.go`'s `runRepo()` is also prepended with the same `checkTrOnPath()` call.
- **Code added:** new helper function `checkTrOnPath()` (location: `cmd/tr/main.go` or a new `cmd/tr/pathdetect.go` file — implementation's choice). On Unix, runs `exec.Command("sh", "-c", "command -v tr")` and inspects the output. On Windows, runs `exec.Command("powershell.exe", "-NoProfile", "-Command", "(Get-Command tr -ErrorAction SilentlyContinue).Name")`.
- **README updated:** Setup section rewritten; PATH instruction added; Git 2.5+ prerequisite line added.
- **Tests added:** `cmd/tr/main_test.go` (or new `pathdetect_test.go`) — test the shell-specific detection + warning output using a fake/limited environment (e.g. set PATH to an empty/non-tr-contained directory; capture stderr; assert on the expected shell-specific advisory string). The cross-platform test is harder — gate on `runtime.GOOS`.
- **Layers affected:** the **binary** layer (new startup behavior); the **user config** layer (no change — the function doesn't write config); the **repo config** layer (no change — the function doesn't write to `.tr.yaml`). No other layers are touched.
- **Dependencies:** none added or removed.