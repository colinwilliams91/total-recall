# Follow-up: Post-commit hook PATH collision

**Status:** Resolved (2026-07-28). Option A adopted — reverted to `os.Executable()`-based path baking.

## Problem

After folding Y4 into Y3, the post-commit hook uses `exec tr ask` (relying on `tr` being on PATH). On systems where the Unix `tr` utility (`/usr/bin/tr`) is on PATH before the Total Recall binary, the hook invokes the wrong program.

Observed in this repo's post-commit hook output:
```
/usr/bin/tr: missing operand after 'ask'
Two strings must be given when translating.
```

The Unix `tr` expects two arguments (translate from → to), gets just `ask`, and errors. The hook doesn't surface a recall question.

## Root cause

The Y4 fold-in traded the stale-binary-path problem for a PATH-collision problem. `os.Executable()`-based path baking was reliable cross-platform; PATH-based lookup depends on system PATH ordering.

The collision was worse on Windows than initially described: Git for Windows runs hooks via MSYS `sh.exe` with a **restricted PATH** that includes `/usr/bin` (where the Unix `tr` lives) but does NOT include the user's Windows PATH additions like `$GOPATH/bin`. Coalescing the install location to `$GOBIN` — while good hygiene — could not fix the collision: the hook shell simply doesn't see that PATH entry.

## Resolution

**Option A adopted:** revert to `os.Executable()`-based path baking.

The post-commit hook template (`postCommitHookScriptTmpl` in `cmd/tr/main.go`) now has two `%s` placeholders filled by `buildPostCommitHookScript(os.Executable())` at `tr repo` time:
- PowerShell branch: native backslash path, single-quoted (`& 'C:\...\tr.exe' ask`)
- sh fallback: forward-slash path, double-quoted (`exec "C:/.../tr.exe" ask`) so MSYS sh can exec the `.exe` directly

The stale-path trade-off (moving/rebuilding the binary requires re-running `tr repo`) was deemed preferable to the PATH-collision failure mode because:
- Stale-path error is loud (`No such file or directory` pointing at the dead path) and only triggers on binary relocation
- Collision is silent, occurs on every commit, with a confusing unrelated error message

### Companion changes

- `scripts/rebuild.ps1` coalesced from `go build -o tr.exe ./cmd/tr` to `go install ./cmd/tr` so contributors and end users share the same `$GOBIN/tr.exe` install path, eliminating a whole class of stale-hook triggers for contributors who switch between repo-local builds and `go install`.
- `cmd/tr/pathdetect.go` warning reason string updated — the post-commit hook no longer relies on PATH resolution at fire time; PATH is still needed for the user to run `tr serve`, `tr repo`, `tr ask` from any terminal.
- `DOCS/ARCHITECTURE/INSTALL_LAYERS.md` leak-point table, testing simulation, and known-edge-cases section updated to reflect the baked-path model and collision resolution.
- `openspec/specs/post-commit-hook/spec.md` and `openspec/specs/path-detection-warning/spec.md` updated to match implementation.

### Additional bugs fixed during verification

- `internal/pipeline/extraction.go`: AI returning a single JSON object instead of an array now falls back to single-object unmarshal instead of failing silently.
- `internal/ai/openai/client.go` and `internal/ai/anthropic/client.go`: HTTP client timeout 10s → 60s (was too aggressive for AI inference).
- `internal/engine/server.go`: pipeline context timeout 30s → 90s (needs to cover two sequential AI calls: extraction + synthesis).
- `cmd/tr/ask.go`: `tr ask` default poll timeout 15s → 60s (was giving up before the pipeline could produce a question).

## Why it's not blocking

- Most developer systems (Windows with Go on PATH, macOS with Homebrew, Linux without Unix `tr` on PATH) don't hit this
- The hook still fires — it just errors instead of surfacing a recall question
- No data loss or corruption

## Options considered

Option A: Revert to `os.Executable()`-based path baking. Stale-path problem is recoverable via `tr repo`. **← Chosen**

Option B: Hook searches a known list of candidate locations (e.g., `$GOPATH/bin/tr`, `/usr/local/bin/tr`, `~/.tr/bin/tr`) and falls back. More portable but more complex. Rejected: existence checks don't prove the candidate is Total Recall (`/usr/bin/tr` is executable too), and the last-resort fallback re-introduces the collision precisely when the user has moved their binary.

Option C: Hook checks if the resolved `tr` is Total Recall by looking for a sentinel flag or version string. Most robust but requires CLI changes. Deferred: adds CLI surface area and double-invocation overhead; can be revisited if stale-path becomes a real friction point.

## Reproduction

```sh
# On a system with /usr/bin/tr on PATH:
git commit -m "test"  # in any repo with tr-managed hooks
# Expected: recall question surfaces
# Actual: /usr/bin/tr: missing operand after 'ask'
```

## Workaround until fixed

~~Run `tr ask` manually after commits, or invoke the full path to the Total Recall binary (e.g., `/path/to/tr.exe ask` or `& 'C:\path\to\tr.exe' ask`).~~

Resolved — re-run `tr repo` to install the new baked-path hook template.
