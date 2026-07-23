# Install & Layer Architecture

Total Recall is a single Go binary distributed via `go install` (or release archives). Its runtime state is split across five independently-managed layers. Understanding these layers — and where they do and do not couple — is essential for installation, testing, worktree support, and reasoning about hook behavior.

---

## The Five Layers

| Layer | Owned by | Lifetime | Reads from | Writes to |
|---|---|---|---|---|
| **Binary** (`total-recall` / `tr.exe`) | One per user; on `$GOPATH/bin` or invoked by absolute path | Until re-install | Nothing at runtime (stateless) | Invoked for `init`, `serve`, `ask`, `config` |
| **User config** (`~/.tr/config.yaml`) | One per user | Until deleted | Runtime config loader | Written by `tr init`; never by hooks |
| **User cache** (`~/.tr/memory.db`) | One per user | Until deleted | Daemon (read concepts); write concepts/answers | Daemon only |
| **Repo config** (`.tr.yaml`) | One per repo | Until deleted | Runtime config loader | Written by `tr repo` |
| **Git hooks** (`.git/hooks/*`) | One per gitdir | Until overwritten | Fired by Git | Written by `tr repo` |
| **Daemon** (`total-recall serve` on `:7331`) | One per machine | Process lifetime | User cache + AI provider | User cache |

The binary is stateless: invoking it from any path executes the same logic against the same user-level state. Its compile-time-baked constants (hook bodies, daemon URL) are emitted into the installable artifacts at `tr repo` time, then the binary is out of the loop until the next `tr repo`.

---

## Crucial Separations

These independence properties are what make the system tractable, and violating them silently is the source of most "why didn't my change take effect?" surprises.

1. **Binary location is irrelevant to which hooks fire.** Hooks fire because Git finds executable files in `<gitdir>/hooks/`. Delete the binary after `tr repo` and the hooks still fire on every commit (they fail politely, but they fire).
2. **Binary location is irrelevant to where `tr repo` installs hooks.** `tr repo` resolves the gitdir from the *current working directory* (`hooks.FindRepoRoot()`). The binary could be on a network drive; running it from worktree A installs into A's gitdir.
3. **Hooks are static files, fully decoupled from the binary after install.** Once `tr repo` writes `.git/hooks/pre-commit`, that file is just bash. It does not invoke or read the binary. It only knows `http://localhost:7331` (a string baked into the hook body at *compile* time and emitted into the script at *`tr repo`* time).
4. **Daemon is fully decoupled from binary and hooks.** Hooks are HTTP clients; daemon is an HTTP server. They share only the URL `http://localhost:7331`. Rebuild the binary, restart the daemon, swap hook files — none affects the others as long as the URL is stable.
5. **User config vs. repo config are physically separate** (`~/.tr/` vs. `<repo>/.tr.yaml`). The loader (`internal/config/merge.go`) deep-merges with explicit rejection of `privacy.*`/`ai.*` from repo config. See [CONFIG.md](./CONFIG.md).

---

## Layer Coupling (Leak Points)

The cross-layer couplings that exist — each is instructive:

