## Requirements

### Requirement: tr init generates a post-commit hook
`tr init` SHALL generate `.git/hooks/post-commit` containing a shell script that calls `tr ask`. The file SHALL be made executable (`chmod 0755`). The generation SHALL be idempotent — running `tr init` again SHALL overwrite the hook cleanly.

#### Scenario: Fresh init
- **WHEN** `tr init` is run in a repo with no post-commit hook
- **THEN** `.git/hooks/post-commit` is created, is executable, and contains `exec "$(which total-recall)" ask`

#### Scenario: Re-run init
- **WHEN** `tr init` is run again in a repo that already has a post-commit hook
- **THEN** the hook is overwritten with the current template content; no error is raised

---

### Requirement: Post-commit hook calls tr ask
The generated post-commit hook SHALL call `total-recall ask` via `exec "$(which total-recall)" ask`. Using `exec` replaces the shell process, keeping process count minimal. Using `$(which total-recall)` avoids hardcoding the binary path.

---

### Requirement: Post-commit hook is installed after existing hooks
The post-commit hook installation step SHALL run after the existing pre-commit, commit-msg, and pre-push hook installations in `runInit()`.

---

### Requirement: tr init TUI informs the user about tr ask
The `tr init` TUI flow SHALL display a note or confirmation before or after installing the post-commit hook, explaining that the hook will surface a recall question in the terminal after each successful commit.

#### Scenario: User sees the hook note
- **WHEN** the user completes the `tr init` TUI flow
- **THEN** they have been informed that a post-commit hook was installed and will surface recall questions
