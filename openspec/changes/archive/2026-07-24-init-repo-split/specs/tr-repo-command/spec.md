## ADDED Requirements

### Requirement: tr repo resolves the actual git hooks directory via git rev-parse
`tr repo` SHALL resolve the hooks directory by running `git rev-parse --git-path hooks` (NOT the `filepath.Join(repoRoot, ".git", "hooks")` shortcut which is wrong inside linked worktrees where `.git` is a pointer file). If the resolved path is relative, it SHALL be absolutized via `filepath.Join(repoRoot, resolved)` (git may emit a relative `.git/hooks` in normal repos). The absolutized result SHALL be used for both the dispatch hooks installation (via `hooks.NewInstaller(repoRoot, hooksDir)`) and the post-commit install (`os.WriteFile(filepath.Join(hooksDir, "post-commit"), ...)`).

The helper `hooks.ResolveHooksDir(repoRoot string)` SHALL implement the above logic and be idempotent (same repo root + same git configuration always yields the same result). It SHALL return an error when:
- `exec.Command("git", "rev-parse", "--git-path", "hooks").Output()` fails (git unavailable or unsupported git version), OR
- the trimmed output is empty (defensive — git normally never returns empty for this command).

On error, `tr repo` SHALL print the error advisory and exit non-zero.

#### Scenario: Normal repo (no worktree)
- **WHEN** `hooks.ResolveHooksDir("/path/to/repo")` is called where `/path/to/repo` is a normal git repository
- **THEN** git outputs `.git/hooks` (relative), `ResolveHooksDir` returns `/path/to/repo/.git/hooks` (absolutized via `filepath.Join`)

#### Scenario: Linked worktree
- **WHEN** `hooks.ResolveHooksDir("/path/to/worktree")` is called where `/path/to/worktree` is a linked worktree (created via `git worktree add`)
- **THEN** git outputs the COMMON gitdir's hooks path (e.g. `/path/to/main-repo/.git/hooks` — absolute), `ResolveHooksDir` returns that absolute path unchanged (no `filepath.Join` since the path is already absolute)

#### Scenario: Git command fails
- **WHEN** `exec.Command("git", "rev-parse", "--git-path", "hooks").Output()` fails (no git on PATH, or git version older than 2.5 which doesn't support `--git-path hooks`)
- **THEN** `ResolveHooksDir` returns `( "", error )` with the wrapped error message; `tr repo` prints the error and `os.Exit(1)`

---

### Requirement: tr repo writes .tr.yaml with hook enablement
`tr repo` SHALL run a huh form prompting the user to enable each of: pre-commit, commit-msg, pre-push. The form SHALL pre-populate from existing `.tr.yaml` if present in the resolved repo root. After confirmation, `tr repo` SHALL write `.tr.yaml` via `config.WriteRepoConfig(repoRoot, repoCfg)`. The form SHALL NOT touch `~/.tr/config.yaml` (user-level config is `tr init`'s territory). Re-running `tr repo` re-prompts only the hook-enablement questions.

#### Scenario: Fresh tr repo with existing .tr.yaml
- **WHEN** `tr repo` is run in `/path/to/repo` and `.tr.yaml` exists with `hooks.pre-commit: true, hooks.commit-msg: false, hooks.pre-push: false`
- **THEN** the form pre-populates with `pre-commit=true`, `commit-msg=false`, `pre-push=false`; the user confirms or changes; the new state is written back to `/path/to/repo/.tr.yaml`

#### Scenario: Fresh tr repo with no existing .tr.yaml
- **WHEN** `tr repo` is run in `/path/to/repo` and no `.tr.yaml` exists
- **THEN** the form pre-populates with `pre-commit=true` (sensible default), `commit-msg=false`, `pre-push=false`; the user confirms; `.tr.yaml` is written fresh

---

### Requirement: tr repo installs dispatch hooks via the resolved hooks dir
After writing `.tr.yaml`, `tr repo` SHALL call `hooks.NewInstaller(repoRoot, hooksDir)` (where `hooksDir` is the absolutized result from `ResolveHooksDir`) and then `installer.InstallEnabled(repoCfg.Hooks)`. The install SHALL write the enabled hook scripts (pre-commit, commit-msg, pre-push and their `.bat` siblings) into `<hooksDir>`. Existing managed hooks (detected via the `# total-recall managed` sentinel) are overwritten in place. Existing unmanaged hooks are chained before the TR dispatch body.

#### Scenario: Fresh install in a normal repo
- **WHEN** `tr repo` runs in a normal repo with no prior hooks
- **THEN** the three dispatch hook scripts (and their `.bat` variants) are written into `<repo>/.git/hooks/`

#### Scenario: Fresh install in a linked worktree
- **WHEN** `tr repo` runs in a linked worktree
- **THEN** the three dispatch hooks land in the COMMON gitdir's hooks directory (e.g. `/path/to/main-repo/.git/hooks/`), shared across the worktree and the main checkout

#### Scenario: Existing managed hooks are replaced
- **WHEN** `tr repo` runs and finds an existing `<hooksDir>/pre-commit` whose first 5 lines match the `# total-recall managed` sentinel
- **THEN** the file is overwritten with the current managed hook body (no chaining of an existing managed hook)

#### Scenario: Existing unmanaged hooks are chained
- **WHEN** `tr repo` runs and finds an existing `<hooksDir>/pre-commit` whose first 5 lines do NOT match the sentinel
- **THEN** the existing content is chained before the TR dispatch body (per the existing installer chain pattern), and the new file is written

---

### Requirement: tr repo installs the post-commit hook via the resolved hooks dir
After the dispatch hooks are installed, `tr repo` SHALL install the post-commit hook at `<hooksDir>/post-commit` via `os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte(postCommitScript), 0o755)`. The post-commit install path is `<hooksDir>/post-commit` — NEVER `filepath.Join(repoRoot, ".git", "hooks", "post-commit")` — because that combination breaks in linked worktrees.

If the write fails (e.g. permissions), `tr repo` SHALL print a warning but continue (the dispatch hooks are already installed; the post-commit hook is a single recall-question surface; its absence degrades UX but doesn't break the repo).

#### Scenario: Normal repo post-commit install
- **WHEN** `tr repo` runs in a normal repo and the dispatcher hooks installation succeeds
- **THEN** the post-commit hook is written to `<repo>/.git/hooks/post-commit` (the resolved hooks dir equivalent)

#### Scenario: Linked worktree post-commit install
- **WHEN** `tr repo` runs in a linked worktree and the dispatcher hooks installation succeeds
- **THEN** the post-commit hook is written to the COMMON gitdir's `post-commit` path (e.g. `/path/to/main-repo/.git/hooks/post-commit`), not the worktree-local path

#### Scenario: Post-commit write permission failure
- **WHEN** `os.WriteFile` fails for the post-commit path (e.g. read-only gitdir)
- **THEN** `tr repo` prints `⚠  Could not install post-commit hook: <err>` and continues; the command exits 0 (no error propagation), and a clear note explains the consequence