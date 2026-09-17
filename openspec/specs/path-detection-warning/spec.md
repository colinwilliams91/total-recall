# path-detection-warning Specification

## Purpose
TBD - created by archiving change install-docs-detection-warning. Update Purpose after archive.
## Requirements


### Requirement: tr init checks whether tr is on PATH before prompting
`torec init` SHALL, as its first action before any user-facing prompts are shown, detect whether `torec` is reachable via `exec.LookPath("torec")` on Unix (or `Get-Command torec` on Windows PowerShell). The detection MUST NOT modify any shell rc file (`~/.bashrc`, `~/.zshrc`, `$PROFILE`, etc.). When `torec` is found on PATH, `torec init` SHALL proceed silently with no PATH-related output.

#### Scenario: tr is on PATH
- **WHEN** `torec init` is run on a system where `torec` is reachable via PATH
- **THEN** `torec init` proceeds with no PATH-detection warning; the first user-facing output is the conversation-analysis opt-in prompt

#### Scenario: tr is not on PATH
- **WHEN** `torec init` is run on a system where `torec` is NOT reachable via PATH
- **THEN** `torec init` prints a warning to stderr; the warning includes a shell-specific one-line command for the user to paste to add `$GOPATH/bin` to their PATH; `torec init` then continues with the normal flow (conversation-analysis opt-in, AI provider, etc.)


### Requirement: Warning message is shell-specific and copy-pasteable
When `torec` is not on PATH, the warning text printed to stderr SHALL include the exact one-line command for the detected shell. The detection SHALL support at minimum:

- **Bash** (`$SHELL` contains `bash`, or `$SHELL` is empty on Linux): `echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc` (or platform-equivalent)
- **Zsh** (`$SHELL` contains `zsh`): `echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc` (or platform-equivalent)
- **PowerShell** (`runtime.GOOS == "windows"`): `Add-Content $PROFILE 'set PATH="$PATH;$(go env GOPATH)/bin"'` or equivalent copy-pasteable PowerShell syntax

Shells NOT in the supported list (fish, nushell, etc.) SHALL still receive a warning; the warning MAY fall back to the bash/zsh form with a note that the user may need to adapt the syntax.

The warning format SHALL be: `⚠  torec not found on PATH. Add $GOPATH/bin to PATH with: <one-line-command>` followed by a brief reason ("torec needs to be on PATH so you can run torec serve, torec repo, and torec ask from any terminal.").

#### Scenario: Bash user without tr on PATH
- **WHEN** `torec init` is run on Linux with `$SHELL=/bin/bash` and `torec` not on PATH
- **THEN** stderr receives a line containing `⚠  torec not found on PATH. Add $GOPATH/bin to PATH with: echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc` (exact wording may vary but the command form MUST be copy-pasteable into bash)

#### Scenario: Zsh user without tr on PATH
- **WHEN** `torec init` is run on macOS with `$SHELL=/bin/zsh` and `torec` not on PATH
- **THEN** stderr receives a line containing the zsh-specific PATH-fix one-liner with `>> ~/.zshrc` as the target file

#### Scenario: PowerShell user without tr on PATH
- **WHEN** `torec init` is run on Windows (`runtime.GOOS == "windows"`) and `torec` not on PATH
- **THEN** stderr receives a line containing the PowerShell-specific PATH-fix one-liner targeting `$PROFILE`


### Requirement: Detection never modifies rc files
The PATH-detection function SHALL only print warning text to stderr. It MUST NOT use `os.OpenFile` or any other write API against shell rc files (`~/.bashrc`, `~/.zshrc`, `$PROFILE`, fish config files, etc.). The detection function's only side effect is the stderr warning. The user takes manual action to apply the suggested command.

#### Scenario: Detection fails to find tr
- **WHEN** the detection function determines `torec` is not on PATH
- **THEN** the function returns without modifying any file; only stderr is written; the user must copy-paste the suggested command into their shell

#### Scenario: rc file is read-only
- **WHEN** the detection function determines `torec` is not on PATH and the user's rc file (e.g. `~/.bashrc`) is read-only
- **THEN** the detection function's behavior is unchanged — it prints the warning to stderr; it never attempts to write to the rc file, so the read-only state is irrelevant


### Requirement: Stale legacy tr binary from this project triggers a warning
On Unix, when PATH detection finds a `tr` binary whose `--version` output identifies it as the Total Recall CLI (`tr version dev` or the release version format printed by this project, NOT a coreutils version string), the detection SHALL print a warning advising the user to delete the stale legacy binary because it shadows the coreutils `tr` translate utility. The warning SHALL NOT be printed when the discovered `tr` is the system coreutils utility or any other unrelated program. Detection SHALL NOT modify any file; only stderr is written.

#### Scenario: Stale Total Recall tr binary detected on PATH
- **WHEN** `torec init` runs PATH detection and finds `tr` on PATH whose `--version` output matches the Total Recall version format
- **THEN** a warning is printed to stderr advising removal of the stale legacy `tr` binary; `torec init` then continues with the normal flow

#### Scenario: System coreutils tr on PATH produces no warning
- **WHEN** `torec init` runs PATH detection and finds `tr` on PATH whose `--version` output is a coreutils version string (e.g. `tr (GNU coreutils) 9.11`)
- **THEN** no legacy-collision warning is printed; the system utility is correctly identified as unrelated to this project

#### Scenario: No tr on PATH at all
- **WHEN** `torec init` runs PATH detection and no `tr` binary exists on PATH
- **THEN** no legacy-collision warning is printed
