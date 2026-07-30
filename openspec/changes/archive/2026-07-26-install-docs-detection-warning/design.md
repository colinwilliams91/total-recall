## Context

Y5 is the smallest-yet phase in the Y-batch: a README rewrite + a new inline startup-detection routine in `runInit()` (and optionally `runRepo()`). It depends on Y2 (binary is named `tr`), Y3 (the init/repo split means `tr init` is the best place to detect "is `tr` on PATH"), and Y4 (Y4's post-commit hook relies entirely on PATH — Y5's detection is the safety net that catches "user forgot to put `tr` on PATH" before they ever install the hook).

Relevant explore-phase decisions:

- **Issue 5 Y4 hook PATH strategy:** chosen 5.B (rely on PATH). The Y4 design notes Y5's `tr init` startup detection as the safety net that makes 5.B robust. Y5 implements that safety net.
- **Issue 6 hooks-dir resolution:** Y3 introduced `git rev-parse --git-path hooks` as the install path resolver. Y5 documents the Git 2.5+ (2015) prerequisite in the README.
- **Anchoring constraints (parent plan):** "NO binary self-installer that auto-modifies the user's PATH. Go convention: user adds $GOPATH/bin to PATH themselves, once. We detect + warn if `tr` is not on PATH; we never write to shell rc files."

Existing layer model: Y5 touches the **binary** layer (new startup detection) only. The README is documentation, not a layer.

The existing `init-ai-setup` spec (modified by Y3) covers `tr init`'s AI-provider form. Y5 adds a new capability `path-detection-warning` (rather than restructuring `init-ai-setup` further) to keep the capability clean — specifying *how `tr init`* invokes the detection is a path-detection concern, not an AI-provider concern.

No users exist. The README rewrite nonetheless aims for clarity for future users.

## Goals / Non-Goals

**Goals:**
- `tr init` (and optionally `tr repo`) perform a single PATH-detection call at startup, printing a shell-specific one-line PATH-fix command when `tr` is not found.
- The README Setup section reflects the post-Y3 install flow with `tr init` and `tr repo` as distinct commands, includes the `go install ...` line with the unchanged module path, includes a non-Go-install path (release archives), and mentions the Git 2.5+ prerequisite.
- The detection never writes to rc files; it only outputs to stderr/stdout.
- Tests cover both Unix (`command -v tr`) and Windows (`Get-Command tr`) detection paths using platform-gated test functions.

**Non-Goals:**
- No "tr install doctor" / "tr install check" subcommand exists as a separate command. The detection runs inline in `tr init` (and optionally `tr repo`).
- No auto-rc-mutation flow, even behind a flag.
- No detection of "old `total-recall` binary on `$GOPATH/bin` from a pre-Y2 install". The user manually deletes it; Y5 mentions this once in the README.
- No detection of "the daemon is not running" — `tr status` already handles that. Y5 doesn't duplicate.
- No version-handshake between binary and installed hooks — same deferred limitation as before.
- No change to the Go module path.
- No editor integrations, IDE integrations, or MCP-client-specific install docs.

## Decisions

### Decision 1 — Run detection before any prompts, only print if missing

`runInit()`'s first action (post-Y3) is `checkTrOnPath()`. The function:

1. Detects the shell via `os.Getenv("SHELL")` on Unix (non-empty → likely bash/zsh; empty → use the OS-default).
2. Detects Windows via `runtime.GOOS == "windows"`.
3. On Unix: runs `exec.Command("sh", "-c", "command -v tr")` and inspects whether the output is non-empty (success exit + non-empty result means `tr` is on PATH).
4. On Windows: runs `exec.Command("powershell.exe", "-NoProfile", "-Command", "(Get-Command tr -ErrorAction SilentlyContinue).Name")` and inspects whether the output is non-empty.
5. If found, returns silently.
6. If not found, prints a warning to stderr with the shell-specific PATH-fix command and returns. The flow continues.

**Rationale:** the detection runs once per command invocation, costs negligible time, never blocks. Printing only when missing means users on correctly-configured systems never see noise.

**Warning text examples (concrete):**
- Bash: `⚠  tr not found on PATH. Add $GOPATH/bin to PATH with: echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc`
- Zsh: `⚠  tr not found on PATH. Add $GOPATH/bin to PATH with: echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc`
- PowerShell: `⚠  tr not found on PATH. Add $GOPATH/bin to PATH with: Add-Content $PROFILE 'set PATH="$PATH;$(go env GOPATH)/bin"'` (or the precise PowerShell form the implementer chooses — keep it copy-pasteable).

### Decision 2 — Detection never writes to rc files

The function's sole output is the warning text to stderr. The user copies and runs the suggested command themselves.

**Rationale (anchoring constraint):** "We detect + warn if `tr` is not on PATH; we never write to shell rc files." This is a hard constraint from the parent plan. The Y5 design respects it.

**Alternative considered and rejected:** auto-write to the detected shell's rc file with user permission (a `y/N` prompt). Rejected because it's simply more code, more risk (writing to wrong file, corrupting it), and violates the explicit anchoring constraint.

### Decision 3 — Detection at `tr repo` startup: optional, implementer's call

The Y5 prompt explicitly said "At tr init startup: detect if `tr` is reachable via `command -v tr` (Unix) / `Get-Command tr` (Windows). If not, print a warning..." The detection at `tr init` is mandatory. The Y5 proposal mentions "Optional subtask: also run the PATH detection at `tr repo` startup" and during explore this is left to the implementer's judgment.