| Coupling | Direction | Mechanism | Implication |
|---|---|---|---|
| Binary → hooks (content) | At `tr repo` time only | Hook script bodies are embedded `const` strings in `internal/hooks/scripts.go`, compiled into the binary, written verbatim to `.git/hooks/` by `tr repo` | Rebuilding the binary does NOT update already-installed hooks. You must re-run `tr repo`. |
| Binary → post-commit hook (path) | At `tr repo` time only | `postCommitHookScript` in `main.go` is a static template that relies on `tr` being on PATH | If `tr` is not on PATH, the post-commit hook will fail. Re-run `tr repo` to refresh. |
| Hooks → daemon (URL) | At hook-fire time | `curl http://localhost:7331/hooks/...` — URL is a string in the hook script | Daemon must be running for dispatch to succeed. No daemon → advisory printed (the #14 surface). |
| post-commit hook → binary (ask) | At hook-fire time | Shells out to `tr ask` via PATH (no baked path) | The only hook that invokes the binary at fire time. Depends on `tr` being on PATH. A second, differently-worded advisory originates here via `ask.go:daemonUnavailableMessage`. |

---

## Worked Example: Binary Location Confusion

A common debugging mistake (and the one that drove the investigation surfacing this doc):

> "The binary with my active changes is on a worktree and I am using it from the main worktree to `init` and see advisory messages."

Mental model that produced the confusion: *the binary's location governs which hooks fire, or where they install, or which advisory surfaces.*

Reality: *the binary's location governs only one thing — **which compiled-in hook body constants get written** when `tr repo` runs. After `tr repo`, the binary is out of the loop for dispatch hooks entirely.*

So running `D:\...\worktree-A\tr.exe` from the main worktree:
- `tr repo` resolved CWD's git repo → main repo (`FindRepoRoot()` uses invocation CWD, not binary location)
- `tr repo` wrote the *new* hook bodies into the main repo's `.git/hooks/` — overwriting the prior ones
- Subsequent commits in the main worktree fired the *newly installed* hooks
- The dispatch hooks (pre-commit) still print *their* advisory when no daemon is reachable — that's correct behavior, not a bug

The binary being on a worktree was a red herring. Only two facts mattered: *which binary* ran (because of embedded constants) and *from which working directory* `init` was invoked (because of gitdir resolution).

---

## New-User Install Flow

```sh
# 1. Binary install (one-time, user-level)
go install github.com/colinwilliams91/total-recall@latest
#   → places `total-recall` on $GOPATH/bin (user must add $GOPATH/bin to PATH once)

# 2. User-level init (one-time, user-level)
tr init
#   → prompts: conversation analysis opt-in, AI provider, API key, model
#   → writes ~/.tr/config.yaml
#   → prints next-step guidance: "Next: cd into your project and run tr repo."

# 3. Start daemon (long-running terminal; keep alive)
tr serve
#   → binds localhost:7331
#   → reads ~/.tr/config.yaml + ~/.tr/memory.db

# 4. Per-repo init (run inside each repo you want recall in)
cd ~/projects/my-app
tr repo
#   → resolves .git/hooks via `git rev-parse --git-path hooks` (worktree-aware)
#   → writes .tr.yaml with hook enablement
#   → writes .git/hooks/{pre-commit,commit-msg,pre-push,post-commit} per selections

# 5. Verify
tr status
git commit -m "..."   # triggers installed hooks (NOT the binary)
```

For a user without Go: download the release archive from GitHub Releases, extract, place on PATH manually. Same downstream flow.

---

## Canonical Testing Simulation

Tests vary four factors independently: **binary** (which compiled version), **user config** (exists / pristine), **repo** (fresh / existing / worktree), **daemon** (running / not).

```powershell
# 1. Build the binary variant under test (location is irrelevant to behavior)
cd D:\repos\open-source\total-recall-05-opsx
.\scripts\rebuild.ps1

# 2. Fresh git repo OUTSIDE any existing repo
$scratch = "C:\tmp\tr-test-$(Get-Random)"
mkdir $scratch; cd $scratch
git init -q
git config user.email t@t
git config user.name t

# 3. Install hooks using the binary under test
& "D:\repos\open-source\total-recall-05-opsx\tr.exe" repo

# 4. (Optional) simulate brand-new user with isolated HOME
#    t.Setenv equivalent for manual testing

# 5. Test matrix:
#    a) daemon DOWN
"x" | Out-File a.txt; git add a.txt
git commit -m "test daemon-down"
# Expected: ONE pre-commit advisory + "Press any key" + ONE ask advisory (different wording)

#    b) daemon UP (separate terminal)
cd D:\repos\open-source\total-recall-05-opsx; .\tr.exe serve
# back in scratch:
"y" | Out-File b.txt; git add b.txt
git commit -m "test daemon-up"
# Expected: no advisory; recall question may surface via post-commit ask TUI
```

**Two non-negotiable simulation rules:**

1. **The scratch must be its own repo, not nested in any existing repo.** Nested `git init` creates an independent gitdir that does NOT inherit parent hooks.
2. **To verify changes to hook bodies, you must re-run `tr repo` against the rebuilt binary.** Source-code changes do not propagate to installed hook files; only `tr repo` does.

---

## Known leak points and edge cases

These are real follow-ups, not novel discoveries — each is a consequence of the layer model. The phase letter refers to the OpenSpec handoff plan; see your `/opsx-explore` proposal.

- **Worktree install** (resolved): `tr repo` from a linked worktree now works correctly — it resolves the hooks dir via `git rev-parse --git-path hooks`, which points to the common gitdir shared across all linked worktrees.
- **Stale post-commit after binary move** (resolved): post-commit hook now relies on `tr` being on PATH rather than capturing the binary path at install time. No more stale-path issue.
- **Binary version drift across repos** (architectural): hooks are static; if a user has 10 repos with `tr repo`'d hooks and upgrades the binary, only repos where they re-run `tr repo` get new hook bodies. No version handshake exists.
- **`tr init` and `tr repo` are separate commands** (resolved): user-config (`tr init`) and repo-config (`tr repo`) are now physically and logically separate. Re-running either command only re-prompts its own concerns.