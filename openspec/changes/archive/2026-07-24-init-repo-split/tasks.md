## 1. New helper: hooks.ResolveHooksDir

- [x] 1.1 In `internal/hooks/install.go`, add a new function `ResolveHooksDir(repoRoot string) (string, error)` that runs `exec.Command("git", "rev-parse", "--git-path", "hooks").Output()`. On error, return `("", fmt.Errorf("resolving git hooks dir: %w", err))`. Otherwise trim the output and if it's not absolute (`!filepath.IsAbs(hooksDir)`), absolutize via `filepath.Join(repoRoot, hooksDir)`. Also guard against empty trimmed output (`if hooksDir == "" { return "", fmt.Errorf("git rev-parse --git-path hooks returned empty") }`).

## 2. Installer signature change

- [x] 2.1 In `internal/hooks/install.go`, change `NewInstaller(repoRoot string) *Installer` to `NewInstaller(repoRoot string, hooksDir string) *Installer`. The struct-literal initializer at line 26 changes `hooksDir: filepath.Join(repoRoot, ".git", "hooks")` to `hooksDir: hooksDir`. All other fields unchanged.
- [x] 2.2 Update callers of `NewInstaller` (grep for `hooks.NewInstaller` across the repo). Likely caller: `cmd/tr/main.go`'s `runRepo()` after the split. Update those call sites to first call `ResolveHooksDir(repoRoot)` then pass the result in.

## 3. Split runInit into runInit (user-level) + runRepo (repo-level)

- [x] 3.1 In `cmd/tr/main.go`, refactor `runInit()` (line 129) into two functions: `runInit()` (lines 1-5 of the existing flow — load user config, run conversation-analysis huh form, run `runInitAI(&cfg)`, write `~/.tr/config.yaml`, print "✓ User config saved to <path>", then print the next-step guidance `Next: cd into your project and run tr repo.` and return nil). Remove the entire "─── Hook installation ───" block (lines 184-272).
- [x] 3.2 In `cmd/tr/main.go`, add `runRepo()` as a new function. Order of operations:
  - `repoRoot, err := hooks.FindRepoRoot()` — if err, print exactly `Total Recall only works with git projects. cd into a project and run tr repo.` and call `os.Exit(1)` (return non-zero).
  - `hooksDir, err := hooks.ResolveHooksDir(repoRoot)` — if err, print error and `os.Exit(1)`.
  - Load existing `.tr.yaml` via `config.LoadRepoConfigFromDir(repoRoot)`. Pre-populate enablement flags from it; default `preCommit=true`, `commitMsg=false`, `prePush=false` on first run.
  - Run huh form for the three hook-enablement prompts (lift verbatim from the original `runInit` hook-form block at lines 205-220).
  - Write `.tr.yaml` via `config.WriteRepoConfig(repoRoot, repoCfg)`.
  - `installer := hooks.NewInstaller(repoRoot, hooksDir)`. Call `installer.InstallEnabled(repoCfg.Hooks)`.
  - Print the post-commit-hook explanation block (lift verbatim from lines 253-258).
  - Write post-commit hook: `postCommitPath := filepath.Join(hooksDir, "post-commit")` (NOT `filepath.Join(repoRoot, ".git", "hooks", "post-commit")`); call `os.WriteFile(postCommitPath, []byte(postCommitScript), 0o755)` with the existing error handling (print `⚠  Could not install post-commit hook: %v` on failure but continue).
- [x] 3.3 Add `repoCmd()` returning a new Cobra command: `&cobra.Command{Use: "repo", Short: "Install Total Recall hooks in this git repository", RunE: func(cmd, args) error { return runRepo() }}`.
- [x] 3.4 Update `root.AddCommand(...)` in `main()` to include `repoCmd()` alongside `initCmd()`.
- [x] 3.5 Update `initCmd()`'s `Short` description: change from "Initialize Total Recall for this project and create user config" to "Configure Total Recall user-level settings (AI provider, API key, model)". This better reflects the split.
- [x] 3.6 Remove the `main.go:184-189` not-in-a-git-repo "skipping hook installation" warning from `runInit`. `runInit` no longer touches git.

## 4. Tests — main_test.go

