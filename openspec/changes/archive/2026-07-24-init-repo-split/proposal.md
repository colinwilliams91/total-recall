## Why

Today `tr init` (runInit in `cmd/total-recall/main.go:129`) conflates two unrelated concerns: (a) user-level configuration (~/.tr/config.yaml — AI provider, API key, model, conversation-analysis opt-in) and (b) repo-level hook installation (.tr.yaml + .git/hooks/*). A single command does both, which means re-running it for a new repo re-prompts the user-level questions, and running it outside a git repo silently skips the hook portion without surfacing the optimal "now cd into a repo and run tr repo" next step. A new user is also expected to perform the split mentally and remember which half is which. Separating into `tr init` (user-level only) and `tr repo` (repo-level only) aligns the commands with the five-layer model — `tr init` touches the user-config + binary layers; `tr repo` touches the repo-config + git-hooks layers — and gives crisp contracts for both.

Worktree installs also break today: the post-commit hook write at `main.go:265` and the installer's hooks-dir at `install.go:27` use `filepath.Join(repoRoot, ".git", "hooks")`, which is wrong inside a linked worktree where `.git` is a pointer file, not a directory. Resolving via `git rev-parse --git-path hooks` instead fixes this for both sites uniformly.

## What Changes

- Split `runInit()` in `cmd/total-recall/main.go` (line 129) into `tr init` and `tr repo` Cobra commands with completely separate `RunE` implementations.
- **`tr init`** — run from ANY directory. Only touches user-level state.
  - Prompts: conversation-analysis opt-in (existing `enableConversationAnalysis` form), AI provider (existing `runInitAI`), API key, model.
  - Writes `~/.tr/config.yaml` only.
  - Does NOT call `hooks.FindRepoRoot`. Does NOT look at git. Does NOT mention hooks in any prompt.
  - At the end, prints: `Next: cd into your project and run tr repo.`
  - Re-running `tr init` only re-prompts user-level questions.
- **`tr repo`** — must be run from inside a git repo.
  - First action: call `hooks.FindRepoRoot()`. If it returns an error, print exactly `Total Recall only works with git projects. cd into a project and run tr repo.` and exit non-zero (`os.Exit(1)`).
  - Resolves the actual hooks folder via `git rev-parse --git-path hooks` (instead of `filepath.Join(repoRoot, ".git", "hooks")` which is wrong inside linked worktrees). Absolutize relative results via `filepath.Join(repoRoot, hooksDir)` when `filepath.IsAbs(hooksDir) == false`.
  - Prompts which hooks to enable (pre-commit, commit-msg, pre-push).
  - Writes `.tr.yaml` in the repo root.
  - Installs the chosen hooks into the resolved hooks folder via a `hooks.Installer` configured with the resolved hooks dir.
  - Installs the post-commit hook into the resolved hooks folder. The post-commit install path is `filepath.Join(resolvedHooksDir, "post-commit")` (not `filepath.Join(repoRoot, ".git", "hooks", "post-commit")` as today).
  - Re-running `tr repo` only re-prompts the hook questions.
- Change `hooks.NewInstaller(repoRoot string)` to `hooks.NewInstaller(repoRoot string, hooksDir string)` (signature change, Issue 6 Decision 6.A). The `hooksDir` field in `Installer` is set from the constructor argument rather than computed from `repoRoot` internally.
- Add a new helper `hooks.ResolveHooksDir(repoRoot string) (string, error)` that runs `git rev-parse --git-path hooks` and absolutizes relative results. `tr repo` calls this once and passes the result into both `NewInstaller` and the post-commit install path.
- Update the `tr init` TUI flow's pre-Y3 messages: the existing printout at `main.go:184-189` ("Not inside a Git repository — skipping hook installation. Run 'total-recall init' from a Git repository root to enable hooks.") goes away entirely — `tr init` no longer touches hooks.
- Y4's post-commit hook template change (drop `os.Executable()` capture) likely folds into Y3 since the post-commit install site is being rewritten for the worktree fix anyway — flagged as coincidence, not a dependency. If Y4 lands first and Y3's design is already included in Y4's work, the post-commit template just falls under Y3's scope expansion.
- Optional subtask (decided during explore): defer `tr config set <key> <value>` subcommand. It is not blocking and adds scope. Tracked as a future phase if needed. The existing `tr config --show` continues to work.

**Non-changes (deliberate):**

- No new hook type is added (no post-checkout hook — Issue 3's Option A deferred).
- No `tr install` / `tr setup` / `tr doctor` umbrella command.
- No "init wizard" auto-runs repo on first try if user is in a repo. The split is strict.
- No interactive mode toggle. Both commands are interactive via huh forms.
- The `tr config --show` subcommand inherited from the existing `configCmd()` in main.go:430 is unchanged.
- The `tr status` command (also inherited) is unchanged by Y3 (the Y1 enhancement adds the stale-questions advisory but that is independent of Y3).

## Capabilities

### New Capabilities

- `tr-repo-command`: the new `tr repo` command — installation of git hooks into the resolved gitdir (worktree-aware), plus repo-config (.tr.yaml) write. Fails hard outside a git repo with an exact-message advisory.

### Modified Capabilities

- `init-ai-setup`: `tr init` is now strictly user-level — no hook installation, no git interaction. The "init presents a named provider picker before the hooks section" requirement is amended: the "before the hooks section" qualifier is removed (hooks aren't part of init any more). Successfully writing `~/.tr/config.yaml` ends with the printed next-step guidance: `Next: cd into your project and run tr repo.`
- `post-commit-hook`: the install site resolves the actual gitdir via `git rev-parse --git-path hooks` and installs into `<resolved>/post-commit` (instead of `<repo>/.git/hooks/post-commit` which breaks in linked worktrees). The existing "tr init generates a post-commit hook" requirement is renamed to "tr repo generates a post-commit hook" in this delta (the install command changed from init to repo).
- `tr-home-override`: unchanged at the spec level — `TR_HOME` still redirects both cache and config; no new requirement for Y3.

## Impact

- **Code edited:**
  - `cmd/total-recall/main.go` (post-Y2: `cmd/tr/main.go`) — `runInit()` (line 129) is split into `runInit()` (user-level only) and `runRepo()` (repo-level only). The `initCmd()` returns the `tr init` command; a new `repoCmd()` returns `tr repo`. `main()`'s `root.AddCommand(...)` adds `repoCmd()` alongside `initCmd()`. About 100 lines reorganized but no significant new code.
  - `internal/hooks/install.go` — `NewInstaller(repoRoot string)` → `NewInstaller(repoRoot string, hooksDir string)`. The `hooksDir` field assignment at line 27 changes from `filepath.Join(repoRoot, ".git", "hooks")` to the passed-in `hooksDir`. `InstallEnabled` uses `inst.hooksDir` unchanged.
- **Code added:**
  - New `hooks.ResolveHooksDir(repoRoot string) (string, error)` helper (probably in `internal/hooks/install.go` alongside `FindRepoRoot`). Runs `git rev-parse --git-path hooks`, absolutizes relative results.
  - New `repoCmd()` function and `runRepo()` function in `cmd/tr/main.go`.
- **Tests updated:**
  - `cmd/tr/main_test.go` — `buildPostCommitHookScript` tests stay unchanged (the hook body template is Y4's concern if it bins differently; Y3 leaves the template as-is unless Y4 is folded in). New tests: `repoCmd` exits non-zero with the exact advisory message when `FindRepoRoot` fails; `repoCmd` resolves hooks dir via `ResolveHooksDir`; post-commit install path uses the resolved dir.
  - `cmd/tr/integration_test.go` — any test that boots `runInit` to install hooks needs splitting. New test: simulate `tr repo` inside a worktree and assert the post-commit hook lands in `<main-repo>/.git/hooks/post-commit` (the common gitdir), not `<worktree>/.git/hooks/post-commit`.
- **Layers affected:** the **binary** layer (new subcommand `tr repo`); the **user config** layer (still written only by `tr init`); the **repo config** layer (still written by `tr repo`); the **git hooks** layer (still installed by `tr repo`, but now resolving worktree-aware hooks dir). The five-layer model's leak points table in `INSTALL_LAYERS.md` is unchanged in structure but the binary→hooks coupling table row is updated: hooks are now installed by `tr repo`, not `tr init`.
- **Dependencies:** none added or removed.