Rationale for making `tr repo` detection optional:
- The documented install flow includes `tr init` before `tr repo`, so the warning fires first at `tr init`.
- A user who skips `tr init` AND runs `tr repo` first by absolute path is contrived; the cost-to-benefit tradeoff (more code, more test scope) is marginal.

Rationale for including it anyway:
- It closes the contrived-but-possible gap.
- Cost is small — the same `checkTrOnPath()` function is invoked from `runRepo()`'s top, identical behavior.

**Decision:** the implementer's task list (tasks.md) includes BOTH the `tr init` invocation (mandatory) AND the `tr repo` invocation (optional — defer if estimated effort exceeds 15 minutes). Track as a checkbox the implementer can mark skippable.

### Decision 4 — README structure mirrors the actual install flow

The Setup section is reorganized to match the post-Y3 flow:
1. **Install the binary** — `go install github.com/colinwilliams91/total-recall@latest` (or `gh release download` equivalent — note module path stays `total-recall`, only the binary name is `tr`).
2. **One-time: add `$GOPATH/bin` to PATH** — add the shell-specific one-liner the user pastes from Y5's warning.
3. **`tr init`** — user-level config (AI provider, API key, model, conversation analysis).
4. **`tr serve`** — long-running daemon.
5. **`tr repo`** — per-repo setup, run from inside each repo.
6. **`git commit -m "..."`** — smoke test (verifies hooks fire).

Plus a one-sentence prerequisite near the top: "Requires Git 2.5+ (2015) for linked worktree support."

**Rationale:** the steps match the user's actual chronological order. Previous Setup sections had the binary install separate from the init steps; the post-Y3 + Y4 reality is that the binary install + PATH setup happens before `tr init` is ever useful — surfacing the PATH step explicitly in the README *and* in the warning text means users see the same instruction in both places.

### Decision 5 — README preserves the binary-rename note for legacy users

Y2 + Y5 do not provide a `total-recall` backup alias, so any prior `total-recall` binary the user has installed needs manual cleanup. Y5's README adds a one-sentence note near the Setup section:

> Existing installs from before the rename: remove the prior `total-recall` binary with `rm $(go env GOPATH)/bin/total-recall` (or the equivalent on Windows). Future invocations use the `tr` command only.

**Rationale:** single-maintainer + no real-userbase, but the documentation should not skip this; it costs one sentence.

## Risks / Trade-offs

- **Risk:** `command -v tr` returns a non-zero exit if the test shell (`sh`) is not available (rare on Unix; never on standard distros). The implementer should prefer `exec.LookPath("tr")` (stdlib, no shell dependency) for the Unix path. → **Mitigation:** use `exec.LookPath` instead of `exec.Command("sh", "-c", "command -v tr")`. `exec.LookPath` uses PATH directly without spawning `sh`. Update the spec/task accordingly when implementing. (This is a refinement of Decision 1; the spirit is the same.)
- **Risk:** `Get-Command tr` on Windows is invoked via `powershell.exe`, but a future Windows might behave differently (e.g., pwsh 7+ strictly enforces `-NoProfile` differently). → **Mitigation:** Y5 uses `-NoProfile -ExecutionPolicy Bypass` to minimize invocation variance; tests cover the detection at the function level, not the cross-process level.
- **Risk:** The shell-detection heuristics (`$SHELL` for Unix, `runtime.GOOS` for Windows) don't catch certain valid configurations (e.g. Nushell, fish, customized setups). The user sees the warning and possibly the wrong one-liner. → **Mitigation:** the one-liner includes the underlying `export PATH=...` or `Add-Content $PROFILE ...` form plausibly adaptable to any shell. A Nushell user can see the bash/zsh guidance and infer `path add` from the structure. Documented limitation in the README.
- **Risk:** README rewrite accidentally includes a `total-recall <command>` reference that Y2 missed. → **Mitigation:** Y5 tasks include a `grep 'total-recall ' README.md` audit step that flags any remaining CLI invocation references.
- **Trade-off:** the optional `tr repo` startup detection adds runtime cost (one `exec.LookPath` call) to every `tr repo` invocation. Microseconds — not worth measuring.

## Migration Plan

This is a code+docs change.

1. Implement Y5 per tasks.md.
2. Build the new binary; verify `tr init` startup prints the warning when run from a shell where `tr` is not on PATH. On Unix, simulate this with `PATH=/usr/local/bin:/usr/bin:/bin tr init` (or similar — empty the directory containing `tr`).
3. README verification: walk through the Setup section line-by-line and confirm a fresh-user-reading-the-README experience is correct: every command is `tr`, every module path is `github.com/colinwilliams91/total-recall`, every shell-specific instruction is in the right syntax for that shell.
4. No data migration (no schema, no config file format changes).

**Rollback:** revert the commit. The README is back to the post-Y2 state; the `tr init` startup detection is gone. Existing `tr init` runs without warning (the user loses the safety net but captures no behavior regression).

## Open Questions

- **Detection at `tr repo`: implementer's call.** The task list includes the `tr repo` invocation with an explicit deferrable marker; no spec-level requirement is added for it (the spec-level requirement is only on `tr init`).
- **Should Y5 also surface the legacy-binary-cleanup note in a `tr init` startup message** (not just the README)? Out of scope; the README mention suffices.
- **Cross-shell detection** (fish, nushell, etc.) is a known limitation. A future phase could add more detection patterns; Y5 covers only the three most common (bash, zsh, powershell).