- [x] 4.1 `buildPostCommitHookScript` tests stay unchanged UNLESS Y4 folded in (see tasks 7.1 below if Y4 folds).
- [x] 4.2 Add a test `TestRunRepoOutsideGitFailsWithExactMessage`: invoke `runRepo` with `cwd` outside any git repo; assert the exact output string `Total Recall only works with git projects. cd into a project and run tr repo.` and `os.Exit(1)` is called (use a `subprocess` pattern or extract the message-print+exit into a helper that can be stubbed in tests).
- [x] 4.3 Add a test `TestResolveHooksDirAbsolutizesRelative`: invoke `ResolveHooksDir` against a normal test fixture repo; assert the returned path is absolute and ends in `.git/hooks`.
- [x] 4.4 Add a test `TestResolveHooksDirWorktree`: create a scratch git repo, `git worktree add` a sibling directory, invoke `ResolveHooksDir(siblingPath)`, assert the returned path is ABSOLUTE and points to the COMMON gitdir (the main repo's `.git/hooks`, not the worktree-local pointer file). Per `INSTALL_LAYERS.md` "Canonical Testing Simulation" rules: the scratch repo must be its own repo, not nested in any existing repo.
- [x] 4.5 Existing `buildPostCommitHookScript` tests verify the hook template body. Y3 doesn't change the template; no edits needed unless Y4 folded.

## 5. Tests — integration_test.go

- [x] 5.1 Any integration test that called `runInit()` to install hooks is split: tests that exercise dispatcher-hook install → invoke `runRepo` instead; tests that exercise user-config creation → invoke `runInit`. Grep `cmd/tr/integration_test.go` for `runInit` calls and audit each.
- [x] 5.2 Add an integration test `TestRepoInstallsIntoCommonGitdirInWorktree`: scratch git repo in a temp dir, `git worktree add` a sibling, invoke `runRepo` inside the worktree, assert installed hooks land in the MAIN repo's `.git/hooks/` directory (not the worktree-local pointer). This is a headless integration test using `net.Listen + engine.Serve()` if the test implies daemon communication; if it only tests the installer, no daemon is needed and a direct `runRepo` call suffices.

## 6. README/docs updates

- [x] 6.1 README's Setup section (or equivalent) is updated to reflect the split: `tr init` writes user config; `tr repo` (run from inside a project) installs hooks. The new-user install flow becomes: `go install ...` → `tr init` (any directory) → `tr serve` (separate terminal) → `cd <project>` → `tr repo` → `git commit -m ...`. The Y5 phase owns the full rewrite; Y3 just clarifies the two-command split wherever the README implied a single `tr init` did both.
- [x] 6.2 `DOCS/ARCHITECTURE/INSTALL_LAYERS.md` updates: (a) the leak-point table row "Binary → hooks (content)" changes from "written by `init`" to "written by `tr repo`" (or similar verbiage); (b) the "New-User Install Flow" block (lines 68-100) updates step 4 from `total-recall init` → `tr repo` (and step 2 from `total-recall init` to `tr init` for the user-level step — though this was already done by Y2; verify Y2's rename touched here too).

## 7. Y4 fold-in (conditional — execute only if reviewer decides to fold Y4 into Y3)

- [x] 7.1 In `cmd/tr/main.go`, modify `postCommitHookScriptTmpl` (line 101-111) to drop the `%s` insertions. New template body:
```sh
#!/bin/sh
# total-recall managed
# Surfaces a recall question after each successful commit.
# Generated by 'tr repo'. Run 'tr repo' again to reinstall.
if command -v powershell.exe >/dev/null 2>&1; then
    powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "tr ask"
    exit $?
fi

exec tr ask
```
- [x] 7.2 Delete `buildPostCommitHookScript(selfPath string)` (the function now takes no args and returns a constant — or inline the constant directly at the call site). Delete the `os.Executable()` capture at line 259.
- [x] 7.3 `postCommitScript := buildPostCommitHookScript()` — adjust call site in `runRepo` to use the new no-arg function (or inline `postCommitHookScriptTmpl` directly).
- [x] 7.4 Update `cmd/tr/main_test.go`'s `buildPostCommitHookScript` tests to assert on the new constant template content (no `%s` substitution). The tests become shorter — they're asserting a static string.
- [x] 7.5 If Y4 folds in, also mark Y4 as folded-completed in the Y4 change proposal (or skip Y4's standalone proposal and reference this Y3 task in Y4's tasks as "folded into init-repo-split task 7).

## 8. Verification

- [x] 8.1 `go build ./... && go vet ./... && go test ./...` — all must pass.
- [x] 8.2 Manual smoke test: from outside a git repo, run `tr repo` — should print the exact not-in-repo message and exit non-zero.
- [x] 8.3 Manual smoke test: inside a linked worktree, run `tr repo` — should install hooks into the common gitdir, not the worktree-local pointer file. Verify by `ls /path/to/main/.git/hooks/` (should contain `pre-commit`, `commit-msg`, `pre-push`, `post-commit` post-`tr repo`).
- [x] 8.4 `openspec validate init-repo-split` passes; spec deltas parse and validate.
