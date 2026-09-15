## Context

`tr`'s TUI surfaces are well-defined and testable today: `tr init` (huh forms), `tr status`, `tr ask` (thinking → question → feedback/done states, see golden views in `cmd/tr/testdata/`), and the git-hook question flow. The TUI requires a daemon on `localhost:7331` and an AI provider for real question synthesis — the AI provider is unacceptable inside a recorded demo (nondeterministic, slow, requires secrets); the daemon is fine when run isolated and seeded with known questions.

VHS (charmbracelet/vhs) records terminal GIFs from `.tape` scripts executed in a virtual ttyd terminal — a real PTY, so Bubble Tea/huh apps render faithfully (huh and VHS are both Charm projects; this is their own demo workflow). Tapes support `Output` to multiple formats (`.gif`, `.txt`/`.ascii`), `Hide`/`Show` for off-camera setup, `Require` for early failure on missing binaries, `Env` for test isolation, and `Wait /regex/` to synchronize with TUI states instead of fixed sleeps. VHS needs `ttyd` + `ffmpeg` on PATH, or its Docker image (`ghcr.io/charmbracelet/vhs`) — but only to *re-render*; consumers of the rendered GIF need nothing.

## Goals / Non-Goals

**Goals:**
- Three distinct, comparable demo tapes, each rendering `.gif` + `.txt`/`.ascii`.
- Deterministic renders: no network to an AI provider, no API key, no live production daemon.
- Faithful demos: real `tr` binary and real TUI (`tr init` huh forms included) wherever feasible; simulated components only where a live dependency is unacceptable (AI provider, production daemon).
- External demo repo: all demo assets (tapes, harness, dummy setup, rendered outputs) live in a dedicated repo outside this one; this repo's footprint shrinks to a single README link plus a pointer note in `CONTRIBUTING.md`.
- Tapes readable and lightly commented — the demo explains itself.

**Non-Goals:**
- CI automation of demo re-rendering (vhs-action) — possible follow-up, not this change.
- Narrated/voiceover or longer marketing videos.
- Changing any Go code, TUI behavior, or adding Go dependencies (VHS is authoring-time only).
- Keeping demo re-rendering contributor-friendly — demo refresh after UX changes is maintainer-owned (see Risks).

## Decisions

**Faithful demos: real `tr` binary in a real PTY, simulated dependencies only where unavoidable.**
VHS's ttyd terminal is a real PTY, so the actual `tr` binary — including the huh forms in `tr init` — renders faithfully on camera. Components are simulated only where a live dependency is unacceptable:

- `tr init` — the real binary, run on camera in variant C. Interactive AI-setup fields get a dummy key typed on camera (or `Hide`-wrapped). Side effects are isolated with `Env TR_HOME` pointing at a temp dir. The accessible-mode `bufio.Scanner` bug noted in the e2e docs does not apply — that only affects `--accessible` mode, not the interactive TUI.
- AI provider — never real. Provider setup is either typed as a dummy value or skipped; no network calls to a provider happen in any demo.
- Daemon-dependent flows (`tr ask`, post-commit quiz) — prefer a real headless daemon: start `tr serve` with `TR_HOME` isolation and seed the cache store with known questions (the same seeding approach as `startTestDaemon` in the integration tests). The demo then shows the real client against real daemon responses. A fake `tr` shim on `PATH` that emits frames copied from the golden views (`cmd/tr/testdata/TestGoldenAsk*.golden`) is the fallback if daemon-seeding proves awkward in the tape environment — the shim keeps renders fully deterministic at the cost of authenticity.

Alternatives considered: shim for everything — rejected as inauthentic when the real binary works in a PTY; real AI provider — rejected (secrets, nondeterminism, cost).

**Three variants differ by scene selection and framing, not gimmicks:**
1. `demo-a-full-flow.tape` — compressed journey: `go install` → `tr init` → `tr serve` → commit → quiz pops up → answered → done. The "whole story in ~20s" cut.
2. `demo-b-quiz-focus.tape` — starts mid-flow (repo already set up): commit, question appears, answer, feedback. The "this is what using it feels like" cut.
3. `demo-c-init-focus.tape` — the setup experience: real `tr init` (huh forms on camera), provider config, `tr repo`, `tr status`. The "getting started is easy" cut.

