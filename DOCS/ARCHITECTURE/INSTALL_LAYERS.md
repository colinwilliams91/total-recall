# Install & Layer Architecture

Total Recall is a single Go binary distributed via `go install` (or release archives). Its runtime state is split across five independently-managed layers. Understanding these layers — and where they do and do not couple — is essential for installation, testing, worktree support, and reasoning about hook behavior.

---

## The Five Layers

| Layer | Owned by | Lifetime | Reads from | Writes to |
|---|---|---|---|---|
| **Binary** (`total-recall` / `tr.exe`) | One per user; on `$GOPATH/bin` or invoked by absolute path | Until re-install | Nothing at runtime (stateless) | Invoked for `init`, `serve`, `ask`, `config` |
| **User config** (`~/.tr/config.yaml`) | One per user | Until deleted | Runtime config loader | Written by `tr init`; never by hooks |
| **User cache** (`~/.tr/memory.db`) | One per user | Until deleted | Daemon (read concepts); write concepts/answers | Daemon only |
| **Repo config** (`.tr.yaml`) | One per repo | Until deleted | Runtime config loader | Written by `tr init` |
| **Git hooks** (`.git/hooks/*`) | One per gitdir | Until overwritten | Fired by Git | Written by `tr init` |
| **Daemon** (`total-recall serve` on `:7331`) | One per machine | Process lifetime | User cache + AI provider | User cache |

The binary is stateless: invoking it from any path executes the same logic against the same user-level state. Its compile-time-baked constants (hook bodies, daemon URL) are emitted into the installable artifacts at `tr init` time, then the binary is out of the loop until the next `init`.

---

## Crucial Separations

These independence properties are what make the system tractable, and violating them silently is the source of most "why didn't my change take effect?" surprises.

1. **Binary location is irrelevant to which hooks fire.** Hooks fire because Git finds executable files in `<gitdir>/hooks/`. Delete the binary after `init` and the hooks still fire on every commit (they fail politely, but they fire).
2. **Binary location is irrelevant to where `tr init` installs hooks.** `init` resolves the gitdir from the *current working directory* (`hooks.FindRepoRoot()`). The binary could be on a network drive; running it from worktree A installs into A's gitdir.
3. **Hooks are static files, fully decoupled from the binary after install.** Once `init` writes `.git/hooks/pre-commit`, that file is just bash. It does not invoke or read the binary. It only knows `http://localhost:7331` (a string baked into the hook body at *compile* time and emitted into the script at *init* time).
4. **Daemon is fully decoupled from binary and hooks.** Hooks are HTTP clients; daemon is an HTTP server. They share only the URL `http://localhost:7331`. Rebuild the binary, restart the daemon, swap hook files — none affects the others as long as the URL is stable.
5. **User config vs. repo config are physically separate** (`~/.tr/` vs. `<repo>/.tr.yaml`). The loader (`internal/config/merge.go`) deep-merges with explicit rejection of `privacy.*`/`ai.*` from repo config. See [CONFIG.md](./CONFIG.md).

---

## Layer Coupling (Leak Points)

The cross-layer couplings that exist — each is instructive:

| Coupling | Direction | Mechanism | Implication |
|---|---|---|---|
| Binary → hooks (content) | At `init` time only | Hook script bodies are embedded `const` strings in `internal/hooks/scripts.go`, compiled into the binary, written verbatim to `.git/hooks/` by `init` | Rebuilding the binary does NOT update already-installed hooks. You must re-run `init`. |
| Binary → post-commit hook (path) | At `init` time only | `postCommitHookScriptTmpl` in `main.go` captures `os.Executable()` and embeds that absolute path | If you move/rebuild the binary to a different path, the installed post-commit hook still invokes the OLD path. Re-run `init` to refresh. |
| Hooks → daemon (URL) | At hook-fire time | `curl http://localhost:7331/hooks/...` — URL is a string in the hook script | Daemon must be running for dispatch to succeed. No daemon → advisory printed (the #14 surface). |
| post-commit hook → binary (ask) | At hook-fire time | Shells out to `tr ask` via the captured binary path | The only hook that invokes the binary at fire time, and the only hook coupled to binary path. A second, differently-worded advisory originates here via `ask.go:daemonUnavailableMessage`. |

---

## Worked Example: Binary Location Confusion

A common debugging mistake (and the one that drove the investigation surfacing this doc):

> "The binary with my active changes is on a worktree and I am using it from the main worktree to `init` and see advisory messages."

Mental model that produced the confusion: *the binary's location governs which hooks fire, or where they install, or which advisory surfaces.*

Reality: *the binary's location governs only one thing — **which compiled-in hook body constants get written** when `init` runs. After `init`, the binary is out of the loop for dispatch hooks entirely.*

So running `D:\...\worktree-A\tr.exe` from the main worktree:
- `init` resolved CWD's git repo → main repo (`FindRepoRoot()` uses invocation CWD, not binary location)
- `init` wrote the *new* hook bodies into the main repo's `.git/hooks/` — overwriting the prior ones
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
total-recall init
#   → prompts: conversation analysis opt-in, AI provider, API key, model
#   → writes ~/.tr/config.yaml
#   → detects "not in a git repo" → skips hook install with a warning

# 3. Start daemon (long-running terminal; keep alive)
total-recall serve
#   → binds localhost:7331
#   → reads ~/.tr/config.yaml + ~/.tr/memory.db

# 4. Per-repo init (run inside each repo you want recall in)
cd ~/projects/my-app
total-recall init
#   → re-prompts user-level questions (see "Lifecycle split" known issue below)
#   → resolves .git/hooks via CWD
#   → writes .tr.yaml with hook enablement
#   → writes .git/hooks/{pre-commit,commit-msg,pre-push,post-commit} per selections
#   → bakes binary path (os.Executable()) into post-commit only

# 5. Verify
total-recall status
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
& "D:\repos\open-source\total-recall-05-opsx\tr.exe" init

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
2. **To verify changes to hook bodies, you must re-run `tr init` against the rebuilt binary.** Source-code changes do not propagate to installed hook files; only `init` does.

---

## Known leak points and edge cases

These are real follow-ups, not novel discoveries — each is a consequence of the layer model. The phase letter refers to the OpenSpec handoff plan; see your `/opsx-explore` proposal.

- **Worktree install** (Y-adjacent): `tr init` from a linked worktree fails because `<worktree>/.git` is a file, not a directory. Resolution: use `git rev-parse --git-path hooks`. Git resolves hooks via the *common gitdir* (`<main>/.git/hooks/`), shared across all linked worktrees — empirically confirmed.
- **Stale post-commit after binary move** (minor): post-commit hook captures binary path at `init`; moving/rebuilding the binary elsewhere leaves post-commit invoking the old path. Fix: re-run `init`, or template in `total-recall` and rely on PATH at fire time.
- **Binary version drift across repos** (architectural): hooks are static; if a user has 10 repos with `init`'d hooks and upgrades the binary, only repos where they re-run `init` get new hook bodies. No version handshake exists.
- **`tr init` re-prompts user-level questions** (lifecycle): user-config and repo-config are physically separate but the init command conflates them into one atomic prompt. See "Lifecycle split" follow-up.