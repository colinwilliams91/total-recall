## Context

Total Recall is a Go-binary CLI with module path `github.com/colinwilliams91/total-recall`. The binary is currently built from `cmd/total-recall/main.go` and the produced executable is named `total-recall` (or `total-recall.exe` on Windows). User-visible strings throughout the codebase embed `total-recall` as the typed command in advisory messages, hook script bodies, and documentation.

The user wants `tr` as the typed command while keeping `Total Recall` as the product name (Repo name, hook sentinel, stderr prefix labels, MCP server implementation name).

Five-layer model (per `DOCS/ARCHITECTURE/INSTALL_LAYERS.md`): this change touches **binary** (rename), **repo config** (no change to contents but advisory text only) and **git hooks** (the *content* of installed hook files changes because embedded CLI invocation strings change; the *sentinel* doesn't, so the installer can still detect and overwrite existing managed hooks).

No users exist. No backwards-compatibility shim is needed.

## Goals / Non-Goals

**Goals:**
- The compiled binary is named `tr` (`tr.exe` on Windows), invocable from `$GOPATH/bin` after `go install`.
- All user-visible CLI invocation strings inside Go source and hook script bodies say `tr`, not `total-recall`.
- Existing managed installations upgrade cleanly: the next `tr repo` (formerly `tr init` post-Y3, but per the Y2 timeline `tr init` still) overwrites installed hooks with the new strings automatically, with no manual cleanup required.
- All tests pass, all builds succeed, no dead code introduced.

**Non-Goals:**
- Renaming the Go module path (`github.com/colinwilliams91/total-recall`). Explicitly out of scope.
- Renaming `internal/...` package paths. Out of scope.
- Renaming the product brand "Total Recall" itself. The hook sentinel `# total-recall managed` and the `[total-recall]` stderr prefix on advisory messages stay — they are product identity labels, not typed commands. (Decision Issue 4/4-r.A.)
- Providing a `total-recall` → `tr` symlink or shim. No backwards-compat alias. (Anchoring constraint.)
- Any auto-PATH-modifying installer. The user adds `$GOPATH/bin` to PATH themselves per Go convention. (Anchoring constraint.)
- Fixing the post-commit hook's path-baking mechanism (`os.Executable()` capture). That is Y4's territory.

## Decisions

### Decision 1 — Source directory rename via `git mv`, not "new directory + copy"

`cmd/total-recall/` → `cmd/tr/` is a `git mv` of every file in the directory (main.go, wire.go, ask.go, 10 *_test.go files). Preserves git history per file. Avoids the alternative (new directory + copy + delete) which loses rename attribution.

**Alternatives considered:** none worth re-litigating; `git mv` is the obvious choice.

### Decision 2 — Binary name comes from the directory, not from an explicit `-o` flag

`go build ./cmd/tr` produces a binary named `tr` automatically because the directory's base name is `tr`. `make build` switches from `go build -o bin/total-recall ./cmd/total-recall` to `go build -o bin/tr ./cmd/tr` — the `-o` flag is preserved for explicit output to `bin/`, but the *binary name* anywhere else is implicit from the directory name.

**Alternatives considered:** keep directory `cmd/total-recall/` and just override with `-o bin/tr` — rejected because the prompt and Issue 4 make clear the source directory is renamed too (consistent namespace, `cmd/tr/` matches the binary name, future readers aren't surprised by `cmd/total-recall` producing a `tr` binary).

### Decision 3 — Hook sentinel is unchanged

`const sentinel = "# total-recall managed"` in `internal/hooks/scripts.go:4` stays. The corresponding literal `# total-recall managed` in `cmd/.../main.go:102`'s post-commit template also stays. The `:: total-recall managed` literals in the `.bat` hook templates (scripts.go:107, 138, 147) also stay.

**Rationale (Issue 4 decision):** the sentinel is a product-identity marker on disk, analogous to `[nginx]` or `[systemd]` labels. The product's brand is still "Total Recall"; only the typed CLI command is `tr`. Keeping the sentinel has a concrete mechanical benefit: the installer's `IsManagedHook` check (install.go:47-64) detects existing installs (from prior `total-recall init` runs) and overwrites them with the newly-`tr`-branded content on the next `tr init`/`tr repo`. The user never manually cleans up `.git/hooks/*`.

**Alternatives considered and rejected:**
- Rename sentinel to `# tr managed` — would require manual `.git/hooks/*` cleanup on upgrade because the installer could no longer detect existing managed hooks (Issue 4 Option B).
- Rename sentinel + add legacy detection for one upgrade cycle (Issue 4 Option C) — would add temporary code with a "delete me someday" marker. Rejected because (a) no users means no upgrade cycle to manage, (b) no dead code is allowed.

### Decision 4 — `[total-recall]` stderr prefix on advisory messages stays

Existing strings like `[total-recall] Daemon not running. Start with total-recall serve.` and `[total-recall] curl not found — install curl to enable dispatch.` keep their `[total-recall]` prefix unchanged. The *CLI invocation* portion of those strings (`Start with total-recall serve`) is renamed to `Start with 'tr serve'`.

**Rationale (Issue 4-r.A decision):** `[total-recall]` is a product label, not a command. Consistent with the Decision 3 principle.

### Decision 5 — CLI invocation strings inside hook bodies are renamed

Strings like `Re-run 'total-recall init' to update.` and `Generated by 'total-recall init'.` become `Re-run 'tr init' to update.` and `Generated by 'tr init'.` etc. These appear in:
- `internal/hooks/scripts.go` — `hookHeader` (line 10), `preCommitBody` (line 31, 57 — stderror advisories include `total-recall serve` invocations), `prePushBody` (line 98), `preCommitBat` (line 109, 128), `commitMsgBat` (line 140), `prePushBat` (line 149, 157). About 8 string edits.
- `cmd/total-recall/main.go` — `postCommitHookScriptTmpl` (line 104: `# Generated by 'total-recall init'`), daemon advisory at line 64 (`Run 'total-recall init' to configure`), post-commit install printout at line 250 (`Start the daemon with: total-recall serve`). About 3 string edits.
- `cmd/total-recall/ask.go` — `daemonUnavailableMessage` (line 44), `stateFeedback` advisory (line 260). About 2 string edits.

**Rationale:** these are typed commands the user is told to run. They must match the binary name. A mixed `total-recall init`/`tr init` landscape is the failure mode Y2 prevents.

### Decision 6 — The `mcp.NewServer` `Name: "total-recall"` field stays

`internal/engine/mcp.go:41` calls `mcp.NewServer(&mcp.Implementation{Name: "total-recall", Version: "v1"}, ...)`. This is the MCP server's product identifier surfaced to MCP clients — it's the product name, not the typed CLI command. Stays unchanged.

### Decision 7 — No tests are added in Y2

Y2 is a rename. Existing tests should continue to pass after the rename. If `main_test.go`'s `buildPostCommitHookScript` tests currently assert on the literal string `total-recall` in the rendered hook, those assertions must be updated as part of the same rename — they're consistency edits, not new tests. No new tests are added in Y2.

**Recommendation:** During implementation, run `go test ./...` to detect any failing string assertion in `main_test.go`. Update those assertions to their post-rename expectations as part of the rename commit. Don't add new tests for strings — they don't test behavior.

## Risks / Trade-offs

- **Risk:** A `total-recall` reference is missed somewhere in the rename, leaving a mixed-brand string landscape. → **Mitigation:** `grep -r 'total-recall' --exclude-dir=openspec --exclude-dir=.git --include='*.go' --include='*.sh' --include='*.bat' --include='*.ps1' --include='*.yaml' --include='*.md' --include='Makefile'` after the rename. The only `total-recall` references that survive should be: the `sentinel` const and its `.bat` mirror literals; `[total-recall]` prefix substrings inside hook scripts and ask.go advisories; `mcp.Implementation{Name: "total-recall"}`; the module path `github.com/colinwilliams91/total-recall` in `go.mod`/`go.sum`/`.goreleaser.yaml`; `INSTALL_LAYERS.md` references to the product name by brand; and the `repoRoot` directory name `total-recall-06-opsx` itself. A grep audit catches anything else.
- **Risk:** User has a previously installed `total-recall` binary on `$GOPATH/bin` that lingers after `go install ./cmd/tr` puts a new `tr` binary alongside it. → **Mitigation:** Document in Y5 (install docs rewrite) that prior `total-recall` binaries can be removed with `rm $(go env GOPATH)/bin/total-recall`. Not a Y2 concern.
- **Trade-off:** Keeping the sentinel as `# total-recall managed` means a user reading `.git/hooks/pre-commit` sees `# total-recall managed` at the top while invoking the CLI as `tr`. This is intentional and aligned with the brand-stays / typed-command-changes decision (Issue 4 and 4-r.A).
- **Risk:** repo-root directory is `total-recall-06-opsx` and `scripts/rebuild.ps1` may reference the absolute path of the binary in the build output; renaming the source dir but not the build script path would break the quick-rebuild command. → **Mitigation:** `scripts/rebuild.ps1` line 8 update is in the explicit task list.

## Migration Plan

This is a self-contained code change with no runtime migration.

1. Implement Y2 (the rename) per tasks.md.
2. `make build` (or `.\scripts\rebuild.ps1` on Windows) produces `tr.exe` in the build output directory.
3. Existing daemon instances keep running (the daemon URL `http://localhost:7331` is unchanged; renaming the binary doesn't affect a running daemon). Restart the daemon with the new binary: stop, run `tr serve` (renamed command).
4. Existing installed hooks auto-refresh on the next `tr init` run from inside any repo (installer detects the managed sentinel and overwrites with newly-`tr`-branded content).

**Rollback:** revert the commit. Since there are no users, no coordination is needed.

## Open Questions

None outstanding. All decisions for Issue 4 (sentinel) and Issue 4-r.A (stderr prefix) were resolved during explore.