Each shares the same visual profile (below) so comparison is about content, not chrome.

**Shared visual profile:** `Set Theme` Charm-style dark theme, `FontSize` ~20, `WindowBar Colorful` with border radius, width ~1000px / ~30 rows — sized to be legible in README full-width embeds. Pacing via `Sleep` beats of 400–800ms and `Set TypingSpeed` for realistic typing; `Wait /regex/` where a frame must appear before advancing.

**Delivery: everything lives in a dedicated demo repo; this repo holds one README link and a CONTRIBUTING.md pointer.**
All demo assets — `.tape` sources, the tracking harness (temp `TR_HOME`, dummy provider key, seeded headless daemon), the dummy project being committed to, rendered `.gif`/`.txt` outputs, and re-render notes — live in a dedicated demo repo (e.g. `total-recall-demos`) outside this codebase. That repo can be a single repo combining the harness and the scratch project the recorded session operates on: the demo itself lives in the very repo it demos (the dummy setup must be its own repo anyway — hooks are per-repo, per the install-layers testing rules). Rendered GIFs are published as **GitHub Releases assets** of the demo repo: each re-record becomes a new tag (e.g. `demos-v3`) with GIFs attached, giving stable URLs plus free versioned history of past demos. `vhs publish` remains an acceptable alternative but the Releases scheme is preferred.

This repo's footprint: exactly one linked image line at the top of `README.md` (the winner's hosted URL) plus a one-line pointer in `CONTRIBUTING.md` telling people where demos live and that re-rendering is maintainer-owned. No demo scaffolding, harness, or binaries enter this repository. Users and contributors need no VHS/ttyd/ffmpeg; those tools are only required by the maintainer re-rendering in the demo repo. After the three renders are compared, the winner's hosted URL goes into the README; the remaining variants continue to exist as releases/tapes in the demo repo.

**Tape hygiene:** `Require` is used for the real `tr` binary in variants that need it (and omitted when the shim fallback is in play); `Hide` wraps all setup (temp `TR_HOME`, dummy key, daemon start/seed, state reset) and cleanup so frames contain only the demo itself. Sparse comments only — scene headers, no narration.

**Demo refresh ownership:** demo re-recording after UX changes is explicitly maintainer-owned, not contributor-owned. Golden tests already fail on ask-view drift, giving a natural signal; the `CONTRIBUTING.md` pointer records where demos live and that expectation, rather than building contributor tooling around it.

## Risks / Trade-offs

- [Demo frames drift from a real `tr` experience that has since changed] → Accepted and maintainer-owned: hosted GIFs are static snapshots that drift regardless of how they were produced; golden tests breaking on ask-view changes is the natural signal to re-record, and the `CONTRIBUTING.md` pointer records that expectation. No contributor tooling is built for this.
- [Demo repo URL or release asset dies] → Release URLs under the maintainer's own GitHub account/org are stable and versioned; tapes stay source-of-truth in the demo repo so any GIF can be re-rendered and re-released.
- [VHS/ttyd/ffmpeg not installed for the maintainer re-recording] → Docker one-liner fallback documented in the demo repo; consumers never need these tools since GIFs are hosted.
- [Creating and maintaining a second repo] → One-time setup cost, can start private; keeps this codebase clean and the demo setup (which must be its own repo anyway) in its natural home.
- [Headless daemon + seeded store adds setup complexity inside the tape] → All daemon start/seed/reset is `Hide`-wrapped off-camera; keep the shim fallback on hand if the seeding proves brittle, at the cost of losing on-camera authenticity for the quiz flow only.
- [Real `tr init` on camera is slower and less deterministic than a shimmed frame] → Accepted per "faithful demo over slight complexity" decision; huh forms render genuinely in the PTY, and `Wait /regex/` synchronizes the tape with form states instead of fixed sleeps.

## Open Questions

None — scene selection across the three variants is intentionally exploratory; picking the winner (and its hosting URL) is an explicit follow-up decision by the user.
