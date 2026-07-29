## ADDED Requirements

### Requirement: tr init checks whether tr is on PATH before prompting
`tr init` SHALL, as its first action before any user-facing prompts are shown, detect whether `tr` is reachable via `exec.LookPath("tr")` on Unix (or `Get-Command tr` on Windows PowerShell). The detection MUST NOT modify any shell rc file (`~/.bashrc`, `~/.zshrc`, `$PROFILE`, etc.). When `tr` is found on PATH, `tr init` SHALL proceed silently with no PATH-related output.

#### Scenario: tr is on PATH
- **WHEN** `tr init` is run on a system where `tr` is reachable via PATH
- **THEN** `tr init` proceeds with no PATH-detection warning; the first user-facing output is the conversation-analysis opt-in prompt

#### Scenario: tr is not on PATH
- **WHEN** `tr init` is run on a system where `tr` is NOT reachable via PATH
- **THEN** `tr init` prints a warning to stderr; the warning includes a shell-specific one-line command for the user to paste to add `$GOPATH/bin` to their PATH; `tr init` then continues with the normal flow (conversation-analysis opt-in, AI provider, etc.)

---

### Requirement: Warning message is shell-specific and copy-pasteable
When `tr` is not on PATH, the warning text printed to stderr SHALL include the exact one-line command for the detected shell. The detection SHALL support at minimum:

- **Bash** (`$SHELL` contains `bash`, or `$SHELL` is empty on Linux): `echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc` (or platform-equivalent)
- **Zsh** (`$SHELL` contains `zsh`): `echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc` (or platform-equivalent)
- **PowerShell** (`runtime.GOOS == "windows"`): `Add-Content $PROFILE 'set PATH="$PATH;$(go env GOPATH)/bin"'` or equivalent copy-pasteable PowerShell syntax

Shells NOT in the supported list (fish, nushell, etc.) SHALL still receive a warning; the warning MAY fall back to the bash/zsh form with a note that the user may need to adapt the syntax.

The warning format SHALL be: `⚠  tr not found on PATH. Add $GOPATH/bin to PATH with: <one-line-command>` followed by a brief reason ("`tr repo` installs a post-commit hook that relies on PATH resolution at fire time.").

#### Scenario: Bash user without tr on PATH
- **WHEN** `tr init` is run on Linux with `$SHELL=/bin/bash` and `tr` not on PATH
- **THEN** stderr receives a line containing `⚠  tr not found on PATH. Add $GOPATH/bin to PATH with: echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc` (exact wording may vary but the command form MUST be copy-pasteable into bash)

#### Scenario: Zsh user without tr on PATH
- **WHEN** `tr init` is run on macOS with `$SHELL=/bin/zsh` and `tr` not on PATH
- **THEN** stderr receives a line containing the zsh-specific PATH-fix one-liner with `>> ~/.zshrc` as the target file

#### Scenario: PowerShell user without tr on PATH
- **WHEN** `tr init` is run on Windows (`runtime.GOOS == "windows"`) and `tr` not on PATH
- **THEN** stderr receives a line containing the PowerShell-specific PATH-fix one-liner targeting `$PROFILE`

---

### Requirement: Detection never modifies rc files
The PATH-detection function SHALL only print warning text to stderr. It MUST NOT use `os.OpenFile` or any other write API against shell rc files (`~/.bashrc`, `~/.zshrc`, `$PROFILE`, fish config files, etc.). The detection function's only side effect is the stderr warning. The user takes manual action to apply the suggested command.

#### Scenario: Detection fails to find tr
- **WHEN** the detection function determines `tr` is not on PATH
- **THEN** the function returns without modifying any file; only stderr is written; the user must copy-paste the suggested command into their shell

#### Scenario: rc file is read-only
- **WHEN** the detection function determines `tr` is not on PATH and the user's rc file (e.g. `~/.bashrc`) is read-only
- **THEN** the detection function's behavior is unchanged — it prints the warning to stderr; it never attempts to write to the rc file, so the read-only state is irrelevant