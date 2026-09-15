## Purpose

Provides scriptable terminal demo recordings (VHS tapes) of the `tr` TUI so the GitHub README landing page can show — rather than describe — the Total Recall experience: install-to-quiz in a few seconds of silent GIF.

## ADDED Requirements

### Requirement: Demo tapes render the tr TUI experience
Each demo tape SHALL script a terminal session showing real `tr` TUI behavior (e.g., the real `tr init` huh forms, `tr status`, `tr ask` flow), recorded from the real `tr` binary wherever feasible. Live AI provider calls SHALL NOT occur in any demo (no API key, no network to a provider); daemon-dependent flows SHALL use an isolated, pre-seeded daemon or, as a fallback, a scripted shim mirroring the real TUI frames.

#### Scenario: Rendering a tape
- **WHEN** `vhs <tape>.tape` is run on a machine with VHS installed
- **THEN** a `.gif` is produced matching the tape's scripted session without network calls to an AI provider

#### Scenario: No API key required
- **WHEN** the recording environment has no AI provider key configured
- **THEN** the tape still renders to completion

#### Scenario: Faithful init flow
- **WHEN** a tape demos the setup experience
- **THEN** it records the real `tr` binary and its interactive TUI forms in the terminal session, not simulated frames, except for daemon/AI dependencies

### Requirement: Three comparable demo variants
The change SHALL provide exactly three distinct tape files, each presenting a different take on the same demo (e.g., differing framing, pacing, or scene selection), so the best can be compared and picked. Tape sources, harness, and rendered outputs SHALL live in a dedicated demo repo outside the main codebase; the main repo SHALL NOT carry demo scaffolding or binary render outputs.

#### Scenario: Comparing variants
- **WHEN** the rendered outputs of the three tapes are viewed side by side
- **THEN** each is a complete, self-contained demo of the `tr` experience, distinguishable in style but equivalent in scope

#### Scenario: Main repo footprint
- **WHEN** the change's work is complete
- **THEN** the main repo contains no demo tapes, harness scripts, or rendered binaries from this work

### Requirement: Dual output format per tape
Every tape SHALL declare both a `.gif` output and an ASCII/`.txt` output so the rendered session can be previewed in text and diffed across re-renders.

#### Scenario: Outputs declared
- **WHEN** a tape file is inspected
- **THEN** it contains at least an `Output *.gif` and an `Output *.txt` (or `.ascii`) command

### Requirement: Concise landing-page placement
The chosen demo SHALL be linked at the very top of `README.md` (the landing page) with a single hosted image reference — the GIF hosted as a release asset of the dedicated demo repo, giving stable, versioned URLs without adding binaries or tooling requirements to the main repo. The demo SHALL be short and simple: no narrated exposition beyond, at most, sparse tape-file comments; the TUI itself is the message. Re-rendering demos SHALL be maintainer-owned; contributors are not expected to regenerate them, and the main repo's contributor documentation SHALL carry a pointer to where demos live and that expectation.

#### Scenario: README embed
- **WHEN** the selected GIF's hosted URL is linked at the top of `README.md`
- **THEN** it renders as the first visual element a visitor sees, below (or beside) the existing logo/badges, without additional exposition text, and no large binary is added to the repository

#### Scenario: Consumer needs no tooling
- **WHEN** a README reader or contributor views the demo
- **THEN** no VHS, ttyd, or ffmpeg installation is required on their part
