## MODIFIED Requirements

### Requirement: Post-commit hook calls tr ask via PATH
The generated post-commit hook SHALL call `tr ask` directly via `exec tr ask` (a literal string, no `$(which ...)` indirection, no baked absolute path). The hook relies on the shell's PATH resolution at hook-fire time to find `tr`. Using `exec` replaces the shell process, keeping process count minimal.

This decision replaces the prior `exec "$(which tr)" ask` form (introduced in the Y2 rename-binary-to-tr change). The `$(which tr)` indirection was removed in Y4 in favor of direct `exec tr ask` because: (a) `$(which tr)` returns an empty string when `tr` is not on PATH, causing `exec "" ask` to fail with a confusing error; direct `exec tr ask` instead fails with the clearer `tr: not found` message; (b) the Y5 `tr init` startup PATH-detection catches "tr not on PATH" before any `tr repo` install attempt that would write a faulty hook, eliminating the need for a `$(which ...)` defensive indirection in the hook itself.

#### Scenario: tr is on PATH and the post-commit hook fires
- **WHEN** Git fires the post-commit hook after a successful commit and `tr` is on the user's PATH
- **THEN** `exec tr ask` resolves `tr` via PATH and the recall-question TUI runs in the same terminal session as the git commit

#### Scenario: tr is not on PATH and the post-commit hook fires
- **WHEN** Git fires the post-commit hook and `tr` is NOT on the user's PATH (the user ignored Y5's `tr init` startup warning)
- **THEN** `exec tr ask` fails with `tr: not found` (or platform-equivalent) in the terminal during the commit; the commit itself is unaffected (post-commit hooks run after the commit is created)

#### Scenario: post-commit hook fires after a binary move (the original use case)
- **WHEN** the user rebuilds `tr` via `go install ./cmd/tr` so a new binary lands at `$GOPATH/bin/tr` (different from the binary location captured at install time under the OLD absolute-path scheme)
- **AND** the post-commit hook was installed BEFORE the rebuild (with the post-Y4 static template — no baked path)
- **THEN** the post-commit hook continues to work seamlessly — `exec tr ask` resolves via PATH to the new binary at the new path; no `tr repo` re-run is required