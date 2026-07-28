## Purpose

Generate a Git `post-commit` hook via `tr repo` that surfaces a recall question in the terminal after each successful commit by invoking `tr ask` via the baked absolute binary path.
## Requirements
### Requirement: tr repo generates a post-commit hook
`tr repo` SHALL resolve the actual git hooks directory via `git rev-parse --git-path hooks` (NOT the `filepath.Join(repoRoot, ".git", "hooks")` shortcut, which is wrong inside linked worktrees where `.git` is a pointer file). It SHALL generate `<resolved-hooks-dir>/post-commit` containing a shell script that calls `tr ask`. The file SHALL be made executable (`chmod 0755`). The generation SHALL be idempotent — running `tr repo` again SHALL overwrite the hook cleanly.

#### Scenario: Fresh tr repo in a normal (non-worktree) repository
- **WHEN** `tr repo` is run in a normal repo with no post-commit hook
- **THEN** `<resolved-hooks-dir>/post-commit` is created, is executable, and contains `exec "<baked-path>" ask` where `<baked-path>` is the forward-slash form of `os.Executable()` at `tr repo` time

#### Scenario: Fresh tr repo in a linked worktree
- **WHEN** `tr repo` is run inside a linked worktree (`git worktree add` had been used)
- **THEN** the resolved hooks dir resolves to the COMMON gitdir (shared across the worktree and the main checkout), and the post-commit hook is written there; subsequent commits in any linked worktree of the same repo fire the same post-commit hook

#### Scenario: Re-run tr repo
- **WHEN** `tr repo` is run again in a repo that already has a post-commit hook
- **THEN** the hook is overwritten with the current template content; no error is raised

---

### Requirement: Post-commit hook calls tr ask via baked binary path
The generated post-commit hook SHALL call `tr ask` via `exec "<baked-path>" ask` where `<baked-path>` is the absolute path of the `tr` binary captured at `tr repo` time via `os.Executable()`, converted to forward slashes for the sh fallback branch. The PowerShell branch SHALL use the native backslash path single-quoted (`& '<baked-path>' ask`). Using `exec` replaces the shell process, keeping process count minimal.

The baked-path approach avoids PATH collision with the Unix `tr` translate utility (`/usr/bin/tr`), which is present in the restricted PATH that Git for Windows uses when running hooks via MSYS sh. PATH-based lookup (`exec tr ask`) silently invokes the wrong binary on such systems.

Trade-off: moving or rebuilding the binary to a new path leaves the installed hook pointing at the old path. Re-running `tr repo` refreshes the baked path. This is preferable to the PATH-collision failure mode because the stale-path error is loud (`No such file or directory` pointing at the dead path) and only triggers on binary relocation, whereas the collision is silent and occurs on every commit.

#### Scenario: Post-commit hook fires with the correct binary
- **WHEN** Git fires the post-commit hook after a successful commit and the baked path still points at a valid `tr` binary
- **THEN** `exec "<baked-path>" ask` runs the Total Recall binary and the recall-question TUI runs in the same terminal session as the git commit

#### Scenario: Post-commit hook fires after a binary move (stale path)
- **WHEN** the user moves or rebuilds the `tr` binary to a new path after `tr repo` was run
- **THEN** `exec "<baked-path>" ask` fails with `No such file or directory` pointing at the old path; the commit itself is unaffected (post-commit hooks run after the commit is created); the user re-runs `tr repo` to refresh the baked path

#### Scenario: No PATH collision with Unix tr on Git for Windows
- **WHEN** Git for Windows fires the post-commit hook via MSYS sh and `/usr/bin/tr` (the Unix translate utility) is on the restricted hook PATH
- **THEN** the hook executes the baked absolute path directly, bypassing PATH resolution entirely; `/usr/bin/tr` is never invoked

### Requirement: Post-commit hook is installed after existing dispatch hooks
The post-commit hook installation step SHALL run after the existing pre-commit, commit-msg, and pre-push hook installations in `runRepo()`. The install order matches the existing `runInit()` flow; only the function name changes (init → repo).

#### Scenario: Install order in tr repo
- **WHEN** `tr repo` completes hook installation
- **THEN** the post-commit hook is written after the dispatch hooks (pre-commit, commit-msg, pre-push) have been installed

---

### Requirement: tr repo TUI informs the user about tr ask
The `tr repo` TUI flow SHALL display a note or confirmation before or after installing the post-commit hook, explaining that the hook will surface a recall question in the terminal after each successful commit. (Pre-existing requirement text from `init-ai-setup`; transferred verbatim to `tr repo` per the Y3 split.)

#### Scenario: User sees the hook note
- **WHEN** the user completes the `tr repo` TUI flow
- **THEN** they have been informed that a post-commit hook was installed and will surface recall questions

---

### Requirement: tr repo hard-fails outside a git repo
`tr repo` SHALL call `hooks.FindRepoRoot()` as its first action. If it returns an error (user is not inside a git repository or git is unavailable), `tr repo` SHALL print exactly: `Total Recall only works with git projects. cd into a project and run tr repo.` and exit non-zero. No further action SHALL be taken — no `.tr.yaml` write, no hook install attempt.

#### Scenario: tr repo outside a git repo
- **WHEN** `tr repo` is run from a directory that is not inside any git repository
- **THEN** the exact message is printed and `os.Exit(1)` is called; no `.tr.yaml` is written and no hooks are installed

#### Scenario: tr repo inside a git repo
- **WHEN** `tr repo` is run inside a git repository (normal or worktree)
- **THEN** `FindRepoRoot` succeeds, the remainder of `tr repo` proceeds (resolve hooks dir, prompt hook enablement, install)

