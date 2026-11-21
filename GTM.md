# Go-To-Market: total-recall v1.0

**Thesis**: preserve engineering intuition in the age of AI agents. Four seconds
per Git event is two letter-grades of skill retention saved.

**Definition of "market-ready"**: a developer on a fresh machine installs via
`brew install` in under five minutes, autostart keeps the daemon alive across
reboots, they get a quiz on their first commit, and after a week of use they can
open `torec stats` and see their retention — plus old concepts coming back to
them unprompted.

**Positioning**: always OSS (GPLv3), optimized for end-users over contributors.
Spaced repetition ships *before* launch — it is what turns "quiz on commit"
(a novelty) into a retention tool (the product).

---

## Wave A — Make it stick (install & daily friction)

Removes the friction that kills adoption and daily retention. All items are
small and parallelizable.

### Workstream A1 — Install & lifecycle (`10-opsx-b`)

| Item | Detail |
|------|--------|
| Package managers | Add `homebrew_casks` + `scoop` to `.goreleaser.yaml` (`brews`/formulas is deprecated since goreleaser v2.16 and fails `goreleaser check`); `release.yml` already fires on tags — config, tap/bucket repos, and per-repo publish tokens |
| Daemon autostart | Opt-in offer in `torec init`: launchd plist (macOS), systemd user unit (Linux), Startup-folder script (Windows). Must interoperate with `torec stop` and the pidfile |
| Status without daemon | Fix #56 — `torec status` / `config --show` work with the daemon down (read pidfile + config directly) |

*Winget* is **post-1.0.0**: Windows CI exists (`windows-latest` in the CI
matrix) so it is feasible, but goreleaser has no native winget publisher — it
needs a one-time manifest PR to `microsoft/winget-pkgs` plus per-release
automation (`winget-releaser`-style action), which adds external-review latency
and release-process coupling. Revisit after v1.0.0 with the portable-installer
manifest.

### Workstream A2 — Launch-blocking polish (`10-opsx-c`)

Tickets validated against the tree on 2026-11-18; stale ones get closed, not
fixed.

| Item | Status |
|------|--------|
| Duplicate daemon advisory on commit (#14) | **Likely live** — on a single commit, `pre-commit` and `commit-msg` hooks each print the no-daemon advisory (`hooks/pre-commit.sh:49`, `hooks/commit-msg.sh:31`); confirm and dedupe |
| Empty-queue message too long (#30) | **Likely stale** — the caught-up message is now one short line (`cmd/torec/ask.go:61`) and the repo-move advisory was removed (`integration_test.go:807`); verify manually, probably close |
| MCP/TUI schema normalization (#33) | **Valid** — cross-surface payload consistency remains architectural debt |
| Default LLM provider value (#34) | **Valid, small** — config defaults to `anthropic` (`internal/config/config.go:156`); decide the zero-friction default (candidate: OpenRouter free tier for a no-BYOK first run) |
| golangci errcheck config (#67) | **Valid** — no `.golangci.yml` exists in the tree; add the errcheck exclusions |
| Stale architecture docs/diagrams (#32) | **Partially stale** — `diagramming-conflicts.txt` exists but docs have been refreshed since June; audit and close |
| `torec init` accessible-mode Scanner bug | **Valid** — documented in `scripts/e2e`; first-run path must be robust |

---

## Wave B — Make it valuable (close the retention loop)

The critical path. Goes through the standard OpenSpec flow
(proposal → design → specs → tasks → implement → verify → archive).

### Workstream B1 — Spaced repetition resurfacing (`10-opsx-d`)

Commits are the only question trigger, and questions must stay relevant to the
commit diff. Resurfacing therefore **piggybacks on normal commit hook events**
— no new event source:

1. A concept answered correctly in the past accrues "decay time". When its
   interval elapses, the *next* normal commit event offers a bonus:
   **"Would you like a bonus question to boost your knowledge retention? y/n"**
2. On `y`, the resurfaced question is asked after the normal commit question;
   on `n`, the bonus is skipped (and the concept re-arms for the next event).
3. After a resurfacing, the concept's timer **exits the loop** — it does not
   reset to a shorter interval. A concept is resurfaced **once** (at most
   twice); this keeps the resurfacing pool from filling with repeat questions
   and preserves the "quiz the commit diff" norm.

Design decisions (resolve in the design doc):

- **Interval policy** — simple interval ladder (1d / 3d / 7d / 14d) keyed by
  the concept's difficulty level and answer correctness; fits the
  "thin, no over-engineering" posture and roadmap 4.2's "recall debt" framing.
- **Scheduling state** — `question_events` already captures answer history;
  a small scheduler table (concept → next-eligible-after, resurfaced-count)
  plus a selection hook in `internal/recall` that offers the bonus question
  alongside the commit-derived question.
- **Decline semantics** — `n` re-arms the concept at the next interval step;
  repeated declines eventually exhaust the concept (same exit rule).
- **Policy-asset impact** — the question-generation policy must handle
  "ask about a concept the user hasn't touched recently" (different framing
  than commit-derived concepts).

**Completion signal**: a concept asked 10 days ago and answered correctly
produces a bonus-question offer on a commit ~day 10, even though no commit
touches that concept — and after being asked once, it never resurfaces again.

### Workstream B2 — `torec stats` (`10-opsx-e`)

Streak, accuracy over a rolling window, concepts covered, resurfaced-and-held
count — read from local `memory.db` only, **no telemetry** (privacy position
worth stating on the landing page). Makes the "data-backed developer tool"
claim real and feeds the launch demo.

---

## Wave C — Make it known (launch)

### Workstream C1 — Launch assets (`10-opsx-f`)

- Demo GIF / asciinema of commit → quiz → feedback → bonus resurfacing
- Landing-page-quality README hero + simple site (or GitHub Pages) carrying
  the manifesto thesis
- Launch channels: Show HN, r/programming + r/LocalLLaMA, MCP server
  directories, Claude Code / Copilot CLI integration guides
- Tag `v1.0.0` — the semver jump is the statement

---

## Branch & parallelization model

Planning lives on `10-opsx`. Each workstream gets a sub-branch in its own
worktree so items run in parallel:

```
10-opsx        planning (this branch) — GTM.md, OpenSpec artifacts for B1
10-opsx-b      Wave A — install & lifecycle
10-opsx-c      Wave A — launch-blocking polish        (parallel with -b)
10-opsx-d      Wave B — spaced repetition resurfacing
10-opsx-e      Wave B — torec stats                   (parallel with -d)
10-opsx-f      Wave C — launch assets
```

Merge order: Wave A (b + c) → Wave B (d + e) → Wave C (f) → tag `v1.0.0`.

Workstream B1's OpenSpec change (proposal/design) is authored on `10-opsx`
before implementation branches open, per the repo's spec-driven
development convention.
