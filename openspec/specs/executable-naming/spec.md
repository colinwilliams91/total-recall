## Purpose

Guarantee that installing the Total Recall CLI never shadows a standard Unix utility name — specifically GNU coreutils `tr` — so that a normal installation cannot break unrelated applications, shell scripts, or desktop-environment launchers that invoke `tr` through PATH.

## Requirements

### Requirement: Installed executable must not be named tr
The project SHALL NOT install, build for release, or document an executable named `tr` — the exact name owned by the GNU coreutils translate utility on Unix systems. The distributed executable SHALL be named `torec`. This applies to every installation mechanism: `go install` (via the package directory name), `make build`, goreleaser release archives (`binary: torec`), the rebuild script, and all documented install instructions.

#### Scenario: go install produces the renamed executable
- **WHEN** a user runs `go install github.com/colinwilliams91/total-recall/cmd/torec@latest`
- **THEN** the installed executable is named `torec` (or `torec.exe` on Windows), and no executable named `tr` is placed onto PATH

#### Scenario: make build produces the renamed executable
- **WHEN** a developer runs `make build`
- **THEN** the output binary is `bin/torec` (or `bin/torec.exe` on Windows), never `bin/tr`

#### Scenario: Release archives carry the renamed executable
- **WHEN** a release is built via goreleaser
- **THEN** every platform archive contains an executable named `torec` (or `torec.exe`), and none contains `tr`

---

### Requirement: System tr remains invocable after installation
On a clean Linux system containing GNU coreutils, installing Total Recall and placing its binary directory ahead of `/usr/bin` in PATH SHALL NOT change how the shell resolves `tr`: `command -v tr` SHALL resolve to the system coreutils `tr`, and `tr '[:lower:]' '[:upper:]'` SHALL continue to invoke the system utility.

#### Scenario: Clean environment resolves system tr
- **WHEN** a Linux environment contains only standard coreutils plus the installed `torec` binary directory put first on PATH
- **THEN** `command -v tr` resolves to the system coreutils `tr` (e.g. `/usr/bin/tr`) and `echo hello | tr '[:lower:]' '[:upper:]'` prints `HELLO` via the system utility

#### Scenario: Desktop launchers keep working
- **WHEN** a desktop-environment launcher (e.g. Hyprland/Omarchy keybind scripts) internally invokes `tr` expecting the coreutils utility while Total Recall is installed
- **THEN** the launcher invokes system coreutils `tr` because no Total Recall executable is named `tr`; launcher functionality is unaffected

#### Scenario: CI guard cannot regress
- **WHEN** CI runs the executable-name guard
- **THEN** the build output and release configuration fail the check if any artifact would be installed as `tr`, and the CI run is marked failed
