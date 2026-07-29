## Context

The current `runInit()` (`cmd/total-recall/main.go:129-273`) is a single command that conflates user-config setup and repo-level hook install. The function:

1. Calls `config.UserConfigPath()`.
2. Loads existing user config (`config.LoadUserConfig()`).
3. Runs a huh form for conversation-analysis opt-in.
4. Runs `runInitAI(&cfg)` which prompts for AI provider, API key, model (the user-level prompts).
5. Writes `~/.tr/config.yaml` via `config.WriteUserConfig`.
6. Calls `hooks.FindRepoRoot()` and if not in a git repo, prints a "skipping hook installation" advisory and returns.
7. Loads existing `.tr.yaml` from the repo root.
8. Runs a huh form for hook enablement (pre-commit, commit-msg, pre-push).
9. Writes `.tr.yaml` via `config.WriteRepoConfig`.
10. Calls `hooks.NewInstaller(repoRoot).InstallEnabled(repoCfg.Hooks)` — installs the three dispatch hooks.
11. Captures `os.Executable()` and writes the post-commit hook to `<repoRoot>/.git/hooks/post-commit`.

Steps 1-5 are user-level. Steps 6-11 are repo-level. Y3 splits them. Step 11 contains the worktree bug (Issue 6) and the Y4 path-baking concern (Issue 5).

Existing layer model: this change touches the **binary** layer (new `tr repo` subcommand; `tr init` no longer touches hooks), the **repo config** layer (still written by `tr repo`, no behavioral change), and the **git hooks** layer (still installed by `tr repo`, but the install path resolves via `git rev-parse --git-path hooks`). The leak-point rows in `INSTALL_LAYERS.md` are updated (the leak mechanism is unchanged — installer writes to gitdir — only the *invocation* command name changes).

Relevant explore-phase decisions:
- Issue 5: Y4 hook PATH strategy — chosen 5.B (rely on PATH). Y4 may fold into Y3, but the post-commit template content is Y4's territory per the plan. This design treats the post-commit template body as out of scope for Y3 unless Y4 explicitly folds in.
- Issue 6: Y3 hooks-dir resolution — chosen 6.A (resolve once in `tr repo`, pass `hooksDir` into `NewInstaller`).

## Goals / Non-Goals

**Goals:**
- `tr init` writes only `~/.tr/config.yaml`; it does not call `hooks.FindRepoRoot()`; it does not mention hooks; it ends with a printed next-step line.
- `tr repo` writes only `.tr.yaml` and `.git/hooks/*`; it fails hard outside a git repo with the exact advisory `Total Recall only works with git projects. cd into a project and run tr repo.`
- `tr repo` resolves the actual hooks dir via `git rev-parse --git-path hooks` (worktree-aware) and uses it for both the dispatch hooks (via the installer) and the post-commit hook install path.
- The Installer's `NewInstaller` API takes the resolved `hooksDir` directly (no internal computation).
- Re-running `tr init` does not re-prompt hook questions; re-running `tr repo` does not re-prompt user-level questions.

