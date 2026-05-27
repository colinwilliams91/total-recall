## ADDED Requirements

### Requirement: tr init prompts user to select which hooks to enable
`total-recall init` SHALL present a Huh prompt for each of the three hooks (`pre-commit`, `commit-msg`, `pre-push`), displaying a one-line workflow impact description alongside each confirm. The user's selections SHALL be written into the `hooks:` section of `.tr.yaml` (creating `.tr.yaml` if it does not exist). Only hooks the user enables SHALL be installed into `.git/hooks/`.

Hook impact descriptions displayed in the prompt:
- `pre-commit` — *"Recall check at every commit. Highest signal, most frequent."*
- `commit-msg` — *"Enriches recall with your commit intent. Runs silently after pre-commit."*
- `pre-push` — *"Architecture-level recall across all commits in the push. Less frequent."*

#### Scenario: User enables all three hooks
- **WHEN** the user confirms all three hooks during `total-recall init`
- **THEN** `.tr.yaml` is created/updated with all three set to `true` and all three scripts are installed in `.git/hooks/`

#### Scenario: User enables only pre-commit
- **WHEN** the user confirms only `pre-commit` during `total-recall init`
- **THEN** `.tr.yaml` has `pre-commit: true`, `commit-msg: false`, `pre-push: false`, and only the pre-commit script is installed

#### Scenario: tr init re-run respects existing .tr.yaml hook selections
- **WHEN** the user runs `total-recall init` again and `.tr.yaml` already has hook selections
- **THEN** the Huh prompts are pre-populated with the existing values so the user can review and update without re-deciding from scratch

---

### Requirement: tr init installs managed hooks into .git/hooks/
`total-recall init` SHALL install hook scripts into the repository's `.git/hooks/` directory for each hook the user enabled in the prompt. Installation occurs after the user config and hook selection steps complete.

#### Scenario: First-time hook installation
- **WHEN** the user runs `total-recall init` in a Git repository, enables hooks, and no existing hooks are present
- **THEN** the enabled hooks are installed as executable scripts in `.git/hooks/` and a confirmation message is printed for each installed hook

#### Scenario: Not inside a Git repository
- **WHEN** the user runs `total-recall init` outside of a Git repository (no `.git/` ancestor directory)
- **THEN** config setup and hook selection complete, but installation is skipped with an advisory message

---

### Requirement: tr init detects and chains existing hooks
If a `.git/hooks/<hook>` file exists and is NOT a Total Recall managed hook, `tr init` SHALL chain it by prepending the existing hook content and appending the Total Recall hook dispatch after it. The user SHALL be informed that chaining occurred.

#### Scenario: Existing unmanaged hook detected
- **WHEN** `total-recall init` encounters `.git/hooks/pre-commit` that does not contain the Total Recall sentinel
- **THEN** the existing content is preserved, the Total Recall dispatch is appended after it, and the sentinel is written into the first 5 lines of the new script

#### Scenario: Existing hook execution failure preserved
- **WHEN** an existing chained hook exits with a non-zero code
- **THEN** the chained script exits with that same non-zero code before the Total Recall dispatch runs

---

### Requirement: Managed hook detection via sentinel
A Total Recall managed hook SHALL be identified by the sentinel comment `# total-recall managed` appearing within the first 5 lines of the hook script. Scripts without this sentinel are treated as unmanaged (user-owned).

#### Scenario: Re-running tr init on a managed hook
- **WHEN** `total-recall init` is run again and `.git/hooks/pre-commit` already contains the Total Recall sentinel
- **THEN** the managed portion of the hook is regenerated/updated in place without re-wrapping or duplicating the existing content

---

### Requirement: Hook scripts are installed as executable
All installed hook scripts SHALL have executable permissions set (`chmod +x` equivalent). On Windows, Git for Windows runs hooks through Git Bash, so `.sh` variants SHALL be installed with the appropriate line endings and permissions recognized by Git Bash.

#### Scenario: Hook file permissions after install
- **WHEN** `total-recall init` completes hook installation on a Unix-like system
- **THEN** each installed hook file has execute permission for the owner (mode `0755`)

---

### Requirement: tr init hook installation is idempotent
Running `total-recall init` multiple times in the same repository SHALL produce the same result as running it once. Re-runs SHALL regenerate managed hooks without duplicating content or prompting unnecessarily.

#### Scenario: Repeated tr init calls
- **WHEN** `total-recall init` is run a second time in a repository where hooks are already installed
- **THEN** hooks are updated (not duplicated), config is updated, and the operation exits successfully
