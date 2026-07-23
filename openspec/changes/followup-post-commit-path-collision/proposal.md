# Follow-up: Post-commit hook PATH collision

**Status:** Open follow-up from `init-repo-split` (Y3)

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

## Why it's not blocking

- Most developer systems (Windows with Go on PATH, macOS with Homebrew, Linux without Unix `tr` on PATH) don't hit this
- The hook still fires — it just errors instead of surfacing a recall question
- No data loss or corruption

## Suggested fix (when addressed)

Option A: Revert to `os.Executable()`-based path baking. Stale-path problem is recoverable via `tr repo`.

Option B: Hook searches a known list of candidate locations (e.g., `$GOPATH/bin/tr`, `/usr/local/bin/tr`, `~/.tr/bin/tr`) and falls back. More portable but more complex.

Option C: Hook checks if the resolved `tr` is Total Recall by looking for a sentinel flag or version string. Most robust but requires CLI changes.

## Reproduction

```sh
# On a system with /usr/bin/tr on PATH:
git commit -m "test"  # in any repo with tr-managed hooks
# Expected: recall question surfaces
# Actual: /usr/bin/tr: missing operand after 'ask'
```

## Workaround until fixed

Run `tr ask` manually after commits, or invoke the full path to the Total Recall binary (e.g., `/path/to/tr.exe ask` or `& 'C:\path\to\tr.exe' ask`).