**Non-Goals:**
- No new `tr config set` subcommand (deferred per the prompt's "optional subtask — decide during explore" framing — decision: defer).
- No interactive "wizard" that runs `tr init` + `tr repo` back-to-back.
- No detection of existing installs (Y5 owns detection-of-tr-on-PATH; Y3 doesn't check).
- No version-handshake between installed hooks and the binary (deferred per `INSTALL_LAYERS.md` known-issue).
- No change to the post-commit hook *template content* unless Y4 is folded in (Issue 5 left that as Y4's design decision). Y3 only changes the *destination path* of the post-commit install, not the template.
- No change to the `tr config --show` subcommand (it stays as the existing `configCmd()`).
- No change to `tr status` (Y1 owns the advisory enhancement there; Y3 doesn't touch it).

## Decisions

### Decision 1 — Split via Cobra commands, not via flags on `init`

`tr init` is the user-level command; `tr repo` is the repo-level command. No `tr init --no-hooks`, no `tr init --user-only`, no `tr repo --init-user`. They are two distinct Cobra commands with separate `RunE` functions (`runInit()` and `runRepo()`).

**Rationale:** the anchoring constraint from the parent plan says "`tr init` and `tr repo` are STRICTLY separate commands." A flag-based split would violate that explicit constraint and re-conflate the mental model. The two halves belong to different layer boundaries (user-config vs repo-config+git-hooks); Cobra commands make that boundary explicit at the CLI syntax level.

### Decision 2 — `tr init` is silent about git and hooks; `tr repo` knows nothing about AI provider

`tr init`'s printed output ends with exactly: `Next: cd into your project and run tr repo.` — that's the only mention of `tr repo` from `tr init`. `tr init` does NOT call `hooks.FindRepoRoot`, does NOT call `hooks.ResolveHooksDir`, does NOT import the hooks package at all (verify: cmd layer dependency to internal/hooks stays as it is for `tr repo`'s path; `tr init`'s function may not need it after the split).

`tr repo`'s flow:
1. `repoRoot, err := hooks.FindRepoRoot()` — if err: print exact advisory, `os.Exit(1)`.
2. `hooksDir, err := hooks.ResolveHooksDir(repoRoot)` — if err: print error advisory, `os.Exit(1)` (this is a "git rev-parse git-path hooks failed" error; the exact message is up to the implementation).
3. Load existing `.tr.yaml` from `repoRoot` for pre-population.
4. Run huh form for pre-commit/commit-msg/pre-push enablement (existing logic, lifted verbatim from `runInit`).
5. Write `.tr.yaml` via `config.WriteRepoConfig(repoRoot, repoCfg)`.
6. `installer := hooks.NewInstaller(repoRoot, hooksDir)`. Call `installer.InstallEnabled(repoCfg.Hooks)`.
7. Print the post-commit hook explanation + install the post-commit hook via `os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte(postCommitScript), 0o755)`.

**Rationale:** the split is symmetric — each command does exactly its half. `tr repo`'s exact-not-in-repo error message is an anchoring constraint verbatim from the prompt; we don't soften or reword it. `tr init`'s next-step message is the only handoff pointer — short and explicit.

### Decision 3 — `hooks.ResolveHooksDir(repoRoot)` helper (new function, Issue 6 Option 6.A)

New function in `internal/hooks/install.go`:

```go
// ResolveHooksDir runs `git rev-parse --git-path hooks` to resolve the
// actual hooks directory (correct for linked worktrees where .git is a
// pointer file). Relative results are joined with repoRoot to produce
// an absolute path.
func ResolveHooksDir(repoRoot string) (string, error) {
    out, err := exec.Command("git", "rev-parse", "--git-path", "hooks").Output()
    if err != nil {
        return "", fmt.Errorf("resolving git hooks dir: %w", err)
    }
    hooksDir := strings.TrimSpace(string(out))
    if !filepath.IsAbs(hooksDir) {
        hooksDir = filepath.Join(repoRoot, hooksDir)
    }
    return hooksDir, nil
}
```

`tr repo` calls this once, then passes the resolved `hooksDir` into `NewInstaller(repoRoot, hooksDir)` AND uses it for the post-commit path: `filepath.Join(hooksDir, "post-commit")`.

**Rationale (Issue 6 Option 6.A):** single source of truth — both consumers (installer and post-commit write) get the same resolved dir. Two git shell-outs is wasteful and inconsistent (one could succeed, the other fail). Options 6.B and 6.C were rejected because either duplicating the shell-out or moving the post-commit write into a new Installer method adds churn without benefit; Y3 is about splitting the command, not refactoring the Installer's API surface beyond the necessary signature change.

### Decision 4 — `NewInstaller(repoRoot, hooksDir)` signature change

`internal/hooks/install.go`'s `NewInstaller(repoRoot string) *Installer` becomes `NewInstaller(repoRoot string, hooksDir string) *Installer`. The `hooksDir: filepath.Join(repoRoot, ".git", "hooks")` line (install.go:27) becomes `hooksDir: hooksDir`.

**Rationale:** exposes the seam that Issue 6 requires without internal lazy-resolution. The Installer continues to hold the `hooksDir` field; just now it's set from a caller-provided arg rather than computed inline. The existing `Install()` and `InstallBat()` methods use `inst.hooksDir` unchanged.

### Decision 5 — Post-commit install path no longer hardcodes `.git/hooks`

Today at `main.go:265`: `postCommitPath := filepath.Join(repoRoot, ".git", "hooks", "post-commit")`. After Y3: `postCommitPath := filepath.Join(hooksDir, "post-commit")` where `hooksDir` is the resolved value from Decision 3.

**Rationale:** the worktree bug lives here. Fixing only the installer (`NewInstaller(repoRoot, hooksDir)`) but leaving `postCommitPath` using `filepath.Join(repoRoot, ".git", ...)` would leave the bug half-fixed: the dispatch hooks would install correctly (via Installer) but the post-commit would still write to the bogus worktree path. The worktree-fix must land in both sites; Decision 3 + Decision 5 together accomplish that.

### Decision 6 — `tr init`'s not-in-repo-no-hooks warning is deleted

The existing printout at `main.go:184-189` (`fmt.Println("\n  ⚠  Not inside a Git repository — skipping hook installation.")` etc.) is removed entirely. `tr init` doesn't know or care about git. If the user wants hooks, they run `tr repo` separately.

**Rationale:** the old printout was a "we tried to do everything and skipped half of it" notice. With the split, there's nothing to skip from `tr init` — it did its whole job (wrote config.yaml). Mentioning git would be a leak of Y3's strict boundary.

### Decision 7 — Re-run behavior: each command re-prompts only its half

`tr init` re-run loads existing `~/.tr/config.yaml`, pre-populates the prompts from it, user can change answers, write back. No hook-related state is consulted at all.

`tr repo` re-run loads existing `.tr.yaml` from the repo root, pre-populates the hook-enablement prompts from it, user can change selections, write back, re-install hooks. No user-config state is consulted at all.

**Rationale:** matches the "each command does exactly its half" boundary. Existing pre-population logic in `runInit` (lines 137-152 for user config, lines 193-202 for repo config) splits cleanly — each half moves to its respective command.

### Decision 8 — `tr config set` subcommand is deferred

The parent prompt marked `tr config set <key> <value>` as "Optional subtask — decide during explore." During explore, the decision is **defer** to a future phase. Rationale: Y3 already adds a new subcommand (`tr repo`) + API surface (`NewInstaller` signature change + `ResolveHooksDir`); adding `tr config set` expands the API surface further and adds test scope. The existing workaround — edit `~/.tr/config.yaml` in a text editor — is acceptable for the no-userbase present. If a future user struggles with this, the subcommand can be added as its own small change proposal.

### Decision 9 — Y4 folding into Y3 is allowed but not required

The parent prompt noted: "Y4 may fold into Y3 if Y3 is already touching the post-commit installer." Y3's scope (Decisions 1-8 above) alters the post-commit *install path* but NOT the post-commit *template body* (`postCommitHookScriptTmpl` + `buildPostCommitHookScript` in main.go:101-117). Y4 owns the template body.

Two paths:
- **Y4 stays separate.** Y3 ends with the post-commit install path fixed (worktree-correct) and the template body unchanged (still captures `os.Executable()` via `%s` insertions). Y4 in a follow-up diff modifies the template body only.
- **Y4 folds in.** Y3's diff also modifies `postCommitHookScriptTmpl` to drop the `%s` insertions (Issue 5 Decision 5.B: rely on `tr ask` via PATH). The Y3 spec delta for `post-commit-hook` then simultaneously captures the path-fix AND the template change.

Y3 ships without bundling Y4 by default. If a reviewer suggests folding them to reduce churn on `main.go`, that's fine and the timeline is unchanged. The design is written to accommodate either.

## Risks / Trade-offs

- **Risk:** the existing `init-ai-setup` spec text says "before the hooks section" in its first requirement — after Y3, `tr init` has no hooks section, so the wording becomes stale. → **Mitigation:** the Y3 spec delta MODIFIED requirement drops the "before the hooks section" qualifier. Captured in specs/init-ai-setup/spec.md.
- **Risk:** `tr repo` requires `hooksDir` to be non-empty; if `git rev-parse --git-path hooks` somehow returns an empty string (would only happen with a broken git install), the post-commit path and installer silently behave oddly (`filepath.Join("", "post-commit")` → `"post-commit"` writes to CWD). → **Mitigation:** `ResolveHooksDir` returns an error on empty output (`if hooksDir == "" { return "", fmt.Errorf("git rev-parse --git-path hooks returned empty") }`). `tr repo` propagates the error and `os.Exit(1)`.
- **Risk:** `git rev-parse --git-path hooks` was introduced in Git 2.5 (2015). Older Git versions return an error. → **Mitigation:** Y5 (docs) will note "Git 2.5+ (2015) required for linked worktree hook resolution." Y3 itself doesn't implement a version check — the error message from `exec.Command` is sufficient.
- **Risk:** anyone running the previous `tr init` (old binary) inside a worktree may have a stale `post-commit` in the wrong location (inside the worktree's `.git/` pointer area, which may have written to a non-existent directory or aborted). → **Mitigation:** no users exist; on the next `tr repo` (new binary), the correct path is used and the stale file (if any) is irrelevant. No migration.
- **Risk:** tests that exercise `runInit` as a single function break. → **Mitigation:** the test layer splits alongside the source layer; main_test.go gets a `tr repo` test, integration_test.go's `startTestDaemon` is unaffected (it constructs a `*Server` directly, doesn't invoke `runInit`), but any test that invoked `runInit()` is retargeted to either `runInit()` (user-level) or `runRepo()` (repo-level). Documented in tasks.
- **Trade-off:** keeping Y4 as a separate phase means the post-commit hook template (in Y3's diff) still captures `os.Executable()` — a known leak point until Y4 lands. If the two phases land back-to-back this is a temporary state of minutes-to-hours. Acceptable.

## Migration Plan

This is a code-only change with no DB schema or config file impact.

1. Implement Y3 (`init-repo-split`) per tasks.md.
2. Existing user-config (`~/.tr/config.yaml`) is unaffected — `tr init` re-run with the new binary continues to write to the same file in the same format.
3. Existing repo-configs (`.tr.yaml` files inside any repo) are unaffected — `tr repo` re-run reads and pre-populates from them.
4. Existing installed hooks in `.git/hooks/` are detected by the installer's `IsManagedHook` (sentinel `# total-recall managed` unchanged) and overwritten with the post-Y3-y2 content. No manual cleanup needed.
5. If the user previously installed hooks inside a linked worktree via the old buggy path, those stale files (if any) are not cleaned up by Y3. The next `tr repo` writes correctly to the common gitdir; the old worktree-local files (if any) become dead artifacts. In practice, the old bug typically *prevented* successful install in worktrees (the path didn't resolve), so there are no stale files to clean.

**Rollback:** revert the commit. The previous single `tr init` flow resumes (modulo any Y2 rename that may have landed). No data migration to undo.

## Open Questions

- **Y4 fold-or-not decision:** allowed as a reviewer judgment call during implementation; not a hard Y3 blocker.
- **Future `tr config set` subcommand:** explicitly deferred; revisit if user pain is observed.
- **Should `tr repo --force` exist to reinstall even when the sentinel-detection views the existing hooks as unmanaged (e.g., user previously had a manually-written pre-commit that was chained)?** No — the existing installer already has chain-detection logic (`IsManagedHook → false → read existing content → chain into new script`). `tr repo --force` would only matter if Y2 had renamed the sentinel (it didn't). Not needed.