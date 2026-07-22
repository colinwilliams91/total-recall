## MODIFIED Requirements

### Requirement: tr init generates a post-commit hook
`tr init` SHALL generate `.git/hooks/post-commit` containing a shell script that calls `tr ask`. The file SHALL be made executable (`chmod 0755`). The generation SHALL be idempotent — running `tr init` again SHALL overwrite the hook cleanly.

#### Scenario: Fresh init
- **WHEN** `tr init` is run in a repo with no post-commit hook
- **THEN** `.git/hooks/post-commit` is created, is executable, and contains `exec "$(which tr)" ask`

#### Scenario: Re-run init
- **WHEN** `tr init` is run again in a repo that already has a post-commit hook
- **THEN** the hook is overwritten with the current template content; no error is raised

---

### Requirement: Post-commit hook calls tr ask
The generated post-commit hook SHALL call `tr ask` via `exec "$(which tr)" ask`. Using `exec` replaces the shell process, keeping process count minimal. Using `$(which tr)` avoids hardcoding the binary path.